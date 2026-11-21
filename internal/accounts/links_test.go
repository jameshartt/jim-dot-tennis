// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package accounts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
)

// testClock is a settable clock for the service.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func withClock(s *Service) *testClock {
	c := &testClock{now: time.Now()}
	s.now = c.Now
	return c
}

func countRows(t *testing.T, db *database.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.Get(&n, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// addPasskey stores a passkey row for userID directly.
func addPasskey(t *testing.T, db *database.DB, userID int64, id string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO webauthn_credentials (user_id, credential_id, public_key, name) VALUES (?, ?, x'00', ?)`,
		userID, []byte(id), id); err != nil {
		t.Fatal(err)
	}
}

// newPasskey is a verified-but-unstored passkey as auth.Passkeys hands one over.
func newPasskey(userID int64, id string) *models.WebAuthnCredential {
	return &models.WebAuthnCredential{UserID: userID, CredentialID: []byte(id), PublicKey: []byte{1}, Name: id, CreatedAt: time.Now()}
}

func TestInviteMakesAPasswordlessAccountAndAHashedLink(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	logs := captureLog(t)

	a, link, err := s.Invite(ctx, UserActor(7, "admin"), "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.SignInMethods() != 0 || a.HasPassword || !a.IsActive || a.Role != models.RoleCaptain {
		t.Fatalf("invited account: %+v", a)
	}
	if passwordHash(t, db, a.ID) != "" {
		t.Error("an invited account has a password hash")
	}
	// At least 128 bits from crypto/rand: the token is 32 random bytes.
	raw, err := base64.RawURLEncoding.DecodeString(link.Token)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token %d bytes (%v), want 32", len(raw), err)
	}
	if _, other, _ := s.Invite(ctx, cli, "sam", models.RoleCaptain, nil); other.Token == link.Token {
		t.Fatal("two links share a token")
	}
	// Only the hash is stored: the token appears nowhere in the table.
	sum := sha256.Sum256([]byte(link.Token))
	var stored struct {
		Hash      string `db:"token_hash"`
		CreatedBy string `db:"created_by"`
	}
	if err := db.Get(&stored, `SELECT token_hash, created_by FROM account_links WHERE id = ?`, link.ID); err != nil {
		t.Fatal(err)
	}
	if stored.Hash != hex.EncodeToString(sum[:]) || stored.CreatedBy != "user:admin(#7)" {
		t.Errorf("stored link: %+v", stored)
	}
	var dumpAll []string
	if err := db.Select(&dumpAll, `SELECT id || '|' || user_id || '|' || kind || '|' || token_hash || '|' || created_by FROM account_links`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(dumpAll, "\n"), link.Token) {
		t.Error("the token itself is stored")
	}
	if link.Path() != "/account/link/"+link.Token || link.Kind != LinkInvite {
		t.Errorf("link: %+v", link.Link)
	}
	if got := link.ExpiresAt.Sub(link.CreatedAt); got != InviteLinkLifetime || InviteLinkLifetime != 48*time.Hour {
		t.Errorf("invite lifetime %v", got)
	}
	// The audit record names actor and target and never the token.
	out := logs.String()
	if !strings.Contains(out, "action=link.create") || !strings.Contains(out, `actor="user:admin(#7)"`) || !strings.Contains(out, `target="pat"`) || !strings.Contains(out, "kind=invite") {
		t.Errorf("audit record for the invite: %q", out)
	}
	if strings.Contains(out, link.Token) || strings.Contains(out, stored.Hash) {
		t.Error("the audit record contains the token or its hash")
	}
	// No password means no password sign-in, whatever is typed.
	authService := auth.NewService(db, auth.DefaultConfig())
	for _, pw := range []string{"", "x", goodPassword} {
		if _, err := authService.Login("pat", pw, httptest.NewRequest(http.MethodPost, "/login", nil)); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("password sign-in with %q for a passwordless account: %v", pw, err)
		}
	}
}

func TestInviteRules(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	mustCreate(t, s, "Alice", models.RoleAdmin)
	users := func() int { return countRows(t, db, `SELECT COUNT(*) FROM users`) }
	before := users()

	if _, _, err := s.Invite(ctx, cli, "alice", models.RolePlayer, nil); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("case-insensitive duplicate: err %v", err)
	}
	var bad *InvalidUsernameError
	if _, _, err := s.Invite(ctx, cli, "no spaces", models.RolePlayer, nil); !errors.As(err, &bad) {
		t.Errorf("bad username: err %v", err)
	}
	if _, _, err := s.Invite(ctx, cli, "pat", models.Role("owner"), nil); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("bad role: err %v", err)
	}
	missing := "no-such-player"
	if _, _, err := s.Invite(ctx, cli, "pat", models.RoleCaptain, &missing); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("unknown player: err %v", err)
	}
	if users() != before || countRows(t, db, `SELECT COUNT(*) FROM account_links WHERE used_at IS NULL`) != 0 {
		t.Fatal("a refused invite left an account or a link behind")
	}

	if _, err := db.Exec(`INSERT INTO clubs (id, name) VALUES (1, 'Club')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO players (id, first_name, last_name, club_id) VALUES ('p1', 'Pat', 'Player', 1)`); err != nil {
		t.Fatal(err)
	}
	player := "p1"
	a, _, err := s.Invite(ctx, cli, "pat", models.RoleCaptain, &player)
	if err != nil || a.PlayerID == nil || *a.PlayerID != "p1" {
		t.Fatalf("invite linked to a player: %+v %v", a, err)
	}
	if _, _, err := s.Invite(ctx, cli, "pat2", models.RoleCaptain, &player); !errors.Is(err, ErrPlayerLinked) {
		t.Errorf("player already linked: err %v", err)
	}
	if found, err := s.Find(ctx, "PAT"); err != nil || found.ID != a.ID {
		t.Errorf("find ignoring case: %+v %v", found, err)
	}

	// An invite is only for an account that cannot sign in yet.
	alice, _ := s.Find(ctx, "alice")
	if _, err := s.IssueLink(ctx, cli, alice.ID, LinkInvite); !errors.Is(err, ErrInviteHasSignIn) {
		t.Errorf("invite for an account with a password: err %v", err)
	}
	if _, err := s.IssueLink(ctx, cli, a.ID, LinkInvite); err != nil {
		t.Errorf("a second invite for a pending account: %v", err)
	}
	if _, err := s.IssueLink(ctx, cli, 4040, LinkRecovery); !errors.Is(err, ErrNotFound) {
		t.Errorf("link for an unknown account: err %v", err)
	}
}

func TestLinkExpiry(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	clock := withClock(s)
	alice := mustCreate(t, s, "alice", models.RoleAdmin)

	_, invite, err := s.Invite(ctx, cli, "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if got := recovery.ExpiresAt.Sub(recovery.CreatedAt); got != time.Hour {
		t.Fatalf("recovery lifetime %v, want 1h", got)
	}

	clock.advance(time.Hour - time.Second)
	if _, err := s.LookupLink(ctx, recovery.Token); err != nil {
		t.Fatalf("recovery link one second before expiry: %v", err)
	}
	clock.advance(time.Second)
	if _, err := s.LookupLink(ctx, recovery.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("recovery link at expiry: err %v", err)
	}
	if _, err := s.RedeemLink(ctx, recovery.Token, Redemption{Password: "a brand new secret"}); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("redeem an expired recovery link: err %v", err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, alice.ID), goodPassword) {
		t.Fatal("an expired link changed the password")
	}

	clock.advance(47*time.Hour - time.Second) // 48h less a second after issue
	if _, err := s.LookupLink(ctx, invite.Token); err != nil {
		t.Fatalf("invite one second before expiry: %v", err)
	}
	if pending, err := s.PendingLinks(ctx); err != nil || len(pending) != 1 || pending[0].ID != invite.ID {
		t.Fatalf("pending links: %+v %v, want only the invite", pending, err)
	}
	clock.advance(time.Second)
	if _, err := s.RedeemLink(ctx, invite.Token, Redemption{Password: goodPassword}); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("redeem an expired invite: err %v", err)
	}
	if pending, _ := s.PendingLinks(ctx); len(pending) != 0 {
		t.Fatalf("expired links still listed as pending: %+v", pending)
	}
}

func TestLinkWorksOnce(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	logs := captureLog(t)
	_, link, err := s.Invite(ctx, cli, "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}

	// A password that does not meet the policy leaves the link unused.
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "short"}); !errors.Is(err, auth.ErrPasswordPolicy) {
		t.Fatalf("weak password: err %v", err)
	}
	if _, err := s.LookupLink(ctx, link.Token); err != nil {
		t.Fatalf("link after a refused password: %v", err)
	}

	done, err := s.RedeemLink(ctx, link.Token, Redemption{Password: goodPassword, RemoveOtherMethods: true})
	if err != nil {
		t.Fatal(err)
	}
	if done.Method != "password" || done.Link.UserID != link.UserID || done.Link.UsedAt == nil {
		t.Errorf("redeemed: %+v", done)
	}
	if !auth.PasswordMatches(passwordHash(t, db, link.UserID), goodPassword) {
		t.Fatal("password not set")
	}
	for _, how := range []Redemption{{Password: "another good secret"}, {Passkey: newPasskey(link.UserID, "k")}} {
		if _, err := s.RedeemLink(ctx, link.Token, how); !errors.Is(err, ErrLinkInvalid) {
			t.Fatalf("second use: err %v, want ErrLinkInvalid", err)
		}
	}
	if _, err := s.LookupLink(ctx, link.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("look up a used link: err %v", err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, link.UserID), goodPassword) || countRows(t, db, `SELECT COUNT(*) FROM webauthn_credentials`) != 0 {
		t.Fatal("a second use changed the account")
	}
	out := logs.String()
	if !strings.Contains(out, "action=link.use") || !strings.Contains(out, `actor="user:pat(#`) || !strings.Contains(out, `target="pat"`) || !strings.Contains(out, "method=password") {
		t.Errorf("audit record for the use: %q", out)
	}
	if strings.Contains(out, link.Token) || strings.Contains(out, goodPassword) {
		t.Error("the log contains the token or the password")
	}
	// Tokens of the wrong shape are refused without a lookup.
	for _, token := range []string{"", "x", link.Token + "A", link.Token[:42], strings.Repeat("!", 43)} {
		if _, err := s.LookupLink(ctx, token); !errors.Is(err, ErrLinkInvalid) {
			t.Errorf("malformed token %q: err %v", token, err)
		}
	}
}

// Many requests racing on one link: exactly one wins, and the account ends
// up with the winner's sign-in method only.
func TestLinkConcurrentRedemption(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	for round, usePasskeys := range []bool{false, true} {
		_, link, err := s.Invite(ctx, cli, fmt.Sprintf("racer%d", round), models.RoleCaptain, nil)
		if err != nil {
			t.Fatal(err)
		}
		const racers = 8
		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			winners []int
			others  []error
			start   = make(chan struct{})
		)
		for i := 0; i < racers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				how := Redemption{Password: fmt.Sprintf("racing password %d", i)}
				if usePasskeys {
					how = Redemption{Passkey: newPasskey(link.UserID, fmt.Sprintf("round%d-key%d", round, i))}
				}
				<-start
				_, err := s.RedeemLink(ctx, link.Token, how)
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					winners = append(winners, i)
				} else {
					others = append(others, err)
				}
			}(i)
		}
		close(start)
		wg.Wait()

		if len(winners) != 1 {
			t.Fatalf("round %d: %d redemptions succeeded, want exactly 1 (errors: %v)", round, len(winners), others)
		}
		for _, err := range others {
			if !errors.Is(err, ErrLinkInvalid) {
				t.Errorf("round %d: a losing redemption failed with %v, want ErrLinkInvalid", round, err)
			}
		}
		if usePasskeys {
			var names []string
			if err := db.Select(&names, `SELECT name FROM webauthn_credentials WHERE user_id = ?`, link.UserID); err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprintf("round%d-key%d", round, winners[0]); len(names) != 1 || names[0] != want {
				t.Errorf("round %d: passkeys stored %v, want only %s", round, names, want)
			}
		} else if !auth.PasswordMatches(passwordHash(t, db, link.UserID), fmt.Sprintf("racing password %d", winners[0])) {
			t.Errorf("round %d: the stored password is not the winner's", round)
		}
		if n := countRows(t, db, `SELECT COUNT(*) FROM account_links WHERE user_id = ? AND used_at IS NOT NULL`, link.UserID); n != 1 {
			t.Errorf("round %d: %d links marked used", round, n)
		}
	}
}

func TestLinkRevocation(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	logs := captureLog(t)
	alice := mustCreate(t, s, "alice", models.RoleAdmin)

	first, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := s.RevokeLink(ctx, UserActor(alice.ID, "alice"), first.ID)
	if err != nil || revoked.RevokedAt == nil || revoked.Username != "alice" {
		t.Fatalf("revoke: %+v %v", revoked, err)
	}
	if _, err := s.LookupLink(ctx, first.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("look up a revoked link: err %v", err)
	}
	if _, err := s.RedeemLink(ctx, first.Token, Redemption{Password: "a brand new secret"}); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("redeem a revoked link: err %v", err)
	}
	if _, err := s.RevokeLink(ctx, cli, first.ID); !errors.Is(err, ErrLinkNotPending) {
		t.Fatalf("revoke twice: err %v", err)
	}
	if _, err := s.RevokeLink(ctx, cli, 4040); !errors.Is(err, ErrLinkNotPending) {
		t.Fatalf("revoke an unknown link: err %v", err)
	}
	if out := logs.String(); !strings.Contains(out, "action=link.revoke") || !strings.Contains(out, `target="alice"`) || strings.Contains(out, first.Token) {
		t.Errorf("audit record for the revoke: %q", out)
	}

	// A new link for the same account cancels the earlier unused one.
	second, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil || third.Superseded != 1 {
		t.Fatalf("third link: superseded %d, %v", third.Superseded, err)
	}
	if _, err := s.LookupLink(ctx, second.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("superseded link still works: err %v", err)
	}
	if _, err := s.LookupLink(ctx, third.Token); err != nil {
		t.Fatalf("newest link: %v", err)
	}
	pending, err := s.PendingLinks(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != third.ID {
		t.Fatalf("pending links: %+v %v", pending, err)
	}
	// Another account's link is not touched by any of this.
	_, other, err := s.Invite(ctx, cli, "pat", models.RoleCaptain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LookupLink(ctx, other.Token); err != nil {
		t.Fatalf("another account's link was cancelled: %v", err)
	}
	// Revoking never changed the account.
	if !auth.PasswordMatches(passwordHash(t, db, alice.ID), goodPassword) {
		t.Fatal("revoking a link changed the password")
	}
}

// Issuing a recovery link must not change the account (or any administrator's
// misclick would lock a user out); using it ends every session.
func TestRecoveryLink(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	alice := mustCreate(t, s, "alice", models.RoleAdmin)
	bob := mustCreate(t, s, "bob", models.RoleCaptain)
	addPasskey(t, db, alice.ID, "old-phone")
	addPasskey(t, db, alice.ID, "old-laptop")
	addPasskey(t, db, bob.ID, "bob-phone")
	addSession(t, db, "a1", alice.ID)
	addSession(t, db, "a2", alice.ID)
	addSession(t, db, "b1", bob.ID)
	passkeys := func(userID int64) int {
		return countRows(t, db, `SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, userID)
	}

	link, err := s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if link.Kind != LinkRecovery || !auth.PasswordMatches(passwordHash(t, db, alice.ID), goodPassword) ||
		validSessions(t, db, alice.ID) != 2 || passkeys(alice.ID) != 2 {
		t.Fatal("issuing a recovery link changed the account")
	}
	if _, err := s.LookupLink(ctx, link.Token); err != nil {
		t.Fatal(err)
	}
	if validSessions(t, db, alice.ID) != 2 {
		t.Fatal("looking a link up ended sessions")
	}

	// Used with a password, keeping the old methods: every session ends, the
	// password is replaced, the passkeys stay.
	done, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "a brand new secret"})
	if err != nil {
		t.Fatal(err)
	}
	if done.SessionsEnded != 2 || validSessions(t, db, alice.ID) != 0 {
		t.Errorf("sessions after recovery: ended %d, %d still valid", done.SessionsEnded, validSessions(t, db, alice.ID))
	}
	if validSessions(t, db, bob.ID) != 1 {
		t.Error("another user's session was ended")
	}
	hash := passwordHash(t, db, alice.ID)
	if !auth.PasswordMatches(hash, "a brand new secret") || auth.PasswordMatches(hash, goodPassword) {
		t.Error("the password was not replaced")
	}
	if passkeys(alice.ID) != 2 || done.PasskeysRemoved != 0 {
		t.Error("passkeys were removed without being asked")
	}

	// Used with a passkey and "remove my old ways of signing in": only the
	// new passkey is left, and the password is gone.
	addSession(t, db, "a3", alice.ID)
	link, err = s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	done, err = s.RedeemLink(ctx, link.Token, Redemption{Passkey: newPasskey(alice.ID, "new-phone"), RemoveOtherMethods: true})
	if err != nil {
		t.Fatal(err)
	}
	if done.Method != "passkey" || done.PasskeysRemoved != 2 || !done.PasswordRemoved || done.SessionsEnded != 1 {
		t.Errorf("redeemed: %+v", done)
	}
	var names []string
	if err := db.Select(&names, `SELECT name FROM webauthn_credentials WHERE user_id = ?`, alice.ID); err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "new-phone" || passwordHash(t, db, alice.ID) != "" {
		t.Errorf("after removing old methods: passkeys %v, password hash %q", names, passwordHash(t, db, alice.ID))
	}
	if a, _ := s.Get(ctx, alice.ID); a.SignInMethods() != 1 || !a.IsActiveAdmin() {
		t.Errorf("account after recovery: %+v", a)
	}
	if passkeys(bob.ID) != 1 || !auth.PasswordMatches(passwordHash(t, db, bob.ID), goodPassword) {
		t.Error("another user's sign-in methods were touched")
	}

	// Used with a password and "remove": the passkeys go, the password is set.
	link, err = s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "third time lucky", RemoveOtherMethods: true}); err != nil {
		t.Fatal(err)
	}
	if passkeys(alice.ID) != 0 || !auth.PasswordMatches(passwordHash(t, db, alice.ID), "third time lucky") {
		t.Error("password + remove did not leave only the new password")
	}

	// A passkey verified for another account is refused and the link survives.
	link, err = s.IssueLink(ctx, cli, alice.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Passkey: newPasskey(bob.ID, "sneaky")}); err == nil {
		t.Fatal("a passkey for another account was accepted")
	}
	// A passkey that is already enrolled (by anyone) is refused; the link
	// and the account are as they were.
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Passkey: newPasskey(alice.ID, "bob-phone"), RemoveOtherMethods: true}); err == nil {
		t.Fatal("a duplicate passkey was accepted")
	}
	if _, err := s.LookupLink(ctx, link.Token); err != nil {
		t.Fatalf("link after refused redemptions: %v", err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, alice.ID), "third time lucky") {
		t.Error("a failed redemption removed the password")
	}
}

func TestDisabledAccountCannotUseALink(t *testing.T) {
	ctx := context.Background()
	s, db := newTestService(t)
	mustCreate(t, s, "alice", models.RoleAdmin)
	bob := mustCreate(t, s, "bob", models.RoleCaptain)

	link, err := s.IssueLink(ctx, cli, bob.ID, LinkRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetActive(ctx, cli, bob.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LookupLink(ctx, link.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("look up a disabled account's link: err %v", err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "a brand new secret"}); !errors.Is(err, ErrLinkInvalid) {
		t.Fatalf("redeem a disabled account's link: err %v", err)
	}
	if !auth.PasswordMatches(passwordHash(t, db, bob.ID), goodPassword) {
		t.Fatal("a disabled account's password was changed")
	}
	// A link can be issued for a disabled account but only works once the
	// account is enabled.
	link, err = s.IssueLink(ctx, cli, bob.ID, LinkRecovery)
	if err != nil {
		t.Fatalf("issue for a disabled account: %v", err)
	}
	if _, err := s.LookupLink(ctx, link.Token); !errors.Is(err, ErrLinkInvalid) {
		t.Fatal("a disabled account's new link works")
	}
	if _, err := s.SetActive(ctx, cli, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemLink(ctx, link.Token, Redemption{Password: "a brand new secret"}); err != nil {
		t.Fatalf("redeem after enabling: %v", err)
	}
}

// Neither List nor Get hands a password hash to the pages or the command line.
func TestAccountsCarryNoPasswordHash(t *testing.T) {
	s, _ := newTestService(t)
	a := mustCreate(t, s, "alice", models.RoleAdmin)
	list, err := s.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if text := fmt.Sprintf("%+v %+v", a, list); strings.Contains(text, "$2a$") || strings.Contains(text, "$2b$") {
		t.Fatalf("an account value contains a password hash: %s", text)
	}
	if !a.HasPassword || a.SignInMethods() != 1 {
		t.Errorf("account: %+v", a)
	}
}
