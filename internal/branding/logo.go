// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package branding validates, stores and serves the club logo, and serves the
// web app manifest and service worker with the club's name, logo and colours.
package branding

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Upload limits for the club logo.
const (
	MaxLogoBytes     = 2 << 20 // 2 MiB
	MaxLogoDimension = 4096    // pixels, either side
)

// Logo validation errors.
var (
	ErrLogoEmpty    = errors.New("the logo file is empty")
	ErrLogoTooLarge = fmt.Errorf("the logo must be at most %d MB", MaxLogoBytes>>20)
	ErrLogoType     = errors.New("the logo must be a PNG, JPEG or WebP image (SVG is not accepted)")
	ErrLogoCorrupt  = errors.New("the logo file could not be read as an image")
	ErrLogoSize     = fmt.Errorf("the logo must be at most %d pixels wide and high", MaxLogoDimension)
)

// Logo is a validated logo upload.
type Logo struct {
	Data   []byte
	MIME   string // image/png, image/jpeg or image/webp
	Ext    string // .png, .jpg or .webp
	Width  int
	Height int
}

// logoNameRe matches the file names Store produces; nothing else is ever served.
var logoNameRe = regexp.MustCompile(`^logo-[0-9a-f]{16}\.(png|jpg|webp)$`)

// ValidLogoName reports whether name is a file name Store could have produced.
func ValidLogoName(name string) bool { return logoNameRe.MatchString(name) }

// ValidateLogo checks an uploaded logo by content (the file name and the
// browser-supplied type are ignored): PNG, JPEG or WebP only, at most
// MaxLogoBytes, with a readable header and sane dimensions.
func ValidateLogo(data []byte) (*Logo, error) {
	if len(data) == 0 {
		return nil, ErrLogoEmpty
	}
	if len(data) > MaxLogoBytes {
		return nil, ErrLogoTooLarge
	}
	logo := &Logo{Data: data, MIME: http.DetectContentType(data)}
	var err error
	switch logo.MIME {
	case "image/png":
		logo.Ext = ".png"
		cfg, e := png.DecodeConfig(bytes.NewReader(data))
		logo.Width, logo.Height, err = cfg.Width, cfg.Height, e
	case "image/jpeg":
		logo.Ext = ".jpg"
		cfg, e := jpeg.DecodeConfig(bytes.NewReader(data))
		logo.Width, logo.Height, err = cfg.Width, cfg.Height, e
	case "image/webp":
		logo.Ext = ".webp"
		logo.Width, logo.Height, err = webpSize(data)
	default:
		return nil, ErrLogoType
	}
	if err != nil {
		return nil, ErrLogoCorrupt
	}
	if logo.Width <= 0 || logo.Height <= 0 {
		return nil, ErrLogoCorrupt
	}
	if logo.Width > MaxLogoDimension || logo.Height > MaxLogoDimension {
		return nil, ErrLogoSize
	}
	return logo, nil
}

// webpSize reads the canvas size from a WebP file header (VP8, VP8L or VP8X).
func webpSize(b []byte) (int, int, error) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, ErrLogoCorrupt
	}
	d := b[20:]
	switch string(b[12:16]) {
	case "VP8X":
		w := int(d[4]) | int(d[5])<<8 | int(d[6])<<16
		h := int(d[7]) | int(d[8])<<8 | int(d[9])<<16
		return w + 1, h + 1, nil
	case "VP8 ":
		if d[3] != 0x9d || d[4] != 0x01 || d[5] != 0x2a {
			return 0, 0, ErrLogoCorrupt
		}
		w := int(binary.LittleEndian.Uint16(d[6:8]) & 0x3fff)
		h := int(binary.LittleEndian.Uint16(d[8:10]) & 0x3fff)
		return w, h, nil
	case "VP8L":
		if d[0] != 0x2f {
			return 0, 0, ErrLogoCorrupt
		}
		bits := binary.LittleEndian.Uint32(d[1:5])
		return int(bits&0x3fff) + 1, int((bits>>14)&0x3fff) + 1, nil
	}
	return 0, 0, ErrLogoCorrupt
}

// Store writes the logo into dir (created if needed) under a content-hashed
// name and returns that name. The write is atomic (temporary file + rename).
func Store(dir string, logo *Logo) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create branding directory: %w", err)
	}
	sum := sha256.Sum256(logo.Data)
	name := "logo-" + hex.EncodeToString(sum[:8]) + logo.Ext
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("store logo: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(logo.Data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("store logo: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("store logo: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return "", fmt.Errorf("store logo: %w", err)
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		return "", fmt.Errorf("store logo: %w", err)
	}
	return name, nil
}

// RemoveOtherLogos deletes stored logos in dir other than keep ("" removes all).
func RemoveOtherLogos(dir, keep string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if name == keep || e.IsDir() || !strings.HasPrefix(name, "logo-") || !ValidLogoName(name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
