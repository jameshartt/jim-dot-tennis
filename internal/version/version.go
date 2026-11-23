// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package version holds the build's identity. The release build sets the
// variables with -ldflags "-X jim-dot-tennis/internal/version.Version=..."
// (see the Dockerfile); a plain `go build` leaves them at "dev"/"unknown".
package version

import (
	"fmt"
	"runtime/debug"
)

// Set at build time.
var (
	Version = "dev"     // release number, e.g. 1.4.0, or a git describe
	Commit  = "unknown" // git commit the build was made from
	Date    = "unknown" // build time, UTC, RFC 3339
)

// ProjectURL is the project's home page. The User-Agent names it so a
// website owner who sees our requests can find out what they are.
const ProjectURL = "https://github.com/jameshartt/jim-dot-tennis"

// UserAgent is the User-Agent header for every request this software makes
// to the BHPLTA website (and any other site it reads pages from): honest,
// naming the software, its version and the project, for example
// "jim-dot-tennis/0.2.1 (+https://github.com/jameshartt/jim-dot-tennis)".
// It never pretends to be a browser: the BHPLTA site resets requests that
// claim to be Chrome and refuses Go's default User-Agent with 403, but
// answers this one.
func UserAgent() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	return "jim-dot-tennis/" + v + " (+" + ProjectURL + ")"
}

// Short is the version alone: what is shown to people and at /version.
func Short() string { return Version }

// FullCommit is the commit, falling back to what the Go toolchain embedded
// when a plain `go build` was run inside a git checkout.
func FullCommit() string {
	if Commit != "unknown" && Commit != "" {
		return Commit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value
			}
		}
	}
	return "unknown"
}

// Long is the version, commit and build date on one line, for the command
// line and the startup log.
func Long() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, shorten(FullCommit()), Date)
}

func shorten(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
