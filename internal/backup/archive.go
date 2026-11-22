// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"jim-dot-tennis/internal/branding"
)

// The archive layout. Entries are regular files only, in this order:
//
//	manifest.json                 always first
//	database/tennis.db            the database copy
//	tournaments/courthive.dump    the tournament database (only when the
//	                              tournament stack is part of the deployment)
//	files/branding/<logo name>    uploaded logos (zero or more)
//
// An archive without tournament data is format 1, exactly as before
// tournament data existed, so older versions of the app still read it. One
// with tournament data is format 2 and its manifest has a "tournaments"
// entry; an older version refuses it rather than restore half of it.
//
// Restore accepts nothing else: any other name, a directory, a link, a
// device, an absolute or ".." path, a duplicate, an entry over its size
// limit, or an entry the manifest does not list at exactly the size it
// lists is refused before anything is written outside the staging folder
// (and before any of that entry is written inside it).
const (
	FormatVersion = 2
	// formatAppOnly is the format of an archive without tournament data.
	formatAppOnly    = 1
	appName          = "jim-dot-tennis"
	manifestName     = "manifest.json"
	databaseName     = "database/tennis.db"
	tournamentName   = "tournaments/courthive.dump"
	filesPrefix      = "files/"
	uploadsDirName   = "branding"
	uploadsPrefix    = filesPrefix + uploadsDirName + "/"
	archiveExtension = ".tar.gz"
)

// Size limits for what restore reads from an archive (which may come from
// anywhere). A club's database is a few megabytes; logos are at most 2 MB.
const (
	maxManifestBytes   = 1 << 20
	maxDatabaseBytes   = 8 << 30
	maxUploadBytes     = 16 << 20
	maxTournamentBytes = 16 << 30
	maxEntries         = 1000
)

// Kinds of backup, recorded in the manifest.
const (
	KindManual       = "manual"        // `jim-dot-tennis backup`
	KindScheduled    = "scheduled"     // the server's schedule
	KindPreMigration = "pre-migration" // taken before migrations ran
	KindDownload     = "download"      // made for the admin "Download backup" button
	// KindPreUpgrade is taken by "tournaments pre-upgrade" before a new
	// tournament server image first starts (and migrates its database).
	KindPreUpgrade = "pre-upgrade"
)

// Manifest describes an archive.
type Manifest struct {
	Format        int         `json:"format"`
	App           string      `json:"app"`
	AppVersion    string      `json:"app_version"`
	AppCommit     string      `json:"app_commit"`
	SchemaVersion uint        `json:"schema_version"`
	CreatedAt     time.Time   `json:"created_at"`
	Kind          string      `json:"kind"`
	Database      FileEntry   `json:"database"`
	Files         []FileEntry `json:"files"`
	// Tournaments is the tournament database's dump; nil when the archive
	// has none (format 1).
	Tournaments *TournamentEntry `json:"tournaments,omitempty"`
	Notice      string           `json:"notice"`
}

// TournamentEntry describes the tournament database's dump in an archive.
type TournamentEntry struct {
	Included bool `json:"included"` // always true when present
	FileEntry
	Format          string `json:"format"`                 // TournamentDumpFormat
	ServerVersion   string `json:"server_version"`         // the Postgres server it was dumped from
	Database        string `json:"database"`               // where it came from, for messages (no password)
	Migrations      int    `json:"migrations"`             // tournament-server migrations applied
	LatestMigration string `json:"latest_migration"`       // the newest of them
	ServerImage     string `json:"server_image,omitempty"` // the tournament server image, when known
}

// HasTournaments reports whether the archive holds tournament data.
func (m Manifest) HasTournaments() bool { return m.Tournaments != nil && m.Tournaments.Included }

// FileEntry is one file in the archive.
type FileEntry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// manifestNotice is written into every manifest.
const manifestNotice = "This backup holds the club's personal data, password hashes, passkey details and the app's secrets. " +
	"Store it where only the people who run the app can read it."

// validEntryName checks an archive entry name against the layout. It is
// strict on purpose: the names restore will write are exactly these.
func validEntryName(name string) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) || strings.ContainsRune(name, 0) {
		return fmt.Errorf("unsafe name %q", name)
	}
	if path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
		return fmt.Errorf("unsafe name %q", name)
	}
	switch {
	case name == manifestName, name == databaseName, name == tournamentName:
		return nil
	case strings.HasPrefix(name, uploadsPrefix):
		base := strings.TrimPrefix(name, uploadsPrefix)
		if strings.Contains(base, "/") || !branding.ValidLogoName(base) {
			return fmt.Errorf("unexpected file %q", name)
		}
		return nil
	}
	return fmt.Errorf("unexpected entry %q", name)
}

// entryLimit is the largest size accepted for name.
func entryLimit(name string) int64 {
	switch name {
	case manifestName:
		return maxManifestBytes
	case databaseName:
		return maxDatabaseBytes
	case tournamentName:
		return maxTournamentBytes
	default:
		return maxUploadBytes
	}
}

// archiveWriter writes the archive layout.
type archiveWriter struct {
	gz *gzip.Writer
	tw *tar.Writer
}

func newArchiveWriter(w io.Writer) *archiveWriter {
	gz := gzip.NewWriter(w)
	return &archiveWriter{gz: gz, tw: tar.NewWriter(gz)}
}

func (a *archiveWriter) add(name string, size int64, modTime time.Time, r io.Reader) error {
	if err := a.tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Size:     size,
		Mode:     0o600,
		ModTime:  modTime.UTC().Truncate(time.Second),
		Format:   tar.FormatPAX,
	}); err != nil {
		return err
	}
	n, err := io.Copy(a.tw, r)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("%s changed size while it was being written", name)
	}
	return nil
}

func (a *archiveWriter) close() error {
	if err := a.tw.Close(); err != nil {
		return err
	}
	return a.gz.Close()
}

// errBadArchive marks an archive that is not a readable, intact backup.
var errBadArchive = errors.New("not a usable backup")

// badArchive wraps a problem with an archive's content.
func badArchive(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errBadArchive, fmt.Sprintf(format, args...))
}

// readArchive reads an archive from r, checking every entry against the
// layout and its size limit and every checksum against the manifest. Once
// the manifest is read it calls onManifest (if set), whose error stops the
// read. For each file after the manifest it calls store (if set) with the
// entry's name and its content, but only for an entry the manifest lists
// with the size its header gives, and never handing over more than that
// size, so what onManifest saw bounds what store can be asked to write.
// It returns the manifest once every file was
// read and matched it; nothing a caller stored may be trusted before then.
func readArchive(r io.Reader, onManifest func(Manifest) error, store func(name string, content io.Reader) error) (Manifest, error) {
	var m Manifest
	gz, err := gzip.NewReader(r)
	if err != nil {
		return m, badArchive("it is not a gzip file (%v)", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	seen := map[string]bool{}
	got := map[string]FileEntry{}
	for i := 0; ; i++ {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return m, badArchive("it is damaged or incomplete (%v)", err)
		}
		if i >= maxEntries {
			return m, badArchive("it has more than %d entries", maxEntries)
		}
		if hdr.Typeflag != tar.TypeReg {
			return m, badArchive("entry %q is not a plain file (links, folders and devices are refused)", hdr.Name)
		}
		if err := validEntryName(hdr.Name); err != nil {
			return m, badArchive("%v", err)
		}
		if seen[hdr.Name] {
			return m, badArchive("entry %q appears twice", hdr.Name)
		}
		seen[hdr.Name] = true
		limit := entryLimit(hdr.Name)
		if hdr.Size < 0 || hdr.Size > limit {
			return m, badArchive("entry %q is %d bytes, more than the %d allowed", hdr.Name, hdr.Size, limit)
		}

		if i == 0 {
			if hdr.Name != manifestName {
				return m, badArchive("the first entry is %q, not %s", hdr.Name, manifestName)
			}
			if m, err = decodeManifest(tr); err != nil {
				return m, err
			}
			if onManifest != nil {
				if err := onManifest(m); err != nil {
					return m, err
				}
			}
			continue
		}

		// Nothing is stored that the manifest does not list, at the size it
		// lists: a hostile archive cannot make restore write more than the
		// manifest declared (which onManifest has already seen).
		listed, ok := listedEntry(m, hdr.Name)
		if !ok {
			return m, badArchive("entry %q is not listed in the manifest", hdr.Name)
		}
		if hdr.Size != listed.Size {
			return m, badArchive("entry %q is %d bytes but the manifest says %d", hdr.Name, hdr.Size, listed.Size)
		}

		// Count and hash exactly what is read; never more than listed.
		h := sha256.New()
		counted := &countingReader{r: io.LimitReader(tr, listed.Size+1)}
		content := io.TeeReader(counted, h)
		if store != nil {
			if err := store(hdr.Name, content); err != nil {
				return m, err
			}
		}
		if _, err := io.Copy(io.Discard, content); err != nil {
			return m, badArchive("entry %q is damaged (%v)", hdr.Name, err)
		}
		if counted.n != listed.Size {
			return m, badArchive("entry %q is not the size the manifest says", hdr.Name)
		}
		got[hdr.Name] = FileEntry{Path: hdr.Name, Size: counted.n, SHA256: hex.EncodeToString(h.Sum(nil))}
	}
	if len(seen) == 0 {
		return m, badArchive("it is empty")
	}

	// Every file the manifest lists must be there with its checksum, and
	// nothing else.
	want := append([]FileEntry{m.Database}, m.Files...)
	if m.Tournaments != nil {
		want = append(want, m.Tournaments.FileEntry)
	}
	if len(got) != len(want) {
		return m, badArchive("it holds %d files but the manifest lists %d", len(got), len(want))
	}
	for _, w := range want {
		g, ok := got[w.Path]
		if !ok {
			return m, badArchive("%s is listed in the manifest but missing", w.Path)
		}
		if g.Size != w.Size || g.SHA256 != w.SHA256 {
			return m, badArchive("%s does not match its checksum in the manifest (the file is damaged or was changed)", w.Path)
		}
	}
	return m, nil
}

// decodeManifest reads the manifest entry from r (at most maxManifestBytes)
// and checks it.
func decodeManifest(r io.Reader) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(io.LimitReader(r, maxManifestBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, badArchive("the manifest cannot be read (%v)", err)
	}
	return m, checkManifest(m)
}

// listedEntry is the manifest's entry for name.
func listedEntry(m Manifest, name string) (FileEntry, bool) {
	if name == m.Database.Path {
		return m.Database, true
	}
	if m.Tournaments != nil && name == m.Tournaments.Path {
		return m.Tournaments.FileEntry, true
	}
	for _, f := range m.Files {
		if f.Path == name {
			return f, true
		}
	}
	return FileEntry{}, false
}

// TotalSize is the number of bytes the manifest says the archive's files
// unpack to.
func (m Manifest) TotalSize() int64 {
	n := m.Database.Size
	for _, f := range m.Files {
		n += f.Size
	}
	if m.Tournaments != nil {
		n += m.Tournaments.Size
	}
	return n
}

// checkManifest checks the manifest's own fields.
func checkManifest(m Manifest) error {
	if m.App != appName {
		return badArchive("the manifest is not from %s", appName)
	}
	if m.Format < 1 {
		return badArchive("the manifest has no format version")
	}
	if m.Format > FormatVersion {
		return fmt.Errorf("%w: this backup was made by a newer version of the app (backup format %d; this program reads format %d). Restore it with version %s of the app or later",
			ErrNewerBackup, m.Format, FormatVersion, m.AppVersion)
	}
	if m.Database.Path != databaseName {
		return badArchive("the manifest does not list the database")
	}
	if m.Database.Size < 0 || m.Database.Size > maxDatabaseBytes {
		return badArchive("the manifest lists a database of %d bytes", m.Database.Size)
	}
	if len(m.Files) >= maxEntries {
		return badArchive("the manifest lists more than %d files", maxEntries)
	}
	if t := m.Tournaments; t != nil {
		switch {
		case m.Format < 2:
			return badArchive("the manifest lists tournament data, which format %d cannot hold", m.Format)
		case !t.Included || t.Path != tournamentName:
			return badArchive("the manifest's tournament entry is not usable")
		case t.Format != TournamentDumpFormat:
			return badArchive("the tournament data is in a format this program cannot restore (%q)", t.Format)
		case t.Size <= 0 || t.Size > maxTournamentBytes:
			return badArchive("the manifest lists tournament data of %d bytes", t.Size)
		}
	}
	seen := map[string]bool{databaseName: true, tournamentName: true}
	for _, f := range m.Files {
		if err := validEntryName(f.Path); err != nil || f.Path == manifestName || seen[f.Path] {
			return badArchive("the manifest lists an unexpected file %q", f.Path)
		}
		if f.Size < 0 || f.Size > maxUploadBytes {
			return badArchive("the manifest lists %q at %d bytes, more than the %d allowed", f.Path, f.Size, maxUploadBytes)
		}
		seen[f.Path] = true
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
