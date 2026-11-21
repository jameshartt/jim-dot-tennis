// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// Passkey limits.
const (
	// MaxPasskeysPerUser bounds how many passkeys one account can hold.
	MaxPasskeysPerUser = 20
	// MaxPasskeyNameLength is the longest name (in characters) a passkey can be given.
	MaxPasskeyNameLength = 60
	// maxPasskeyBody bounds the JSON a browser posts to finish a ceremony.
	maxPasskeyBody = 64 << 10
	// passkeyAttemptUsername is the name failed passkey sign-ins are recorded
	// under in login_attempts. A failed passkey attempt is never recorded
	// against a real username, so it can never count towards that user's
	// password throttle (the per-address password count leaves it out too).
	passkeyAttemptUsername = repository.PasskeyAttemptUsername
)

// Passkey throttling, per client address only (see windowLimiter).
const (
	passkeyBeginLimit    = 30 // ceremonies begun per address...
	passkeyBeginWindow   = time.Minute
	passkeyFailureLimit  = 10 // failed sign-in assertions per address...
	passkeyFailureWindow = 15 * time.Minute
)

var (
	// ErrPasskeysUnavailable means there is no trusted public address to tie
	// passkeys to (see Passkeys.relyingParty).
	ErrPasskeysUnavailable = errors.New("passkeys are not available on this address")
	// ErrPasskeyThrottled means this client address has made too many attempts.
	ErrPasskeyThrottled = errors.New("too many passkey attempts")
	// ErrPasskeyCeremony means there is no live ceremony for this browser: it
	// was never begun here, has expired, or has already been used.
	ErrPasskeyCeremony = errors.New("passkey request expired or already used")
	// ErrPasskeyRejected means the passkey response did not verify. It is
	// deliberately the only error for every verification failure.
	ErrPasskeyRejected = errors.New("passkey was not accepted")
	// ErrPasskeyBlocked means the passkey verified but carries a clone warning.
	ErrPasskeyBlocked = errors.New("passkey is blocked pending review")
	// ErrPasskeyLimit means the account already holds MaxPasskeysPerUser passkeys.
	ErrPasskeyLimit = errors.New("too many passkeys on this account")
)

// PublicURLSource provides the instance's configured public address
// (*config.ClubSettings). It is read on every ceremony, never cached, because
// an administrator can change it on the settings page while the app runs.
type PublicURLSource interface {
	ResolvePublicURL() (config.PublicURL, bool, error)
}

// Passkeys runs WebAuthn registration and sign-in ceremonies.
type Passkeys struct {
	service     *Service
	creds       repository.WebAuthnCredentialRepository
	settings    repository.SettingsRepository
	publicURL   PublicURLSource
	displayName func() string

	ceremonies *ceremonyStore
	begins     *windowLimiter
	failures   *windowLimiter
	now        func() time.Time

	decoyMu  sync.Mutex
	decoyKey []byte

	audit PasskeyAudit
}

// PasskeyAudit records a change to an account's passkeys: who did it, what
// (one of the PasskeyAction constants) and a short detail. It is the seam
// for the account audit log.
type PasskeyAudit func(ctx context.Context, actor *models.User, action, detail string)

// Passkey audit actions.
const (
	PasskeyActionAdd          = "passkey.add"
	PasskeyActionRename       = "passkey.rename"
	PasskeyActionRemove       = "passkey.remove"
	PasskeyActionCloneWarning = "passkey.clone_warning" // sign counter went backwards; sign-in refused
	PasskeyActionClearWarning = "passkey.clear_warning" // an administrator cleared a clone warning
)

// SetAudit sets where passkey changes are recorded. Without it they are
// written to the log only.
func (p *Passkeys) SetAudit(audit PasskeyAudit) { p.audit = audit }

// event records a passkey change through the audit seam, or the log.
func (p *Passkeys) event(ctx context.Context, actor *models.User, action, detail string) {
	if p.audit != nil {
		p.audit(ctx, actor, action, detail)
		return
	}
	log.Printf("Account event: %s by user %q (ID %d): %s", action, actor.Username, actor.ID, detail)
}

// NewPasskeys returns the passkey service. displayName is the club name shown
// in the browser's passkey prompt.
func NewPasskeys(service *Service, publicURL PublicURLSource, displayName func() string) *Passkeys {
	return &Passkeys{
		service:     service,
		creds:       repository.NewWebAuthnCredentialRepository(service.db),
		settings:    repository.NewSettingsRepository(service.db),
		publicURL:   publicURL,
		displayName: displayName,
		ceremonies:  newCeremonyStore(),
		begins:      newWindowLimiter(passkeyBeginLimit, passkeyBeginWindow),
		failures:    newWindowLimiter(passkeyFailureLimit, passkeyFailureWindow),
		now:         time.Now,
	}
}

// relyingPartyAddress decides the origin and relying-party ID passkeys are
// bound to for this request.
//
// With a public URL configured (PUBLIC_URL or the settings page; always the
// case in production, where startup requires it) that URL is the only origin
// accepted, and its host is the relying-party ID. The request's Host header
// plays no part.
//
// With no public URL, passkeys are off, with one exception for development:
// when DEV_MODE is on (which production refuses) and the request was made to
// "localhost", the origin is http://localhost with the request's port.
// localhost is the one plain-http origin browsers allow WebAuthn on, and a
// passkey made there is scoped to "localhost" and useless anywhere else, so
// trusting the Host header for exactly that name gives an attacker nothing.
func (p *Passkeys) relyingPartyAddress(r *http.Request) (origin, rpID string, err error) {
	pub, ok, err := p.publicURL.ResolvePublicURL()
	if err != nil {
		return "", "", fmt.Errorf("%w: invalid public URL: %v", ErrPasskeysUnavailable, err)
	}
	if ok {
		if config.Production() && !pub.HTTPS() {
			return "", "", fmt.Errorf("%w: the public URL is not https", ErrPasskeysUnavailable)
		}
		return pub.Origin, pub.Hostname, nil
	}
	if config.Production() || !config.DevMode() || r == nil {
		return "", "", fmt.Errorf("%w: no public URL is configured", ErrPasskeysUnavailable)
	}
	host, port := r.Host, ""
	if h, prt, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		host, port = h, prt
	}
	if !strings.EqualFold(host, config.FallbackHost) {
		return "", "", fmt.Errorf("%w: no public URL is configured and the request is not for localhost", ErrPasskeysUnavailable)
	}
	origin = "http://" + config.FallbackHost
	if port != "" {
		if n, convErr := strconv.Atoi(port); convErr != nil || n < 1 || n > 65535 {
			return "", "", fmt.Errorf("%w: bad port", ErrPasskeysUnavailable)
		}
		if port != "80" {
			origin += ":" + port
		}
	}
	return origin, config.FallbackHost, nil
}

// relyingParty builds the WebAuthn configuration for this request from the
// current public address.
func (p *Passkeys) relyingParty(r *http.Request) (*webauthn.WebAuthn, string, error) {
	origin, rpID, err := p.relyingPartyAddress(r)
	if err != nil {
		return nil, "", err
	}
	name := strings.TrimSpace(p.displayName())
	if name == "" {
		name = config.DefaultDisplayName
	}
	required := true
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: ceremonyLifetime, TimeoutUVD: ceremonyLifetime}
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: name,
		RPOrigins:     []string{origin},
		// No attestation: nothing is checked against a metadata service, so
		// the app never contacts anyone about a user's authenticator.
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			// Discoverable credentials, so sign-in needs no username.
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: &required,
			UserVerification:   protocol.VerificationPreferred,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		// For example a public URL whose host is an IP address, which cannot
		// be a relying-party ID.
		return nil, "", fmt.Errorf("%w: %v", ErrPasskeysUnavailable, err)
	}
	return wa, origin, nil
}

// Available reports whether passkeys can be used on this request's address.
func (p *Passkeys) Available(r *http.Request) bool {
	_, _, err := p.relyingParty(r)
	return err == nil
}

// passkeyUser adapts an account to the library's User interface.
type passkeyUser struct {
	handle []byte
	name   string
	creds  []webauthn.Credential
}

func (u *passkeyUser) WebAuthnID() []byte                         { return u.handle }
func (u *passkeyUser) WebAuthnName() string                       { return u.name }
func (u *passkeyUser) WebAuthnDisplayName() string                { return u.name }
func (u *passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// libraryCredential converts a stored passkey to the library's form.
func libraryCredential(m models.WebAuthnCredential) webauthn.Credential {
	var flags protocol.AuthenticatorFlags
	if m.UserPresent {
		flags |= protocol.FlagUserPresent
	}
	if m.UserVerified {
		flags |= protocol.FlagUserVerified
	}
	if m.BackupEligible {
		flags |= protocol.FlagBackupEligible
	}
	if m.BackupState {
		flags |= protocol.FlagBackupState
	}
	var transports []protocol.AuthenticatorTransport
	for _, t := range strings.Split(m.Transports, ",") {
		if t != "" {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
	}
	return webauthn.Credential{
		ID:                m.CredentialID,
		PublicKey:         m.PublicKey,
		AttestationType:   m.AttestationType,
		AttestationFormat: m.AttestationFormat,
		Transport:         transports,
		Flags:             webauthn.NewCredentialFlags(flags),
		Authenticator: webauthn.Authenticator{
			AAGUID:     m.AAGUID,
			SignCount:  m.SignCount,
			Attachment: protocol.AuthenticatorAttachment(m.Attachment),
		},
	}
}

// loadUser builds the library user for an account with its stored passkeys.
func (p *Passkeys) loadUser(ctx context.Context, user *models.User) (*passkeyUser, []models.WebAuthnCredential, error) {
	handle, err := p.creds.UserHandle(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	stored, err := p.creds.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	u := &passkeyUser{handle: handle, name: user.Username}
	for _, m := range stored {
		u.creds = append(u.creds, libraryCredential(m))
	}
	return u, stored, nil
}

// List returns userID's passkeys, oldest first.
func (p *Passkeys) List(ctx context.Context, userID int64) ([]models.WebAuthnCredential, error) {
	return p.creds.ListByUser(ctx, userID)
}

// BeginRegistration starts enrolling a passkey for the signed-in user. It
// returns the options for navigator.credentials.create() and the ceremony
// ticket to set as the browser's ceremony cookie. sessionFP ties the ceremony
// to the session that began it.
func (p *Passkeys) BeginRegistration(r *http.Request, user *models.User, sessionFP string) (*protocol.CredentialCreation, string, error) {
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return nil, "", err
	}
	now := p.now()
	if !p.begins.hit(ThrottleKey(r), now) {
		return nil, "", ErrPasskeyThrottled
	}
	wu, stored, err := p.loadUser(r.Context(), user)
	if err != nil {
		return nil, "", err
	}
	if len(stored) >= MaxPasskeysPerUser {
		return nil, "", ErrPasskeyLimit
	}
	// Excluding the user's existing passkeys stops the same authenticator
	// being enrolled twice.
	exclude := make([]protocol.CredentialDescriptor, 0, len(wu.creds))
	for _, c := range wu.creds {
		exclude = append(exclude, c.Descriptor())
	}
	options, data, err := wa.BeginRegistration(wu,
		webauthn.WithExclusions(exclude),
		webauthn.WithRegistrationOrigin(origin),
	)
	if err != nil {
		return nil, "", fmt.Errorf("begin passkey registration: %w", err)
	}
	ticket, err := p.ceremonies.put(ceremony{
		kind: ceremonyRegister, data: *data, origin: origin, userID: user.ID, session: sessionFP,
	}, now)
	if err != nil {
		return nil, "", err
	}
	return options, ticket, nil
}

// FinishRegistration verifies the browser's response to BeginRegistration and
// stores the new passkey for user. The ceremony is consumed whatever the
// outcome.
func (p *Passkeys) FinishRegistration(r *http.Request, user *models.User, sessionFP, ticket string, body io.Reader, name string) (*models.WebAuthnCredential, error) {
	m, err := p.verifyRegistration(r, user, sessionFP, ticket, body, name)
	if err != nil {
		return nil, err
	}
	if err := p.creds.Create(r.Context(), m); err != nil {
		return nil, err
	}
	p.event(r.Context(), user, PasskeyActionAdd, fmt.Sprintf("passkey_id=%d", m.ID))
	return m, nil
}

// verifyRegistration verifies the browser's response to BeginRegistration and
// returns the new passkey for user without storing it. The ceremony is
// consumed whatever the outcome. binding must be the value the ceremony was
// begun with (the session fingerprint, or an account link's binding).
func (p *Passkeys) verifyRegistration(r *http.Request, user *models.User, binding, ticket string, body io.Reader, name string) (*models.WebAuthnCredential, error) {
	now := p.now()
	c, ok := p.ceremonies.take(ticket, ceremonyRegister, now)
	if !ok {
		return nil, ErrPasskeyCeremony
	}
	// The ceremony belongs to the user and the session (or account link)
	// that began it.
	if c.userID != user.ID || c.session == "" || !hmac.Equal([]byte(c.session), []byte(binding)) {
		return nil, ErrPasskeyCeremony
	}
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return nil, err
	}
	// The public address may have changed since the ceremony began; only the
	// current one counts.
	if c.origin != origin || c.data.RelyingPartyID != wa.Config.RPID {
		return nil, ErrPasskeyCeremony
	}
	wu, stored, err := p.loadUser(r.Context(), user)
	if err != nil {
		return nil, err
	}
	if len(stored) >= MaxPasskeysPerUser {
		return nil, ErrPasskeyLimit
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(io.LimitReader(body, maxPasskeyBody))
	if err != nil {
		logPasskeyError("registration response", err)
		return nil, ErrPasskeyRejected
	}
	cred, err := wa.CreateCredential(wu, c.data, parsed)
	if err != nil {
		logPasskeyError("registration", err)
		return nil, ErrPasskeyRejected
	}

	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		// Transport hints come from the browser; keep only plain tokens.
		if s := string(t); s != "" && len(s) <= 20 && !strings.ContainsAny(s, ", \t\r\n") {
			transports = append(transports, s)
		}
	}
	m := &models.WebAuthnCredential{
		UserID:            user.ID,
		CredentialID:      cred.ID,
		PublicKey:         cred.PublicKey,
		AttestationType:   cred.AttestationType,
		AttestationFormat: cred.AttestationFormat,
		AAGUID:            cred.Authenticator.AAGUID,
		SignCount:         cred.Authenticator.SignCount,
		Transports:        strings.Join(transports, ","),
		Attachment:        string(cred.Authenticator.Attachment),
		UserPresent:       cred.Flags.UserPresent,
		UserVerified:      cred.Flags.UserVerified,
		BackupEligible:    cred.Flags.BackupEligible,
		BackupState:       cred.Flags.BackupState,
		Name:              PasskeyName(name, now),
		CreatedAt:         now,
	}
	return m, nil
}

// BeginLogin starts a passkey sign-in. With an empty username the browser
// offers any passkey it holds for this site (discoverable sign-in). With a
// username the options also list that account's passkeys, which helps
// browsers that cannot list them; the response has the same shape whether or
// not the username exists or has passkeys (see decoyDescriptors).
func (p *Passkeys) BeginLogin(r *http.Request, username string) (*protocol.CredentialAssertion, string, error) {
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return nil, "", err
	}
	now := p.now()
	ip := ThrottleKey(r)
	if p.failures.blocked(ip, now) || !p.begins.hit(ip, now) {
		return nil, "", ErrPasskeyThrottled
	}
	opts := []webauthn.LoginOption{webauthn.WithLoginOrigin(origin)}
	if username = strings.TrimSpace(username); username != "" {
		allowed, err := p.allowedDescriptors(r.Context(), username)
		if err != nil {
			return nil, "", err
		}
		opts = append(opts, webauthn.WithAllowedCredentials(allowed))
	}
	// Always begun as a discoverable sign-in: the account is resolved at the
	// end from the user handle the authenticator returns, never from the
	// username typed here.
	options, data, err := wa.BeginDiscoverableLogin(opts...)
	if err != nil {
		return nil, "", fmt.Errorf("begin passkey sign-in: %w", err)
	}
	ticket, err := p.ceremonies.put(ceremony{kind: ceremonyLogin, data: *data, origin: origin}, now)
	if err != nil {
		return nil, "", err
	}
	return options, ticket, nil
}

// allowedDescriptors is the allow list for a username-first sign-in: the
// account's passkeys, or decoys when there is no such active account or it
// has no passkeys. Only the type and id are sent (no transport hints), so
// real and decoy entries look alike.
func (p *Passkeys) allowedDescriptors(ctx context.Context, username string) ([]protocol.CredentialDescriptor, error) {
	var user models.User
	if err := p.service.db.GetContext(ctx, &user, `SELECT * FROM users WHERE username = ? AND is_active = true`, username); err == nil {
		stored, err := p.creds.ListByUser(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		if len(stored) > 0 {
			out := make([]protocol.CredentialDescriptor, 0, len(stored))
			for _, m := range stored {
				out = append(out, protocol.CredentialDescriptor{Type: protocol.PublicKeyCredentialType, CredentialID: m.CredentialID})
			}
			return out, nil
		}
	}
	return p.decoyDescriptors(ctx, username)
}

// decoyDescriptors returns a made-up credential id for username, so that
// asking about a username that does not exist, is inactive or has no
// passkeys gets an answer shaped like a real one. The id is derived from the
// username with a per-instance random key, so it is the same every time the
// same username is asked about (a fresh random id each time would give the
// game away) and cannot be told from a real id without the key. No
// authenticator holds it, and a ceremony begun with it can never succeed:
// the library requires the signing account to own every id in the allow
// list. What this does not hide: an account with several passkeys returns
// several ids, a decoy always one.
func (p *Passkeys) decoyDescriptors(ctx context.Context, username string) ([]protocol.CredentialDescriptor, error) {
	key, err := p.decoyKeyBytes(ctx)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("passkey-decoy:" + username))
	sum := mac.Sum(nil)
	// Real ids vary in length by authenticator; pick a common one.
	length := []int{16, 20, 32}[int(sum[len(sum)-1])%3]
	mac = hmac.New(sha256.New, key)
	mac.Write([]byte("passkey-decoy-id:" + username))
	id := mac.Sum(nil)[:length]
	return []protocol.CredentialDescriptor{{Type: protocol.PublicKeyCredentialType, CredentialID: id}}, nil
}

// decoyKeyBytes returns the instance's decoy key, generating and storing it
// on first use.
func (p *Passkeys) decoyKeyBytes(ctx context.Context) ([]byte, error) {
	p.decoyMu.Lock()
	defer p.decoyMu.Unlock()
	if p.decoyKey != nil {
		return p.decoyKey, nil
	}
	if stored, found, err := p.settings.GetString(ctx, repository.SettingPasskeyDecoyKey); err != nil {
		return nil, err
	} else if found {
		if key, err := hex.DecodeString(stored); err == nil && len(key) >= 16 {
			p.decoyKey = key
			return key, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := p.settings.SetString(ctx, repository.SettingPasskeyDecoyKey, hex.EncodeToString(key)); err != nil {
		return nil, err
	}
	p.decoyKey = key
	return key, nil
}

// FinishLogin verifies the browser's response to BeginLogin and returns the
// account that signed in. The ceremony is consumed whatever the outcome.
// Every verification failure is ErrPasskeyRejected and counts towards this
// client address's throttle (never towards any user's).
func (p *Passkeys) FinishLogin(r *http.Request, ticket string, body io.Reader) (*models.User, error) {
	now := p.now()
	ip := ThrottleKey(r)
	// Taken before anything else, so a ceremony is spent even by a throttled
	// or malformed attempt.
	c, ok := p.ceremonies.take(ticket, ceremonyLogin, now)
	if p.failures.blocked(ip, now) {
		return nil, ErrPasskeyThrottled
	}
	if !ok {
		return nil, ErrPasskeyCeremony
	}
	wa, origin, err := p.relyingParty(r)
	if err != nil {
		return nil, err
	}
	if c.origin != origin || c.data.RelyingPartyID != wa.Config.RPID {
		return nil, ErrPasskeyCeremony
	}

	reject := func(what string, err error) (*models.User, error) {
		logPasskeyError(what, err)
		p.failures.record(ip, now)
		p.service.recordLoginAttempt(passkeyAttemptUsername, r, false)
		return nil, ErrPasskeyRejected
	}

	parsed, err := protocol.ParseCredentialRequestResponseBody(io.LimitReader(body, maxPasskeyBody))
	if err != nil {
		return reject("sign-in response", err)
	}

	var (
		account *models.User
		stored  []models.WebAuthnCredential
	)
	// The account comes from the user handle inside the signed response.
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		userID, found, err := p.creds.UserIDByHandle(r.Context(), userHandle)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errors.New("unknown user handle")
		}
		var user models.User
		// Inactive accounts cannot sign in.
		if err := p.service.db.GetContext(r.Context(), &user, `SELECT * FROM users WHERE id = ? AND is_active = true`, userID); err != nil {
			return nil, errors.New("no active account for user handle")
		}
		wu, creds, err := p.loadUser(r.Context(), &user)
		if err != nil {
			return nil, err
		}
		account, stored = &user, creds
		return wu, nil
	}
	_, cred, err := wa.ValidatePasskeyLogin(handler, c.data, parsed)
	if err != nil {
		return reject("sign-in", err)
	}
	var used *models.WebAuthnCredential
	for i := range stored {
		if hmac.Equal(stored[i].CredentialID, cred.ID) {
			used = &stored[i]
		}
	}
	if account == nil || used == nil {
		return reject("sign-in", errors.New("verified credential not found in store"))
	}

	if cred.Authenticator.CloneWarning {
		// The signature is valid but the authenticator's counter did not go
		// up: either a buggy authenticator or a copy of it is in use. Do not
		// sign in; mark the passkey so administrators see it and it stays
		// refused until someone has looked.
		if used.CloneWarningAt == nil {
			if _, err := p.creds.SetCloneWarning(r.Context(), used.ID, &now); err != nil {
				log.Printf("Failed to store passkey clone warning for passkey ID %d: %v", used.ID, err)
			}
		}
		log.Printf("SECURITY WARNING: passkey ID %d of user %q (ID %d) reported a sign counter that did not increase (stored %d). "+
			"It may have been copied. The sign-in was refused and the passkey is blocked until an administrator clears the warning.",
			used.ID, account.Username, account.ID, used.SignCount)
		p.event(r.Context(), account, PasskeyActionCloneWarning, fmt.Sprintf("passkey_id=%d", used.ID))
		p.failures.record(ip, now)
		p.service.recordLoginAttempt(passkeyAttemptUsername, r, false)
		return nil, ErrPasskeyBlocked
	}
	if used.CloneWarningAt != nil {
		log.Printf("Refused sign-in with passkey ID %d of user ID %d: clone warning not cleared", used.ID, account.ID)
		p.failures.record(ip, now)
		p.service.recordLoginAttempt(passkeyAttemptUsername, r, false)
		return nil, ErrPasskeyBlocked
	}

	used.SignCount = cred.Authenticator.SignCount
	used.UserPresent = cred.Flags.UserPresent
	used.UserVerified = cred.Flags.UserVerified
	used.BackupEligible = cred.Flags.BackupEligible
	used.BackupState = cred.Flags.BackupState
	if err := p.creds.RecordUse(r.Context(), used, now); err != nil {
		// Without the new counter stored a later copy could not be noticed.
		log.Printf("Failed to record passkey use for passkey ID %d: %v", used.ID, err)
		return nil, fmt.Errorf("record passkey use: %w", err)
	}
	p.service.recordLoginAttempt(account.Username, r, true)
	return account, nil
}

// Rename renames one of user's own passkeys.
func (p *Passkeys) Rename(ctx context.Context, user *models.User, id int64, name string) error {
	if err := p.creds.Rename(ctx, user.ID, id, PasskeyName(name, p.now())); err != nil {
		return err
	}
	p.event(ctx, user, PasskeyActionRename, fmt.Sprintf("passkey_id=%d", id))
	return nil
}

// Remove deletes one of user's own passkeys, unless it is their last
// sign-in method (repository.ErrLastSignInMethod).
func (p *Passkeys) Remove(ctx context.Context, user *models.User, id int64) error {
	if err := p.creds.Delete(ctx, user.ID, id); err != nil {
		return err
	}
	p.event(ctx, user, PasskeyActionRemove, fmt.Sprintf("passkey_id=%d", id))
	return nil
}

// CloneWarnings lists passkeys blocked by a clone warning, for administrators.
func (p *Passkeys) CloneWarnings(ctx context.Context) ([]models.PasskeyWarning, error) {
	return p.creds.CloneWarnings(ctx)
}

// ClearCloneWarning lets a passkey sign in again after an administrator has
// reviewed its clone warning.
func (p *Passkeys) ClearCloneWarning(ctx context.Context, admin *models.User, id int64) error {
	found, err := p.creds.SetCloneWarning(ctx, id, nil)
	if err != nil {
		return err
	}
	if !found {
		return repository.ErrPasskeyNotFound
	}
	p.event(ctx, admin, PasskeyActionClearWarning, fmt.Sprintf("passkey_id=%d", id))
	return nil
}

// PasskeyName cleans a user-given passkey name: control characters removed,
// whitespace collapsed, at most MaxPasskeyNameLength characters. An empty
// name becomes "Passkey added <date>".
func PasskeyName(name string, now time.Time) string {
	name = strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	name = strings.ToValidUTF8(name, "")
	if utf8.RuneCountInString(name) > MaxPasskeyNameLength {
		name = strings.TrimSpace(string([]rune(name)[:MaxPasskeyNameLength]))
	}
	if name == "" {
		return "Passkey added " + now.Format("2 Jan 2006")
	}
	return name
}

// logPasskeyError logs why a ceremony failed. The library's debug text can
// include the expected challenge, so only the error type and summary are
// written.
func logPasskeyError(what string, err error) {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		log.Printf("Passkey %s failed: %s: %s", what, pe.Type, pe.Details)
		return
	}
	log.Printf("Passkey %s failed: %v", what, err)
}
