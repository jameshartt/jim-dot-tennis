// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package secrets

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// deploy/init-tournaments.sh checks every line before it writes anything:
// a leftover placeholder for the LAST secret it would add still leaves .env
// exactly as it was (it used to write five secrets first, then refuse).
func TestInitTournamentsRefusesPlaceholderBeforeWriting(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	script := filepath.Join(repoDir(t, "deploy"), "init-tournaments.sh")
	for _, line := range []string{"COURTHIVE_JWT_SECRET=change-me", "COURTHIVE_PG_PASSWORD=", "export COURTHIVE_SIGNER_TOKEN='<token>'"} {
		envFile := filepath.Join(t.TempDir(), ".env")
		before := []byte("DOMAIN=tennis.example.org\n" + line + "\n")
		if err := os.WriteFile(envFile, before, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(sh, script, envFile).CombinedOutput()
		if err == nil {
			t.Fatalf("%s: the script succeeded; want a refusal. Output:\n%s", line, out)
		}
		name := strings.TrimPrefix(strings.SplitN(line, "=", 2)[0], "export ")
		if !strings.Contains(string(out), name+" is in") || !strings.Contains(string(out), "no real value") {
			t.Errorf("%s: output does not name the line: %s", line, out)
		}
		if strings.Contains(string(out), "added") {
			t.Errorf("%s: the script added something before refusing: %s", line, out)
		}
		after, err := os.ReadFile(envFile)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(after, before) {
			t.Errorf("%s: .env changed before the refusal:\n%s", line, after)
		}
	}
}
