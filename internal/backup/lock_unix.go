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

// restoreLockFileName is the file a restore holds an exclusive lock on, and
// the account and backup commands a shared one (see AcquireRestoreLock).
const restoreLockFileName = ".jim-dot-tennis.restore.lock"

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
	return acquire(path, syscall.LOCK_EX)
}

// AcquireRestoreLock takes the restore lock (Config.RestoreLockPath) for a
// restore, exclusively, without waiting: ErrLocked if a command or another
// restore holds it.
//
// Why a second lock: the server holds the main lock for its whole life, and
// the account commands (`user ...`, `admin-passkeys`, `setup code`) and
// `backup` are meant to run alongside it, so they cannot take the main lock.
// They take this one shared instead (AcquireCommandLock), which any number
// of them, and the server, can hold at once; a restore takes it exclusively,
// so a command refuses while a restore runs (rather than writing to a
// database that is about to be replaced), and a restore refuses while a
// command is still working.
func AcquireRestoreLock(path string) (*Lock, error) {
	return acquire(path, syscall.LOCK_EX)
}

// AcquireCommandLock takes the restore lock shared, without waiting:
// ErrLocked while a restore holds it.
func AcquireCommandLock(path string) (*Lock, error) {
	return acquire(path, syscall.LOCK_SH)
}

func acquire(path string, how int) (*Lock, error) {
	// Read-only and world-readable (it is empty): a lock file created by a
	// restore run as root can still be opened by the app's own user.
	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open the lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB); err != nil {
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

// freeSpace is the space available to this user on the file system holding
// dir; ok is false when it cannot be told.
func freeSpace(dir string) (bytes uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	// The field types differ between platforms; Bsize is never negative.
	return uint64(st.Bavail) * uint64(st.Bsize), true
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
