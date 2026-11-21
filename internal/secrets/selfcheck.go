// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package secrets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// SecretEnvVars are the environment variables the app reads that hold a
// secret. The production self-check refuses a known placeholder in any of
// them. (CourtHive's JWT secret is read by the CourtHive server, not by this
// app; see WI-144.)
var SecretEnvVars = []string{
	EnvWrappedAccess,
}

// knownPlaceholders are values that appear as examples in this repository's
// .env.example files and documentation, or are otherwise generic defaults.
// Compared case-insensitively after trimming.
var knownPlaceholders = []string{
	"change-me",
	"changeme",
	"change_me",
	"example",
	"password",
	"secret",
	"placeholder",
	"your-password",
	"your-password-here",
	"your-secure-password",
	"<wrapped-access-password>",
}

// IsPlaceholder reports whether v is a known placeholder or default value.
func IsPlaceholder(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "<") && strings.HasSuffix(v, ">") {
		return true // a <placeholder> copied from the docs
	}
	for _, p := range knownPlaceholders {
		if v == p {
			return true
		}
	}
	return false
}

// CheckEnv returns an error naming every secret variable set to a known
// placeholder, when production is true. Values are never included.
func CheckEnv(production bool, getenv func(string) string) error {
	if !production {
		return nil
	}
	var bad []string
	for _, name := range SecretEnvVars {
		if IsPlaceholder(getenv(name)) {
			bad = append(bad, name)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("refusing to start in production: %s set to a placeholder value from the documentation. "+
		"Set a value of your own, or leave it unset so the app generates one for this instance",
		strings.Join(bad, ", "))
}

// knownCredentialFingerprints are SHA-256 fingerprints of password hashes
// whose passwords are known outside any one club: the hashes that early
// versions of the migrations inserted into every new database (accounts
// created by the original author), and the end-to-end test administrator's
// published password. Only fingerprints are kept, so neither the hashes nor
// the account names are in the source.
var knownCredentialFingerprints = map[string]string{
	"06456ebb89a24e2fb0a01f4e5f02254664a0859c377fc00b650e8f20f55372d5": "seeded by an early migration",
	"a66ea3d00def506f6813b846174ba5f1909397631f0cf870b3a0a3b0e6b95982": "seeded by an earlier migration 004",
	"ecc70b1c3299dad86ed9365eaa2b6f9eefde8c223824007740e6427e6fdf306c": "seeded by an earlier migration 004",
	"545bdfba4f3cf684009747dca5b31fe7e6afff5c5e6f4347507a1e189848b6cf": "the published end-to-end test password",
}

func fingerprint(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(sum[:])
}

// KnownCredentialAccounts returns the usernames of accounts whose password
// hash is one whose password is known outside this instance (see
// knownCredentialFingerprints). Such accounts exist only in databases created
// before the seeded accounts were removed, or seeded with test data.
func KnownCredentialAccounts(ctx context.Context, db sqlx.QueryerContext) ([]string, error) {
	var rows []struct {
		Username     string `db:"username"`
		PasswordHash string `db:"password_hash"`
	}
	if err := sqlx.SelectContext(ctx, db, &rows, `SELECT username, password_hash FROM users`); err != nil {
		return nil, fmt.Errorf("check accounts for known passwords: %w", err)
	}
	var names []string
	for _, r := range rows {
		if _, ok := knownCredentialFingerprints[fingerprint(r.PasswordHash)]; ok {
			names = append(names, r.Username)
		}
	}
	sort.Strings(names)
	return names, nil
}

// StartupCheck runs at server start. In production (APP_ENV=production) it
// returns an error if a secret variable holds a known placeholder; the caller
// must refuse to start. In every mode it makes sure the instance has its own
// Club Wrapped access code (generating one if needed, without logging it) and
// warns about accounts whose password is known outside the instance. It
// never changes an account.
func StartupCheck(ctx context.Context, db sqlx.ExtContext) error {
	production := os.Getenv("APP_ENV") == "production"
	if err := CheckEnv(production, os.Getenv); err != nil {
		return err
	}

	wa := NewWrappedAccess(db)
	if _, fromEnv, err := wa.Code(ctx); err != nil {
		log.Printf("Warning: %v", err)
	} else if fromEnv {
		log.Printf("Club Wrapped access code: set by %s", EnvWrappedAccess)
	} else {
		log.Printf("Club Wrapped access code: generated for this instance; administrators can see it on the Club Settings page")
	}

	names, err := KnownCredentialAccounts(ctx, db)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else if len(names) > 0 {
		log.Printf("WARNING: %d account(s) still use a password that is known outside this instance "+
			"(created by an old version of the database migrations, or by test data): %s. "+
			"Change their passwords or remove them on the admin users page.",
			len(names), strings.Join(names, ", "))
	}
	return nil
}
