// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build !unix

package backup

import "errors"

const lockFileName = ".jim-dot-tennis.lock"

// Lock is not available on this platform (releases are built for Linux).
type Lock struct{}

// ErrLocked: another process holds the lock.
var ErrLocked = errors.New("the data directory is in use by another process")

// AcquireLock always fails here: without a lock, restore cannot tell whether
// the server is running, so neither runs.
func AcquireLock(string) (*Lock, error) {
	return nil, errors.New("the data directory lock needs Linux or another Unix system")
}

func matchOwner(string, ...string) error { return nil }

// Release does nothing.
func (l *Lock) Release() {}
