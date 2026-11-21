// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build unix

package backup

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockFileName is the file in the data directory the server holds a lock on.
const lockFileName = ".jim-dot-tennis.lock"

// Lock is an exclusive operating-system lock (flock) on the data directory's
// lock file. The server holds one for as long as it runs; restore takes one
// for as long as it works. The kernel releases it when the process ends, by
// any means (a crash, kill -9, the container stopping), so it never goes
// stale the way a PID file does, and it works between containers that share
// the data volume, since they share the kernel and the file.
type Lock struct {
	f *os.File
}

// ErrLocked: another process holds the lock (the server is running, or a
// restore is in progress).
var ErrLocked = errors.New("the data directory is in use by another process")

// AcquireLock takes the lock without waiting: ErrLocked if another process
// holds it.
func AcquireLock(path string) (*Lock, error) {
	// Read-only and world-readable (it is empty): a lock file created by a
	// restore run as root can still be opened by the app's own user.
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open the lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &Lock{f: f}, nil
}

// matchOwner gives paths the owner of ref, when this process runs as root
// (a restore run with `docker run --user root` must not leave the database
// unreadable to the app's own user). Otherwise the files are already owned
// by the user running restore, who is the app's user.
func matchOwner(ref string, paths ...string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	info, err := os.Stat(ref)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	for _, p := range paths {
		if err := os.Lchown(p, int(st.Uid), int(st.Gid)); err != nil {
			return fmt.Errorf("give %s to the app's user: %w", p, err)
		}
	}
	return nil
}

// Release gives the lock up. The file stays: removing it would let two
// processes hold locks on different files of the same name.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
	l.f = nil
}
