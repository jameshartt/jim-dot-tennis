// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt work factor for every password hash the app
// writes (first-run setup, invite and recovery links, the sign-in methods
// page). It is the one place the cost is set: a stored hash with a lower
// cost is upgraded the next time its owner signs in with the password
// (NeedsRehash, Service.Login).
const BcryptCost = 12

// Password policy limits. bcrypt only reads the first 72 bytes of a password,
// so longer ones are refused rather than silently truncated.
const (
	MinPasswordLength = 12
	MaxPasswordBytes  = 72
)

// commonPasswordList is the short list of well-known passwords refused even
// when they are long enough. It is compiled in; nothing is fetched.
//
//go:embed common_passwords.txt
var commonPasswordList string

// commonPasswords holds commonPasswordList in the form commonKey compares.
var commonPasswords = func() map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(commonPasswordList, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[commonKey(line)] = true
	}
	return out
}()

// commonKey is how a password is compared with the common list: lower case,
// with spaces removed, so "Password 1234" is caught as "password1234".
func commonKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, s)
}

// IsCommonPassword reports whether password is on the common-password list.
func IsCommonPassword(password string) bool { return commonPasswords[commonKey(password)] }

// ErrPasswordPolicy is wrapped by every ValidatePassword error.
var ErrPasswordPolicy = errors.New("password does not meet the policy")

// Reasons a password is refused (PasswordPolicyError.Code). They are short
// and safe to put in a URL; PasswordPolicyMessage turns one into a sentence.
const (
	PolicyTooShort = "short"
	PolicyTooLong  = "long"
	PolicyUsername = "username"
	PolicyRepeated = "repeated"
	PolicyCommon   = "common"
	PolicySame     = "same" // the new password is the current one
)

// PasswordPolicyError is the error ValidatePassword returns. It wraps
// ErrPasswordPolicy; Error() is "<ErrPasswordPolicy>: <message for the person>".
type PasswordPolicyError struct{ Code string }

func (e *PasswordPolicyError) Error() string {
	return ErrPasswordPolicy.Error() + ": " + PasswordPolicyMessage(e.Code)
}

func (e *PasswordPolicyError) Unwrap() error { return ErrPasswordPolicy }

// PasswordPolicyMessage is the sentence shown to the person choosing a
// password for a PasswordPolicyError code ("" for an unknown code).
func PasswordPolicyMessage(code string) string {
	switch code {
	case PolicyTooShort:
		return fmt.Sprintf("Use at least %d characters.", MinPasswordLength)
	case PolicyTooLong:
		return fmt.Sprintf("Use at most %d characters (fewer if you use accented letters or symbols).", MaxPasswordBytes)
	case PolicyUsername:
		return "Don't include your username in the password."
	case PolicyRepeated:
		return "Don't use the same character over and over."
	case PolicyCommon:
		return "That password is too common. Pick something less guessable."
	case PolicySame:
		return "Choose a password different from the current one."
	}
	return ""
}

// PolicyMessage returns the sentence for the person when err is a password
// policy error, and ok=false otherwise.
func PolicyMessage(err error) (string, bool) {
	var pe *PasswordPolicyError
	if errors.As(err, &pe) {
		return PasswordPolicyMessage(pe.Code), true
	}
	return "", false
}

// ValidatePassword checks password against the policy for new and changed
// passwords: at least MinPasswordLength characters, at most MaxPasswordBytes
// bytes, not containing the username, not a single repeated character and
// not on the common-password list. There are deliberately no composition
// rules (no "must contain a digit"). The policy is only for passwords being
// chosen: a stored password that predates it keeps working. The error is a
// *PasswordPolicyError.
func ValidatePassword(password, username string) error {
	fail := func(code string) error { return &PasswordPolicyError{Code: code} }
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fail(PolicyTooShort)
	}
	if len(password) > MaxPasswordBytes {
		return fail(PolicyTooLong)
	}
	lower := strings.ToLower(password)
	if username != "" && strings.Contains(lower, strings.ToLower(username)) {
		return fail(PolicyUsername)
	}
	if _, size := utf8.DecodeRuneInString(password); strings.Count(password, password[:size])*size == len(password) {
		return fail(PolicyRepeated)
	}
	if IsCommonPassword(password) {
		return fail(PolicyCommon)
	}
	return nil
}

// SamePasswordError is the policy error for a new password that is the
// current one.
func SamePasswordError() error { return &PasswordPolicyError{Code: PolicySame} }

// HashPassword returns the bcrypt hash of password at BcryptCost.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// PasswordMatches reports whether password matches the bcrypt hash.
func PasswordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NeedsRehash reports whether hash was made with a lower cost than
// BcryptCost (or cannot be read), so it should be replaced the next time its
// password is known.
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	return err != nil || cost < BcryptCost
}
