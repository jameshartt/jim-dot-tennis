// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"jim-dot-tennis/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// Regression tests for the findings of the WI-134 security review.

func loginRequest(ip string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/login", nil)
	r.RemoteAddr = ip + ":1234"
	return r
}

// F1: guesses sent at the same moment used to all read "not throttled yet"
// and all be checked. Now at most the limit are checked, however many arrive
// together, and a right password is never recorded as a failure.
func TestLoginThrottleHoldsUnderConcurrency(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("realuser", models.RoleCaptain)
	const ip = "203.0.113.50"

	var wg sync.WaitGroup
	var mu sync.Mutex
	outcomes := map[string]int{}
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := env.svc.Login("realuser", fmt.Sprintf("guess-%d", i), loginRequest(ip))
			mu.Lock()
			defer mu.Unlock()
			outcomes[fmt.Sprint(err)]++
		}(i)
	}
	wg.Wait()
	limit := env.svc.config.MaxLoginAttempts
	if got := outcomes[ErrInvalidCredentials.Error()]; got != limit {
		t.Errorf("%d of 40 parallel guesses had the password checked, want exactly the limit %d (outcomes %v)", got, limit, outcomes)
	}
	if got := int(env.svc.compares.Load()); got != limit {
		t.Errorf("%d password compares ran, want %d", got, limit)
	}
	if got := env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = 'realuser' AND NOT success`); got != limit {
		t.Errorf("%d failures recorded, want %d", got, limit)
	}
	if outcomes[ErrTooManyAttempts.Error()]+outcomes[ErrLoginBusy.Error()] != 40-limit {
		t.Errorf("outcomes %v: the rest should be throttled or turned away as busy", outcomes)
	}

	// Another address is not held up or locked out by any of that...
	if _, err := env.svc.Login("realuser", testPassword, loginRequest("203.0.113.51")); err != nil {
		t.Fatalf("right password from another address: %v", err)
	}
	// ...and right passwords sent together are all successes, none a failure.
	env.addUser("busy", models.RoleCaptain)
	var failed int
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := env.svc.Login("busy", testPassword, loginRequest("203.0.113.60")); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if failed != 0 || env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = 'busy' AND NOT success`) != 0 {
		t.Errorf("%d of 6 simultaneous right passwords failed or were recorded as failures", failed)
	}
}

// F1: password compares are bounded, and a sign-in that cannot get its turn
// gives up cleanly, without being recorded as a failed attempt.
func TestLoginGivesUpWhenCompareSlotsAreBusy(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("realuser", models.RoleCaptain)
	old := loginWait
	loginWait = 100 * time.Millisecond
	t.Cleanup(func() { loginWait = old })

	if cap(bcryptSlots) > 4 {
		t.Fatalf("%d bcrypt slots: the bound is meant to be small", cap(bcryptSlots))
	}
	for i := 0; i < cap(bcryptSlots); i++ {
		bcryptSlots <- struct{}{}
	}
	start := time.Now()
	_, err := env.svc.Login("realuser", testPassword, loginRequest("203.0.113.70"))
	for i := 0; i < cap(bcryptSlots); i++ {
		<-bcryptSlots
	}
	if !errors.Is(err, ErrLoginBusy) {
		t.Fatalf("sign-in with every slot busy: %v, want ErrLoginBusy", err)
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("gave up after %v, want about %v", waited, loginWait)
	}
	if n := env.count(`SELECT COUNT(*) FROM login_attempts`); n != 0 {
		t.Errorf("%d attempts recorded for a sign-in that was never checked", n)
	}
	// Free again: the same sign-in works.
	if _, err := env.svc.Login("realuser", testPassword, loginRequest("203.0.113.70")); err != nil {
		t.Fatalf("sign-in once slots are free: %v", err)
	}

	// Too many waiting from one address are turned away at once.
	locks := newAddressLocks()
	release, ok := locks.acquire(context.Background(), "a")
	if !ok {
		t.Fatal("first acquire failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 1; i < maxLoginWaitersPerAddress; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); locks.acquire(ctx, "a") }()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		locks.mu.Lock()
		refs := locks.byKey["a"].refs
		locks.mu.Unlock()
		if refs == maxLoginWaitersPerAddress || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, ok := locks.acquire(context.Background(), "a"); ok {
		t.Error("an acquire beyond the waiter limit was queued")
	}
	if r, ok := locks.acquire(context.Background(), "b"); !ok {
		t.Error("another address was held up")
	} else {
		r()
	}
	cancel()
	wg.Wait()
	release()
	if len(locks.byKey) != 0 {
		t.Errorf("%d locks left behind", len(locks.byKey))
	}
}

// F4: sign-in used to answer at once for an unknown, inactive or passwordless
// account and after a 200 ms compare for a real one. Every case now costs
// exactly one compare, against a dummy hash of the same cost.
func TestLoginAlwaysComparesAPassword(t *testing.T) {
	if cost, err := bcrypt.Cost([]byte(dummyPasswordHash)); err != nil || cost != BcryptCost {
		t.Fatalf("dummy hash cost %d (%v), want BcryptCost %d", cost, err, BcryptCost)
	}
	env := newPasskeyEnv(t)
	env.addUser("realuser", models.RoleCaptain)
	env.addUser("inactive", models.RoleCaptain)
	env.svc.db.MustExec(`UPDATE users SET is_active = FALSE WHERE username = 'inactive'`)
	env.svc.db.MustExec(`INSERT INTO users (username, password_hash, role) VALUES ('nopass', NULL, 'captain')`)
	for i, name := range []string{"realuser", "ghost", "nopass", "inactive"} {
		before := env.svc.compares.Load()
		_, err := env.svc.Login(name, "wrong-password-xyz", loginRequest(fmt.Sprintf("203.0.113.%d", 10+i)))
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("%s: %v", name, err)
		}
		if n := env.svc.compares.Load() - before; n != 1 {
			t.Errorf("%s: %d password compares, want 1", name, n)
		}
	}
	// The dummy hash matches nothing a person could type, even its own text.
	if _, err := env.svc.Login("ghost", dummyPasswordHash, loginRequest("203.0.113.20")); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("ghost: %v", err)
	}
}

// F3: what a failed sign-in stores is bounded, an over-long username costs no
// compare and is not echoed, and the sign-in form body is limited.
func TestSignInStoresBoundedClientData(t *testing.T) {
	env := newPasskeyEnv(t)
	env.addUser("realuser", models.RoleCaptain)
	post := func(form url.Values, ua string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("User-Agent", ua)
		r.RemoteAddr = "203.0.113.99:1"
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, r)
		return rec
	}
	longUA := strings.Repeat("U", 100000)

	// A 2 MB "username" is refused before anything is stored.
	if rec := post(url.Values{"username": {strings.Repeat("A", 2<<20)}, "password": {"x"}}, longUA); rec.Code != http.StatusBadRequest {
		t.Errorf("2 MB form: status %d, want 400", rec.Code)
	}
	// One that fits the body limit but is no account name: no compare, stored cut, not echoed.
	long := strings.Repeat("B", 4000)
	before := env.svc.compares.Load()
	rec := post(url.Values{"username": {long}, "password": {"x"}}, longUA)
	if env.svc.compares.Load() != before {
		t.Error("an over-long username cost a password compare")
	}
	if strings.Contains(rec.Body.String(), "BBBBBBBB") {
		t.Error("the over-long username is echoed in the page")
	}
	post(url.Values{"username": {"realuser"}, "password": {"wrong"}}, longUA)
	if n := env.count(`SELECT COUNT(*) FROM login_attempts`); n != 2 {
		t.Fatalf("%d attempts recorded, want 2", n)
	}
	if n := env.count(`SELECT MAX(LENGTH(username)) FROM login_attempts`); n > MaxUsernameBytes {
		t.Errorf("stored username is %d bytes, limit %d", n, MaxUsernameBytes)
	}
	if n := env.count(`SELECT MAX(LENGTH(user_agent)) FROM login_attempts`); n > MaxUserAgentBytes {
		t.Errorf("stored user agent is %d bytes, limit %d", n, MaxUserAgentBytes)
	}
	// A session stores a bounded user agent too.
	if rec := post(url.Values{"username": {"realuser"}, "password": {testPassword}}, longUA); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in: status %d", rec.Code)
	}
	if n := env.count(`SELECT MAX(MAX(LENGTH(user_agent)), MAX(LENGTH(device_info))) FROM sessions`); n > MaxUserAgentBytes {
		t.Errorf("stored session user agent is %d bytes, limit %d", n, MaxUserAgentBytes)
	}
	if got := clip("héllo", 2); got != "h" {
		t.Errorf("clip split a character: %q", got)
	}
}

// F2: a session alone can no longer change how an account signs in. This is
// the reviewer's takeover: with only the victim's session, add a passkey,
// use it to "confirm" a password change, remove the owner's methods.
func TestSessionAloneCannotChangeSignInMethods(t *testing.T) {
	env := newPasskeyEnv(t)
	env.keepQuarantine = true
	id := env.addUser("victim", models.RoleCaptain)
	owner := env.browser("198.51.100.1")
	owner.passwordSignIn("victim")
	ownerAuth := newAuthenticator(t)
	ownerCred := owner.enrol(ownerAuth, "owner's phone", true)
	env.pk.enrolments = newEnrolmentLog() // the owner's passkey has been there a while

	thief := env.browser("198.51.100.7")
	thief.passwordSignIn("victim") // stands for: holds a session of the victim, nothing else
	passkeys := func() int { return env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, id) }

	// Add a passkey: refused without proof, with a wrong password, and with
	// nothing stored.
	rec := thief.postJSON("/account/passkeys/begin", []byte("{}"))
	wantStatus(t, rec, http.StatusForbidden)
	if thief.cookies[ceremonyCookie] != "" {
		t.Error("a ceremony was begun without proof")
	}
	wantStatus(t, thief.postJSON("/account/passkeys/begin", []byte(`{"current_password":"not the password"}`)), http.StatusForbidden)
	// Remove the owner's passkey or password: refused.
	if rec := thief.postForm("/account/passkeys/1/delete", nil); !strings.Contains(rec.Header().Get("Location"), "error=reauth") {
		t.Errorf("remove passkey without proof: Location %q", rec.Header().Get("Location"))
	}
	if rec := thief.postForm("/account/passkeys/1/delete", url.Values{"current_password": {"guess"}}); !strings.Contains(rec.Header().Get("Location"), "error=wrong") {
		t.Errorf("remove passkey with a wrong password: Location %q", rec.Header().Get("Location"))
	}
	if passkeys() != 1 {
		t.Fatalf("%d passkeys, want the owner's one", passkeys())
	}
	// Wrong current passwords are limited per address across every form.
	for i := 0; i < currentPasswordFailureLimit; i++ {
		thief.postJSON("/account/passkeys/begin", []byte(`{"current_password":"guess again"}`))
	}
	wantStatus(t, thief.postJSON("/account/passkeys/begin", enrolBody), http.StatusTooManyRequests)

	// The owner, who knows the password, adds a second passkey. That ends
	// the account's other sessions (the thief's) and is audited with how
	// ownership was proven.
	newAuth := newAuthenticator(t)
	newCred := owner.enrol(newAuth, "new laptop", true)
	if passkeys() != 2 {
		t.Fatalf("%d passkeys after the owner added one", passkeys())
	}
	if rec := thief.do(http.MethodGet, SignInMethodsPath, "", nil); rec.Code != http.StatusSeeOther {
		t.Errorf("the other session still works after a passkey was added (status %d)", rec.Code)
	}
	if rec := owner.do(http.MethodGet, SignInMethodsPath, "", nil); rec.Code != http.StatusOK {
		t.Errorf("the browser that added the passkey was signed out (status %d)", rec.Code)
	}
	last := env.events[len(env.events)-1]
	if !strings.Contains(last, PasskeyActionAdd) || !strings.Contains(last, "reauth=password") || !strings.Contains(last, "sessions_ended=1") {
		t.Errorf("audit event for the added passkey: %q", last)
	}

	// The passkey added a moment ago cannot confirm anything: not in the
	// session that added it, and not from another session either.
	for _, b := range []*browser{owner, env.browser("198.51.100.9")} {
		if !b.signedIn() {
			b.passwordSignIn("victim")
		}
		begin := b.postJSON(ReauthBeginPath, []byte("{}"))
		wantStatus(t, begin, http.StatusOK)
		finish := b.postJSON(ReauthFinishPath, newAuth.get(begin.Body.Bytes(), newCred))
		wantStatus(t, finish, http.StatusForbidden)
		if b.cookies[reauthCookie] != "" {
			t.Error("a proof was minted from a passkey added a moment ago")
		}
	}
	// Once the quarantine has passed it can, except in the session that added it.
	env.advance(NewPasskeyQuarantine + time.Second)
	begin := owner.postJSON(ReauthBeginPath, []byte("{}"))
	wantStatus(t, owner.postJSON(ReauthFinishPath, newAuth.get(begin.Body.Bytes(), newCred)), http.StatusForbidden)
	other := env.browser("198.51.100.10")
	other.passwordSignIn("victim")
	reauth(t, other, newAuth, newCred)
	// The older passkey always can, and a confirmation is good for one change.
	owner.passwordSignIn("victim")
	reauth(t, owner, ownerAuth, ownerCred)
	if rec := owner.postForm("/account/passkeys/2/delete", nil); !strings.Contains(rec.Header().Get("Location"), "done=removed") {
		t.Fatalf("remove with a passkey confirmation: Location %q", rec.Header().Get("Location"))
	}
	if rec := owner.postForm("/account/passkeys/1/delete", nil); !strings.Contains(rec.Header().Get("Location"), "error=reauth") {
		t.Errorf("the confirmation worked twice: Location %q", rec.Header().Get("Location"))
	}
	last = env.events[len(env.events)-1]
	if !strings.Contains(last, PasskeyActionRemove) || !strings.Contains(last, "reauth=passkey") || !strings.Contains(last, "sessions_ended=") {
		t.Errorf("audit event for the removed passkey: %q", last)
	}
	if rec := other.do(http.MethodGet, SignInMethodsPath, "", nil); rec.Code != http.StatusSeeOther {
		t.Errorf("the other session still works after a passkey was removed (status %d)", rec.Code)
	}
}

// F2: no legitimate path dead-ends. A passkey-only account adds a second
// passkey by confirming with the first, even when the first is new, because
// it is the account's only way in; and a cancelled browser prompt does not
// use the confirmation up.
func TestPasskeyOnlyAccountCanStillChangeSignInMethods(t *testing.T) {
	env := newPasskeyEnv(t)
	env.keepQuarantine = true
	id := env.addUser("solo", models.RoleCaptain)
	b := env.browser("198.51.100.1")
	b.passwordSignIn("solo")
	first := newAuthenticator(t)
	cred := b.enrol(first, "first", true)
	// Passkey-only from here on; the passkey was added in this very session.
	env.svc.db.MustExec(`UPDATE users SET password_hash = NULL WHERE id = ?`, id)

	reauth(t, b, first, cred) // allowed: it is the only usable sign-in method
	// The person cancels the browser's prompt: begin, no finish. Then tries again.
	wantStatus(t, b.postJSON("/account/passkeys/begin", []byte("{}")), http.StatusOK)
	begin := b.postJSON("/account/passkeys/begin", []byte("{}"))
	wantStatus(t, begin, http.StatusOK)
	second := newAuthenticator(t)
	body, _ := second.create(begin.Body.Bytes(), true)
	wantStatus(t, b.postJSON("/account/passkeys/finish?name=second", body), http.StatusOK)
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, id); n != 2 {
		t.Fatalf("%d passkeys, want 2", n)
	}
	// The confirmation was used up by that; a second enrolment needs another.
	wantStatus(t, b.postJSON("/account/passkeys/begin", []byte("{}")), http.StatusForbidden)

	// The page shows what to do in each state.
	page := b.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	for _, want := range []string{"data-passkey-reauth", "data-signout-elsewhere", "data-passkey-remove-open"} {
		if !strings.Contains(page, want) {
			t.Errorf("passkey-only page lacks %s", want)
		}
	}
	if strings.Contains(page, `id="passkey-add"`) || strings.Contains(page, "data-passkey-remove>") {
		t.Error("passkey-only page offers to add or remove before confirming")
	}
}

// F6: a passkey blocked by a clone warning is not a way to sign in, so the
// only working passkey cannot be removed because a blocked one remains; the
// blocked one can still be removed.
func TestBlockedPasskeyDoesNotCountAsASignInMethod(t *testing.T) {
	env := newPasskeyEnv(t)
	id := env.addUser("cap", models.RoleCaptain)
	b := env.browser("198.51.100.8")
	b.passwordSignIn("cap")
	a1, a2 := newAuthenticator(t), newAuthenticator(t)
	b.enrol(a1, "one", true)
	cred2 := b.enrol(a2, "two", true)
	env.svc.db.MustExec(`UPDATE users SET password_hash = NULL WHERE id = ?`, id)
	env.svc.db.MustExec(`UPDATE webauthn_credentials SET clone_warning_at = CURRENT_TIMESTAMP WHERE id = 1`)
	usable := func() int {
		return env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ? AND clone_warning_at IS NULL`, id)
	}
	if n, _ := env.svc.CountSignInMethods(context.Background(), id); n != 1 {
		t.Fatalf("sign-in methods = %d, want 1 (the blocked passkey does not count)", n)
	}

	reauth(t, b, a2, cred2)
	rec := b.postForm("/account/passkeys/2/delete", nil)
	if !strings.Contains(rec.Header().Get("Location"), "error=last") || usable() != 1 {
		t.Fatalf("deleting the only usable passkey: Location %q, usable passkeys left %d", rec.Header().Get("Location"), usable())
	}
	page := b.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	if strings.Count(page, "data-passkey-remove-open") != 1 || strings.Count(page, "data-passkey-last-method") != 1 {
		t.Error("the page should offer to remove the blocked passkey only")
	}
	reauth(t, b, a2, cred2)
	rec = b.postForm("/account/passkeys/1/delete", nil)
	if !strings.Contains(rec.Header().Get("Location"), "done=removed") {
		t.Fatalf("deleting the blocked passkey: Location %q", rec.Header().Get("Location"))
	}
	if usable() != 1 || env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, id) != 1 {
		t.Fatal("the usable passkey should be the one left")
	}
}

// F7: an administrator's passkey must verify the person (PIN, fingerprint,
// face, screen lock); other accounts keep "preferred".
func TestAdministratorsNeedUserVerification(t *testing.T) {
	env := newPasskeyEnv(t)
	adminID := env.addUser("root", models.RoleAdmin)
	env.addUser("cap", models.RoleCaptain)

	// A captain can use a security key with no PIN, start to finish.
	cap := env.browser("198.51.100.1")
	cap.passwordSignIn("cap")
	begin := cap.postJSON("/account/passkeys/begin", enrolBody)
	var opts creationOptions
	if err := json.Unmarshal(begin.Body.Bytes(), &opts); err != nil || opts.PublicKey.AuthenticatorSelection.UserVerification != "preferred" {
		t.Fatalf("captain's options ask for user verification %q (%v), want preferred", opts.PublicKey.AuthenticatorSelection.UserVerification, err)
	}
	plainKey := newAuthenticator(t)
	plainKey.noUV = true
	body, capCred := plainKey.create(begin.Body.Bytes(), true)
	wantStatus(t, cap.postJSON("/account/passkeys/finish", body), http.StatusOK)
	wantStatus(t, env.browser("198.51.100.2").passkeySignIn(plainKey, capCred), http.StatusOK)

	// An administrator cannot enrol one...
	root := env.browser("198.51.100.3")
	root.passwordSignIn("root")
	begin = root.postJSON("/account/passkeys/begin", enrolBody)
	rogue := newAuthenticator(t)
	rogue.noUV = true
	body, _ = rogue.create(begin.Body.Bytes(), true)
	rec := root.postJSON("/account/passkeys/finish", body)
	wantStatus(t, rec, http.StatusForbidden)
	if !strings.Contains(rec.Body.String(), "PIN") {
		t.Errorf("enrolment refusal does not say what to do: %s", rec.Body.String())
	}
	if n := env.count(`SELECT COUNT(*) FROM webauthn_credentials WHERE user_id = ?`, adminID); n != 0 {
		t.Fatal("an unverified passkey was stored for an administrator")
	}
	// ...and one that stops verifying (or an account promoted with such a
	// passkey) cannot sign in or confirm changes with it.
	key := newAuthenticator(t)
	cred := root.enrol(key, "key", true)
	key.noUV = true
	fresh := env.browser("198.51.100.4")
	rec = fresh.passkeySignIn(key, cred)
	wantStatus(t, rec, http.StatusForbidden)
	if fresh.signedIn() || !strings.Contains(rec.Body.String(), "PIN") {
		t.Errorf("sign-in without user verification: signed in %v, body %s", fresh.signedIn(), rec.Body.String())
	}
	if n := env.count(`SELECT COUNT(*) FROM login_attempts WHERE username = ?`, passkeyAttemptUsername); n != 0 {
		t.Errorf("the refusal was counted as %d failed passkey attempts", n)
	}
	begin = root.postJSON(ReauthBeginPath, []byte("{}"))
	var ropts requestOptions
	if err := json.Unmarshal(begin.Body.Bytes(), &ropts); err != nil || ropts.PublicKey.UserVerification != "required" {
		t.Errorf("administrator's confirmation asks for user verification %q (%v), want required", ropts.PublicKey.UserVerification, err)
	}
	rec = root.postJSON(ReauthFinishPath, key.get(begin.Body.Bytes(), cred))
	wantStatus(t, rec, http.StatusForbidden)
	if root.cookies[reauthCookie] != "" || !strings.Contains(rec.Body.String(), "PIN") {
		t.Errorf("a proof was minted without user verification, or the message is unclear: %s", rec.Body.String())
	}
	// With verification both work.
	key.noUV = false
	wantStatus(t, fresh.passkeySignIn(key, cred), http.StatusOK)
	reauth(t, root, key, cred)
}

// F10: "Sign out everywhere else" ends the account's other sessions and is
// audited; a passkey sign-in ends the session the browser already carried.
func TestSignOutEverywhereElseAndSessionRotation(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, id := env.enrolledUser("alice", true)
	here, there := env.browser("198.51.100.1"), env.browser("198.51.100.2")
	here.passwordSignIn("alice")
	there.passwordSignIn("alice")
	valid := func() int { return env.count(`SELECT COUNT(*) FROM sessions WHERE user_id = ? AND is_valid`, id) }
	before := valid()
	if before < 2 {
		t.Fatalf("%d valid sessions, want at least 2", before)
	}
	page := here.do(http.MethodGet, SignInMethodsPath, "", nil).Body.String()
	if !strings.Contains(page, fmt.Sprintf(`data-other-sessions="%d"`, before-1)) || !strings.Contains(page, `action="`+SignOutElsewherePath+`"`) {
		t.Error("page does not show the other sessions and the control")
	}
	rec := here.postForm(SignOutElsewherePath, nil)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "done=signed-out-elsewhere") {
		t.Fatalf("sign out everywhere else: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if valid() != 1 {
		t.Errorf("%d valid sessions left, want this one only", valid())
	}
	if here.do(http.MethodGet, SignInMethodsPath, "", nil).Code != http.StatusOK || there.do(http.MethodGet, SignInMethodsPath, "", nil).Code != http.StatusSeeOther {
		t.Error("this browser should stay signed in and the other be signed out")
	}
	if last := env.events[len(env.events)-1]; !strings.Contains(last, SessionsActionEndOthers) || !strings.Contains(last, fmt.Sprintf("sessions_ended=%d", before-1)) {
		t.Errorf("audit event: %q", last)
	}
	// It needs a session.
	if rec := env.browser("198.51.100.3").postForm(SignOutElsewherePath, nil); !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Errorf("without a session: Location %q", rec.Header().Get("Location"))
	}

	// A passkey sign-in in a browser that already carries a session ends
	// that session.
	old := here.cookies["session_token"]
	wantStatus(t, here.passkeySignIn(a, cred), http.StatusOK)
	if here.cookies["session_token"] == old {
		t.Fatal("no new session")
	}
	if n := env.count(`SELECT COUNT(*) FROM sessions WHERE token_hash = ? AND is_valid`, HashSessionToken(old)); n != 0 {
		t.Error("the session the browser carried is still valid after a passkey sign-in")
	}
}

// I1: the app itself forbids framing, on every response.
func TestNoFramingHeaders(t *testing.T) {
	h := NoFraming(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	for _, path := range []string{"/", "/login", "/admin/league/users", "/static/passkey.js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options %q, want DENY", path, got)
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
			t.Errorf("%s: Content-Security-Policy %q", path, got)
		}
	}
}

// I2: passkey confirmation is throttled under the same key as everything
// else (an IPv6 client counts as its /64), not the bare address.
func TestReauthThrottlesByThrottleKey(t *testing.T) {
	env := newPasskeyEnv(t)
	a, cred, _ := env.enrolledUser("alice", true)
	_ = a
	_ = cred
	b := env.browser("2001:db8:1:2:aaaa::1")
	b.ip = "[2001:db8:1:2:aaaa::1]"
	b.passwordSignIn("alice")
	wantStatus(t, b.postJSON(ReauthBeginPath, []byte("{}")), http.StatusOK)
	env.pk.begins.mu.Lock()
	defer env.pk.begins.mu.Unlock()
	if _, ok := env.pk.begins.events["2001:db8:1:2::/64"]; !ok {
		keys := []string{}
		for k := range env.pk.begins.events {
			keys = append(keys, k)
		}
		t.Errorf("confirmation counted under %v, want the /64 2001:db8:1:2::/64", keys)
	}
}
