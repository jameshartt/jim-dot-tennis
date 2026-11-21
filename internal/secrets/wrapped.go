// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package secrets holds the per-instance secrets the app needs and the
// startup self-check that keeps known placeholder or shared values out of
// production. Nothing here has a built-in default: a secret is either set by
// the instance's operator (environment) or generated with crypto/rand on the
// instance and stored in its own database.
package secrets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// EnvWrappedAccess overrides the stored Club Wrapped access code.
const EnvWrappedAccess = "WRAPPED_ACCESS_PASSWORD"

// SettingWrappedAccessCode is the app_settings key holding the generated
// Club Wrapped access code.
const SettingWrappedAccessCode = "wrapped_access_code"

// WrappedCookieName is the cookie that carries a granted Club Wrapped view.
const WrappedCookieName = "wrapped_access"

// codeAlphabet: upper-case letters and digits without the look-alikes 0/O
// and 1/I. 32 symbols, so a random byte mod 32 is uniform.
const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// codeLength is the number of symbols in a generated code (12 x 5 = 60 bits),
// shown as XXXX-XXXX-XXXX.
const codeLength = 12

// WrappedAccess is the Club Wrapped access code: WRAPPED_ACCESS_PASSWORD when
// set, otherwise a code generated on first use and stored in app_settings.
// The code is a low-friction gate shared with members, not an account
// credential, but it is unique to the instance.
type WrappedAccess struct {
	db     sqlx.ExtContext
	getenv func(string) string
}

// NewWrappedAccess returns the Club Wrapped access code store over db (a
// *database.DB or *sqlx.Tx).
func NewWrappedAccess(db sqlx.ExtContext) *WrappedAccess {
	return &WrappedAccess{db: db, getenv: os.Getenv}
}

// Code returns the effective access code and whether it comes from the
// environment. With no environment value and nothing stored yet, a code is
// generated and stored; concurrent first uses all end up with the same code.
func (w *WrappedAccess) Code(ctx context.Context) (code string, fromEnv bool, err error) {
	if v := strings.TrimSpace(w.getenv(EnvWrappedAccess)); v != "" {
		return v, true, nil
	}
	stored, err := w.stored(ctx)
	if err != nil {
		return "", false, err
	}
	if stored != "" {
		return stored, false, nil
	}
	generated, err := newCode()
	if err != nil {
		return "", false, fmt.Errorf("generate Club Wrapped access code: %w", err)
	}
	// Insert only if still absent, then read back whichever code won.
	if _, err := w.db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO NOTHING`, SettingWrappedAccessCode, generated, time.Now().UTC()); err != nil {
		return "", false, fmt.Errorf("store Club Wrapped access code: %w", err)
	}
	stored, err = w.stored(ctx)
	if err != nil {
		return "", false, err
	}
	if stored == "" {
		return "", false, errors.New("club Wrapped access code was not stored")
	}
	return stored, false, nil
}

func (w *WrappedAccess) stored(ctx context.Context) (string, error) {
	var v string
	err := sqlx.GetContext(ctx, w.db, &v, `SELECT value FROM app_settings WHERE key = ?`, SettingWrappedAccessCode)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read Club Wrapped access code: %w", err)
	}
	return strings.TrimSpace(v), nil
}

// Check reports whether presented matches the access code. Surrounding space
// is ignored; for a generated code, case, spaces and dashes are ignored too.
func (w *WrappedAccess) Check(ctx context.Context, presented string) (bool, error) {
	code, fromEnv, err := w.Code(ctx)
	if err != nil {
		return false, err
	}
	presented = strings.TrimSpace(presented)
	if presented == "" {
		return false, nil
	}
	if !fromEnv {
		code, presented = normalizeCode(code), normalizeCode(presented)
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(code)) == 1, nil
}

// CookieValue returns the value of the cookie that grants a Club Wrapped
// view. It is derived from the access code, so it differs per instance and
// changes when the code changes; it is not the code itself.
func (w *WrappedAccess) CookieValue(ctx context.Context) (string, error) {
	code, _, err := w.Code(ctx)
	if err != nil {
		return "", err
	}
	return cookieValueFor(code), nil
}

// CookieValid reports whether value is the current grant cookie value.
func (w *WrappedAccess) CookieValid(ctx context.Context, value string) bool {
	if value == "" {
		return false
	}
	want, err := w.CookieValue(ctx)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(want)) == 1
}

func cookieValueFor(code string) string {
	sum := sha256.Sum256([]byte("jim-dot-tennis club wrapped access v1\x00" + code))
	return hex.EncodeToString(sum[:])
}

// newCode returns a random code grouped as XXXX-XXXX-XXXX.
func newCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, v := range buf {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(codeAlphabet[int(v)%len(codeAlphabet)])
	}
	return b.String(), nil
}

// normalizeCode makes typed input comparable to a generated code.
func normalizeCode(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, strings.ToUpper(s))
}
