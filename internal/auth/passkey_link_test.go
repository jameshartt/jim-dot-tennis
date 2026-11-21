// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"net/http"
	"testing"

	"jim-dot-tennis/internal/models"
)

// mountLinkCeremony adds two endpoints that drive BeginLinkRegistration and
// VerifyLinkRegistration the way internal/accounts' link page does: for the
// account ?user=, bound to ?binding= (which the real page derives from the
// link's token). It records what a successful verify returned.
func mountLinkCeremony(env *passkeyEnv, verified *[]*models.WebAuthnCredential) {
	mux := http.NewServeMux()
	templateDir := ""
	NewPasskeyHandler(env.pk, templateDir, "/admin/league").RegisterRoutes(mux, NewMiddleware(env.svc, nil, nil))
	login := NewHandler(env.svc, templateDir, "/admin/league")
	login.RegisterRoutes(mux)
	user := func(r *http.Request) *models.User {
		var u models.User
		if err := env.svc.db.Get(&u, `SELECT * FROM users WHERE username = ?`, r.URL.Query().Get("user")); err != nil {
			env.t.Fatalf("test user: %v", err)
		}
		return &u
	}
	mux.HandleFunc("POST /link/begin", func(w http.ResponseWriter, r *http.Request) {
		options, err := env.pk.BeginLinkRegistration(w, r, user(r), r.URL.Query().Get("binding"))
		if err != nil {
			WritePasskeyFailure(w, err)
			return
		}
		WritePasskeyJSON(w, http.StatusOK, options)
	})
	mux.HandleFunc("POST /link/finish", func(w http.ResponseWriter, r *http.Request) {
		cred, err := env.pk.VerifyLinkRegistration(w, r, user(r), r.URL.Query().Get("binding"), "Link passkey")
		if err != nil {
			WritePasskeyFailure(w, err)
			return
		}
		*verified = append(*verified, cred)
		WritePasskeyJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	})
	env.handler = CrossOriginGuard(mux)
}

// A passkey ceremony begun from an invite or recovery link can only be
// finished by the same browser, presenting the same link, for the same
// account; verifying stores nothing (the caller stores the passkey in the
// transaction that consumes the link).
func TestLinkPasskeyCeremonyIsBoundToLinkBrowserAndAccount(t *testing.T) {
	env := newPasskeyEnv(t)
	var verified []*models.WebAuthnCredential
	mountLinkCeremony(env, &verified)
	env.addUser("pat", models.RoleCaptain)
	env.addUser("sam", models.RoleCaptain)
	stored := func() int { return env.count(`SELECT COUNT(*) FROM webauthn_credentials`) }
	const linkA, linkB = "link:aaaaaaaa", "link:bbbbbbbb"

	begin := func(b *browser, user, binding string) []byte {
		t.Helper()
		rec := b.postJSON("/link/begin?user="+user+"&binding="+binding, []byte("{}"))
		wantStatus(t, rec, http.StatusOK)
		return rec.Body.Bytes()
	}
	finish := func(b *browser, user, binding string, body []byte) int {
		t.Helper()
		return b.postJSON("/link/finish?user="+user+"&binding="+binding, body).Code
	}

	// The whole ceremony, no session anywhere: verified, and nothing stored.
	holder := env.browser("198.51.100.1")
	a := newAuthenticator(t)
	body, _ := a.create(begin(holder, "pat", linkA), false)
	if code := finish(holder, "pat", linkA, body); code != http.StatusOK {
		t.Fatalf("finish: status %d", code)
	}
	if len(verified) != 1 || verified[0].ID != 0 || verified[0].Name != "Link passkey" || len(verified[0].CredentialID) == 0 {
		t.Fatalf("verified passkey: %+v", verified)
	}
	var pat models.User
	if err := env.svc.db.Get(&pat, `SELECT * FROM users WHERE username = 'pat'`); err != nil {
		t.Fatal(err)
	}
	if verified[0].UserID != pat.ID {
		t.Fatalf("verified for user %d, want %d", verified[0].UserID, pat.ID)
	}
	if stored() != 0 {
		t.Fatal("verifying a link ceremony stored a passkey")
	}
	if holder.signedIn() {
		t.Fatal("the ceremony created a session")
	}
	// The challenge is spent: the same response cannot be replayed.
	if code := finish(holder, "pat", linkA, body); code != http.StatusBadRequest {
		t.Fatalf("replayed finish: status %d, want 400", code)
	}

	// Another link (even for the same account) cannot finish the ceremony,
	// and the attempt spends it.
	body, _ = a.create(begin(holder, "pat", linkA), false)
	if code := finish(holder, "pat", linkB, body); code != http.StatusBadRequest {
		t.Fatalf("finish with another link: status %d, want 400", code)
	}
	if code := finish(holder, "pat", linkA, body); code != http.StatusBadRequest {
		t.Fatalf("finish after a spent ceremony: status %d, want 400", code)
	}

	// Another browser holding the same link cannot finish a ceremony it did
	// not begin (it has no ceremony cookie), so a response captured from the
	// holder's browser is no use to anyone else.
	body, _ = a.create(begin(holder, "pat", linkA), false)
	other := env.browser("203.0.113.5")
	if code := finish(other, "pat", linkA, body); code != http.StatusBadRequest {
		t.Fatalf("finish from another browser: status %d, want 400", code)
	}
	// The holder's own ceremony is still there for the holder.
	if code := finish(holder, "pat", linkA, body); code != http.StatusOK {
		t.Fatalf("the holder's finish after someone else's failed attempt: status %d", code)
	}

	// The ceremony is for one account: it cannot enrol a passkey for another.
	body, _ = a.create(begin(holder, "pat", linkA), false)
	if code := finish(holder, "sam", linkA, body); code != http.StatusBadRequest {
		t.Fatalf("finish for another account: status %d, want 400", code)
	}

	// A link ceremony cannot be finished through the signed-in endpoint, and
	// a signed-in ceremony cannot be finished through a link.
	signedIn := env.browser("198.51.100.9")
	signedIn.passwordSignIn("pat")
	body, _ = a.create(begin(signedIn, "pat", linkA), false)
	if code := signedIn.postJSON("/account/passkeys/finish", body).Code; code != http.StatusBadRequest {
		t.Fatalf("link ceremony finished at the signed-in endpoint: status %d, want 400", code)
	}
	rec := signedIn.postJSON("/account/passkeys/begin", enrolBody)
	wantStatus(t, rec, http.StatusOK)
	body, _ = a.create(rec.Body.Bytes(), false)
	if code := finish(signedIn, "pat", linkA, body); code != http.StatusBadRequest {
		t.Fatalf("signed-in ceremony finished through a link: status %d, want 400", code)
	}

	// No binding, no ceremony.
	if code := holder.postJSON("/link/begin?user=pat&binding=", []byte("{}")).Code; code != http.StatusBadRequest {
		t.Fatalf("begin with an empty binding: status %d, want 400", code)
	}
	if stored() != 0 {
		t.Fatalf("%d passkeys stored by link ceremonies, want none", stored())
	}
	if len(verified) != 2 {
		t.Fatalf("%d ceremonies verified, want 2", len(verified))
	}
}
