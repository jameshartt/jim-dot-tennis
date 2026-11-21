// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build unix

package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/backup"
)

// While a restore runs, the account and backup commands refuse (they would
// write to a database about to be replaced); with only the server running
// they work; and a restore refuses while such a command is working.
func TestCLICommandsRefuseDuringRestore(t *testing.T) {
	cliTestDB(t)
	t.Setenv(backup.EnvDir, "")
	t.Setenv(backup.EnvKeep, "")
	t.Setenv(backup.EnvInterval, "")
	cfg, err := backup.FromEnv()
	if err != nil {
		t.Fatal(err)
	}

	server, err := backup.AcquireLock(cfg.LockPath())
	if err != nil {
		t.Fatal(err)
	}
	if r := runCLITest(t, "user", "list"); r.code != exitOK {
		t.Fatalf("user list beside the running server: %+v", r)
	}
	server.Release()

	restoring, err := backup.AcquireRestoreLock(cfg.RestoreLockPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"user", "list"}, {"user", "create", "newcaptain", "--role", "captain"}, {"admin-passkeys", "status"}, {"backup"}} {
		r := runCLITest(t, args...)
		if r.code != exitError || !strings.Contains(r.stderr, "a restore is in progress") {
			t.Errorf("%v during a restore: %+v", args, r)
		}
	}
	restoring.Release()
	if r := runCLITest(t, "user", "list"); r.code != exitOK || strings.Contains(r.stdout, "newcaptain") {
		t.Errorf("user list after the restore: %+v", r)
	}

	command, err := backup.AcquireCommandLock(cfg.RestoreLockPath())
	if err != nil {
		t.Fatal(err)
	}
	defer command.Release()
	r := runCLITest(t, "restore", "/nonexistent.tar.gz")
	if r.code != exitError || !strings.Contains(r.stderr, "a command") {
		t.Errorf("restore while a command runs: %+v", r)
	}
}

// Ctrl-C (SIGINT) during a restore cancels its context instead of killing
// the process, so the restore can clean up.
func TestRestoreContextCatchesInterrupt(t *testing.T) {
	ctx, stop := restoreContext(context.Background())
	defer stop()
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the interrupt did not cancel the restore's context")
	}
}
