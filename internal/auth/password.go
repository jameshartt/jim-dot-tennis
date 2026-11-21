// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// BcryptCost is the bcrypt work factor for every password hash the app
// writes (first-run setup, the admin users page). One constant so new hashes
// are never weaker in one place than another.
const BcryptCost = 12

// Password policy limits. bcrypt only reads the first 72 bytes of a password,
// so longer ones are refused rather than silently truncated.
const (
	MinPasswordLength = 10
	MaxPasswordBytes  = 72
)

// commonPasswords are refused outright even when they meet the length rule.
var commonPasswords = map[string]bool{
	"password12": true, "password123": true, "password1234": true,
	"1234567890": true, "0123456789": true, "qwertyuiop": true,
	"tennisclub": true, "tennis1234": true, "letmein123": true,
	"administrator": true, "changeme123": true, "welcome123": true,
}

// ErrPasswordPolicy is wrapped by every ValidatePassword error.
var ErrPasswordPolicy = errors.New("password does not meet the policy")

// ValidatePassword checks password against the policy for new passwords:
// at least MinPasswordLength characters, at most MaxPasswordBytes bytes, not
// the username, not a single repeated character and not a well-known
// password. The returned error's message is written for the person choosing
// the password.
func ValidatePassword(password, username string) error {
	fail := func(msg string) error { return fmt.Errorf("%w: %s", ErrPasswordPolicy, msg) }
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fail(fmt.Sprintf("Use at least %d characters.", MinPasswordLength))
	}
	if len(password) > MaxPasswordBytes {
		return fail(fmt.Sprintf("Use at most %d characters (fewer if you use accented letters or symbols).", MaxPasswordBytes))
	}
	lower := strings.ToLower(password)
	if username != "" && strings.Contains(lower, strings.ToLower(username)) {
		return fail("Don't include your username in the password.")
	}
	if _, size := utf8.DecodeRuneInString(password); strings.Count(password, password[:size])*size == len(password) {
		return fail("Don't use the same character over and over.")
	}
	if commonPasswords[lower] {
		return fail("That password is too common. Pick something less guessable.")
	}
	return nil
}

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
