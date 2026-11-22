// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"jim-dot-tennis/internal/models"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Re-authentication with a passkey.
//
// Changing how an account signs in (adding or removing a passkey; adding,
// changing or removing the password) needs proof that the person at the
// keyboard is the account's owner, not just someone holding its session
// (ownership.go). The proof is either the current password or a fresh
// passkey assertion, made here:
//
//  1. POST ReauthBeginPath (signed in, JSON) starts an assertion limited to
//     the signed-in user's own passkeys. The ceremony is tied to this
//     browser (ceremony cookie), the user and the session that began it.
//  2. POST ReauthFinishPath verifies it like a sign-in (signature, origin,
//     relying party, counter, clone warning) and that the passkey is one of
//     the user's. On success it mints a proof: 256 random bits, kept in
//     memory by hash, bound to the user and the session's fingerprint, valid
//     for ReauthProofLifetime, and given to the browser as an HttpOnly,
//     SameSite=Strict cookie limited to /account/.
//  3. The change consumes the proof (ConsumeReauthProof): it is removed on
//     that first use whatever the outcome, and only counts for the same user
//     and the same session.
//
// A passkey that was itself added from a session a moment ago, or in this
// session, mints nothing (ErrPasskeyTooNew) unless it is the account's only
// usable sign-in method. An administrator's assertion must carry the
// user-verified flag (ErrUserVerificationRequired).
//
// A sign-in, however recent, is never proof by itself: only a ceremony run
// for this purpose mints one.

// Re-authentication routes (signed in, JSON).
const (
	ReauthBeginPath  = "/account/reauth/passkey/begin"
	ReauthFinishPath = "/account/reauth/passkey/finish"
)

// ReauthBackupRedirect is where a confirmation made for a backup download
// returns to (admin.backupsPath).
const ReauthBackupRedirect = "/admin/league/backups?confirmed=1"

// ReauthProofLifetime is how long a passkey re-authentication counts for.
const ReauthProofLifetime = 5 * time.Minute

// reauthCookie carries the proof; reauthCookiePath limits it to the account pages.
const (
	reauthCookie     = "reauth_proof"
	reauthCookiePath = "/account/"
)

// ceremonyReauth is a re-authentication assertion. It is kept apart from
// ceremonyLogin so a sign-in ticket can never mint a proof, nor the reverse.
const ceremonyReauth ceremonyKind = 10

// maxReauthProofs bounds the in-memory proof store.
const maxReauthProofs = 5000

// ErrNoPasskeys: the account has no passkey to re-authenticate with.
var ErrNoPasskeys = errors.New("this account has no passkey")

type reauthProof struct {
	userID  int64
	session string
	expires time.Time
}

// reauthStore holds minted proofs by the hash of their token.
type reauthStore struct {
	mu     sync.Mutex
	byHash map[[sha256.Size]byte]reauthProof
}

func newReauthStore() *reauthStore {
	return &reauthStore{byHash: map[[sha256.Size]byte]reauthProof{}}
}

func (s *reauthStore) put(p reauthProof, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.byHash) >= maxReauthProofs {
		for k, v := range s.byHash {
			if !v.expires.After(now) {
				delete(s.byHash, k)
			}
		}
		if len(s.byHash) >= maxReauthProofs {
			for k := range s.byHash { // still full: drop one
				delete(s.byHash, k)
				break
			}
		}
	}
	s.byHash[sha256.Sum256([]byte(token))] = p
	return token, nil
}

// get returns the proof for token. take also removes it.
func (s *reauthStore) get(token string, take bool) (reauthProof, bool) {
	if token == "" {
		return reauthProof{}, false
	}
	key := sha256.Sum256([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byHash[key]
	if take {
		delete(s.byHash, key)
	}
	return p, ok
}

// valid reports whether p proves userID with the session whose fingerprint is fp at now.
func (p reauthProof) valid(userID int64, fp string, now time.Time) bool {
	return p.userID == userID && fp != "" && hmac.Equal([]byte(p.session), []byte(fp)) && p.expires.After(now)
}

// BeginReauth starts a passkey assertion for the signed-in user, limited to
// their own passkeys, and returns the options for navigator.credentials.get()
// and the ceremony ticket.
func (p *Passkeys) BeginReauth(r *http.Request, user *models.User, sessionFP string) (*protocol.CredentialAssertion, string, error) {
	if sessionFP == "" {
		return nil, "", ErrPasskeyCeremony
	}
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return nil, "", err
	}
	now := p.now()
	ip := ThrottleKey(r)
	if p.failures.blocked(ip, now) || !p.begins.hit(ip, now) {
		return nil, "", ErrPasskeyThrottled
	}
	wu, _, err := p.loadUser(r.Context(), user)
	if err != nil {
		return nil, "", err
	}
	if len(wu.creds) == 0 {
		return nil, "", ErrNoPasskeys
	}
	opts := []webauthn.LoginOption{webauthn.WithLoginOrigin(origin)}
	if requireUserVerification(user) {
		// The library then refuses an assertion without the user-verified flag.
		opts = append(opts, webauthn.WithUserVerification(protocol.VerificationRequired))
	}
	options, data, err := wa.BeginLogin(wu, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("begin passkey re-authentication: %w", err)
	}
	ticket, err := p.ceremonies.put(ceremony{
		kind: ceremonyReauth, data: *data, origin: origin, userID: user.ID, session: sessionFP,
	}, now)
	if err != nil {
		return nil, "", err
	}
	return options, ticket, nil
}

// FinishReauth verifies the browser's response to BeginReauth and returns a
// new proof token. The ceremony is consumed whatever the outcome; failures
// count towards the client address's passkey throttle.
func (p *Passkeys) FinishReauth(r *http.Request, user *models.User, sessionFP, ticket string, body io.Reader) (string, error) {
	now := p.now()
	ip := ThrottleKey(r)
	c, ok := p.ceremonies.take(ticket, ceremonyReauth, now)
	if p.failures.blocked(ip, now) {
		return "", ErrPasskeyThrottled
	}
	if !ok || c.userID != user.ID || sessionFP == "" || !hmac.Equal([]byte(c.session), []byte(sessionFP)) {
		return "", ErrPasskeyCeremony
	}
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return "", err
	}
	if c.origin != origin || c.data.RelyingPartyID != wa.Config.RPID {
		return "", ErrPasskeyCeremony
	}
	reject := func(what string, err error) (string, error) {
		logPasskeyError(what, err)
		p.failures.record(ip, now)
		return "", ErrPasskeyRejected
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(io.LimitReader(body, maxPasskeyBody))
	if err != nil {
		return reject("re-authentication response", err)
	}
	wu, stored, err := p.loadUser(r.Context(), user)
	if err != nil {
		return "", err
	}
	// ValidateLogin checks the passkey is one of wu's (and the user handle,
	// when the authenticator sends one, is wu's).
	cred, err := wa.ValidateLogin(wu, c.data, parsed)
	if err != nil {
		if requireUserVerification(user) && !parsed.Response.AuthenticatorData.Flags.UserVerified() {
			logPasskeyError("re-authentication", err)
			return "", ErrUserVerificationRequired
		}
		return reject("re-authentication", err)
	}
	if requireUserVerification(user) && !parsed.Response.AuthenticatorData.Flags.UserVerified() {
		return "", ErrUserVerificationRequired
	}
	var used *models.WebAuthnCredential
	for i := range stored {
		if hmac.Equal(stored[i].CredentialID, cred.ID) {
			used = &stored[i]
		}
	}
	if used == nil {
		return reject("re-authentication", errors.New("verified credential not found in store"))
	}
	if cred.Authenticator.CloneWarning || used.CloneWarningAt != nil {
		if used.CloneWarningAt == nil {
			if _, err := p.creds.SetCloneWarning(r.Context(), used.ID, &now); err != nil {
				log.Printf("Failed to store passkey clone warning for passkey ID %d: %v", used.ID, err)
			}
			log.Printf("SECURITY WARNING: passkey ID %d of user %q (ID %d) reported a sign counter that did not increase during re-authentication. "+
				"It is blocked until an administrator clears the warning.", used.ID, user.Username, user.ID)
			p.event(r.Context(), user, PasskeyActionCloneWarning, fmt.Sprintf("passkey_id=%d", used.ID))
		}
		p.failures.record(ip, now)
		return "", ErrPasskeyBlocked
	}
	used.SignCount = cred.Authenticator.SignCount
	used.UserPresent = cred.Flags.UserPresent
	used.UserVerified = cred.Flags.UserVerified
	used.BackupEligible = cred.Flags.BackupEligible
	used.BackupState = cred.Flags.BackupState
	if err := p.creds.RecordUse(r.Context(), used, now); err != nil {
		return "", fmt.Errorf("record passkey use: %w", err)
	}
	// A passkey added from a session a moment ago (or in this one) proves
	// only that the session could add it. The one exception is a passkey
	// that is the account's only usable way in: there is nothing older to
	// ask for, and removing the others needed proof too.
	if p.enrolments.quarantined(used.ID, sessionFP, now) {
		methods, err := p.service.CountSignInMethods(r.Context(), user.ID)
		if err != nil {
			return "", err
		}
		if methods > 1 {
			return "", ErrPasskeyTooNew
		}
	}
	return p.reauth.put(reauthProof{userID: user.ID, session: sessionFP, expires: now.Add(ReauthProofLifetime)}, now)
}

func (p *Passkeys) setReauthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     reauthCookie,
		Value:    token,
		Path:     reauthCookiePath,
		MaxAge:   int(ReauthProofLifetime / time.Second),
		HttpOnly: true,
		Secure:   p.service.config.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (p *Passkeys) clearReauthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: reauthCookie, Value: "", Path: reauthCookiePath, MaxAge: -1,
		HttpOnly: true, Secure: p.service.config.CookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func reauthToken(r *http.Request) string {
	c, err := r.Cookie(reauthCookie)
	if err != nil {
		return ""
	}
	return c.Value
}

// HasReauthProof reports, without using it up, whether the request carries a
// live passkey re-authentication of userID made in this session. It is for
// showing the page; only ConsumeReauthProof authorises a change.
func (p *Passkeys) HasReauthProof(r *http.Request, userID int64) bool {
	if p == nil {
		return false
	}
	proof, ok := p.reauth.get(reauthToken(r), false)
	return ok && proof.valid(userID, p.service.sessionFingerprint(r), p.now())
}

// ConsumeReauthProof uses up the request's passkey re-authentication and
// reports whether it proved userID in this session and had not expired. The
// proof is removed (and its cookie cleared) whatever the answer, so it works
// at most once.
func (p *Passkeys) ConsumeReauthProof(w http.ResponseWriter, r *http.Request, userID int64) bool {
	if p == nil {
		return false
	}
	token := reauthToken(r)
	if token == "" {
		return false
	}
	p.clearReauthCookie(w)
	proof, ok := p.reauth.get(token, true)
	return ok && proof.valid(userID, p.service.sessionFingerprint(r), p.now())
}

// reauthBegin starts a passkey re-authentication for the signed-in user.
func (h *PasskeyHandler) reauthBegin(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok || !requireJSON(w, r) {
		return
	}
	options, ticket, err := h.passkeys.BeginReauth(r, user, h.service.sessionFingerprint(r))
	if err != nil {
		if errors.Is(err, ErrNoPasskeys) {
			writePasskeyJSON(w, http.StatusConflict, passkeyError{Error: "You have no passkey to confirm with. Use your current password instead."})
			return
		}
		writePasskeyFailure(w, err)
		return
	}
	h.setCeremonyCookie(w, ticket)
	writePasskeyJSON(w, http.StatusOK, options)
}

// reauthFinish verifies the assertion and gives the browser the proof.
func (h *PasskeyHandler) reauthFinish(w http.ResponseWriter, r *http.Request) {
	user, ok := contextUser(w, r)
	if !ok || !requireJSON(w, r) {
		return
	}
	token, err := h.passkeys.FinishReauth(r, user, h.service.sessionFingerprint(r), h.takeCeremonyCookie(w, r),
		http.MaxBytesReader(w, r.Body, maxPasskeyBody))
	if err != nil {
		writePasskeyFailure(w, err)
		return
	}
	h.passkeys.setReauthCookie(w, token)
	// Back to the page (and the part of it) the person was in.
	redirect := SignInMethodsPath + "?done=verified#passkeys"
	switch r.URL.Query().Get("for") {
	case "password":
		redirect = SignInMethodsPath + "?done=verified#password"
	case "backup":
		// The backups page (administrators); the download it then posts to
		// is under /account/, where this cookie is sent.
		redirect = ReauthBackupRedirect
	}
	writePasskeyJSON(w, http.StatusOK, map[string]string{"redirect": redirect})
}

// registerReauthRoutes adds the re-authentication endpoints (RegisterRoutes calls it).
func (h *PasskeyHandler) registerReauthRoutes(mux *http.ServeMux, signedIn func(http.HandlerFunc) http.Handler) {
	mux.Handle("POST "+ReauthBeginPath, signedIn(h.reauthBegin))
	mux.Handle("POST "+ReauthFinishPath, signedIn(h.reauthFinish))
}
