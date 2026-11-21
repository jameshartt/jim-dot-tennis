// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build !unix

package backup

import "errors"

const (
	lockFileName        = ".jim-dot-tennis.lock"
	restoreLockFileName = ".jim-dot-tennis.restore.lock"
)

// Lock is not available on this platform (releases are built for Linux).
type Lock struct{}

// ErrLocked: another process holds the lock.
var ErrLocked = errors.New("the data directory is in use by another process")

var errNoLocks = errors.New("the data directory lock needs Linux or another Unix system")

// AcquireLock always fails here: without a lock, restore cannot tell whether
// the server is running, so neither runs.
func AcquireLock(string) (*Lock, error) { return nil, errNoLocks }

// AcquireRestoreLock always fails here, so restore does not run.
func AcquireRestoreLock(string) (*Lock, error) { return nil, errNoLocks }

// AcquireCommandLock does nothing here: restore cannot run on this platform,
// so there is nothing for a command to wait for.
func AcquireCommandLock(string) (*Lock, error) { return &Lock{}, nil }

func matchOwner(string, ...string) error { return nil }

func freeSpace(string) (uint64, bool) { return 0, false }

// Release does nothing.
func (l *Lock) Release() {}
