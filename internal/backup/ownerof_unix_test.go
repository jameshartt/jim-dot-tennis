// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build unix

package backup

import (
	"os"
	"syscall"
	"testing"
)

func ownerOf(t *testing.T, path string) (int, int) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	return int(st.Uid), int(st.Gid)
}
