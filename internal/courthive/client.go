// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package courthive is the app's client for the bundled CourtHive tournament
// stack. jim-dot-tennis is the identity source: it creates tournament
// accounts, decides who belongs to the club's provider and hands signed-in
// people over to tournament management with single-use tokens. The tournament
// stack holds nothing that gives access to the app.
//
// Two services are involved, both on the instance's internal network: the
// tournament server (providers, accounts, hand-over tokens) and CourtHive's
// identity service, which is the only signer of tokens. The client signs in
// to the identity service as the instance's service account, with a password
// generated on the club's own server at install, and uses the token it is
// given. It holds no signing key. Tokens and the password are never logged
// and never returned to callers, with one exception: the single-use hand-over
// token, which exists to be put in a redirect.
//
// See docs/courthive_bundle_design.md, section 4.
package courthive

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// ServiceAccountEmail is the account the app acts as. The deployment
	// creates it once as a tournament-stack administrator; no person uses it.
	ServiceAccountEmail = "service@jim-dot-tennis.invalid"

	// ProvisionerName is the name of the provisioner the app registers on the
	// tournament server.
	ProvisionerName = "jim-dot-tennis"

	// SSOProvider labels every tournament account the app creates.
	SSOProvider = "jim-dot-tennis"

	// InstanceExternalID identifies the permanent, never-used provider
	// administrator. The tournament server refuses to remove a provider's
	// last administrator; this account means a person can always be removed.
	InstanceExternalID = "instance"

	// AccountDomain is the non-routable domain of tournament account
	// addresses. The tournament stack needs an email-shaped key; these can
	// never receive mail.
	AccountDomain = "jim-dot-tennis.invalid"

	// RoleProviderAdmin sees and manages every tournament of the provider.
	RoleProviderAdmin = "PROVIDER_ADMIN"
	// RoleDirector sees only tournaments they own or are assigned to.
	RoleDirector = "DIRECTOR"

	// minPasswordLength rejects obviously weak service passwords. The
	// deployment generates 48 hex characters.
	minPasswordLength = 24

	// tokenMargin is how long before its expiry a service token stops being
	// reused.
	tokenMargin = 2 * time.Minute

	maxResponseBytes = 1 << 20
)

// userRoles are the global roles a person needs for tournament management to
// work. Accounts made through the provisioner interface start with none, and
// the calendar is then refused.
var userRoles = []string{"client", "generate", "score"}

// ErrNotConfigured is returned when the service account's password was not
// given to the app (tournaments are not switched on in the deployment).
var ErrNotConfigured = errors.New("courthive: the tournament service account is not configured")

// ErrLastAdministrator is returned when the tournament server refuses to
// remove or demote a provider's only administrator.
var ErrLastAdministrator = errors.New("courthive: the provider must keep at least one administrator")

// APIError is a failure reported by the tournament stack.
type APIError struct {
	Status  int    // HTTP status
	Code    string // the server's error code, when it gave one
	Message string // the server's message, truncated
	Op      string // what the client was doing
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("courthive: %s: %s (%s, HTTP %d)", e.Op, e.Message, e.Code, e.Status)
	}
	return fmt.Sprintf("courthive: %s: %s (HTTP %d)", e.Op, e.Message, e.Status)
}

// Error codes the provisioner interface reports. It sends some of them with
// a success status, so the body is always inspected.
const (
	codeAbbreviationExists = "ABBREVIATION_EXISTS"
	codeSSOIdentityExists  = "SSO_IDENTITY_EXISTS"
	codeEmailExists        = "EMAIL_EXISTS"
)

// Config says where the tournament stack is and how the app signs in to it.
type Config struct {
	// ServerURL is the tournament server's internal address, for example
	// http://courthive-server:8383.
	ServerURL string
	// IdentityURL is the identity service's internal address, for example
	// http://courthive-hiveid:3140.
	IdentityURL string
	// ServicePassword is the service account's password. Empty means
	// tournaments are not switched on.
	ServicePassword string
}

// Client talks to one instance's tournament stack.
type Client struct {
	serverURL   string
	identityURL string
	password    string
	http        *http.Client
	now         func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// New returns a client. An empty service password gives a client whose calls
// return ErrNotConfigured; a password that is set but too short is an error,
// so a weak value never goes unnoticed.
func New(cfg Config) (*Client, error) {
	clean := func(name, raw string) (string, error) {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if raw == "" {
			return "", fmt.Errorf("courthive: no %s address", name)
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("courthive: the %s address is not a URL", name)
		}
		return raw, nil
	}
	server, err := clean("tournament server", cfg.ServerURL)
	if err != nil {
		return nil, err
	}
	identity, err := clean("identity service", cfg.IdentityURL)
	if err != nil {
		return nil, err
	}
	password := strings.TrimSpace(cfg.ServicePassword)
	if password != "" && len(password) < minPasswordLength {
		return nil, fmt.Errorf("courthive: the service password is shorter than %d characters", minPasswordLength)
	}
	return &Client{
		serverURL:   server,
		identityURL: identity,
		password:    password,
		http:        &http.Client{Timeout: 15 * time.Second},
		now:         time.Now,
	}, nil
}

// Configured reports whether the client can act on the tournament stack.
func (c *Client) Configured() bool { return c != nil && c.password != "" }

// AccountEmail is the tournament account address for a jim-dot-tennis username.
func AccountEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@" + AccountDomain
}

// serviceToken returns a token for the service account, signing in to the
// identity service when there is none or the one held is about to expire.
func (c *Client) serviceToken(ctx context.Context) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.expires.Add(-tokenMargin)) {
		return c.token, nil
	}
	var out struct {
		Token              string `json:"token"`
		MustChangePassword bool   `json:"mustChangePassword"`
	}
	status, raw, err := c.send(ctx, "signing in to the identity service", http.MethodPost,
		c.identityURL+"/auth/login", "", map[string]string{"email": ServiceAccountEmail, "password": c.password})
	if err != nil {
		return "", err
	}
	if apiErr := failure("signing in to the identity service", status, raw); apiErr != nil {
		return "", apiErr
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		if out.MustChangePassword {
			return "", errors.New("courthive: the service account is flagged to change its password; recreate it with the deployment's set-up step")
		}
		return "", errors.New("courthive: signing in to the identity service: no token in the answer")
	}
	expires, err := tokenExpiry(out.Token)
	if err != nil {
		return "", fmt.Errorf("courthive: signing in to the identity service: %w", err)
	}
	c.token, c.expires = out.Token, expires
	return c.token, nil
}

// forgetToken drops the held token, so the next call signs in again. Used
// when the server refuses the token and after a change that alters what the
// service account's token must carry.
func (c *Client) forgetToken() {
	c.mu.Lock()
	c.token, c.expires = "", time.Time{}
	c.mu.Unlock()
}

// tokenExpiry reads the expiry from a token's payload. The client does not
// verify the signature: it received the token from the identity service over
// the internal network and only needs to know when to fetch another.
func tokenExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, errors.New("the token is not in the expected form")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, errors.New("the token is not in the expected form")
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, errors.New("the token carries no expiry")
	}
	return time.Unix(claims.Exp, 0), nil
}

// send performs one HTTP exchange and returns the status and body.
func (c *Client) send(ctx context.Context, op, method, target, token string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("courthive: %s: %w", op, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("courthive: %s: %w", op, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// The transport error can quote the URL but never a header or body.
		return 0, nil, fmt.Errorf("courthive: %s: %w", op, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("courthive: %s: reading the answer: %w", op, err)
	}
	return resp.StatusCode, raw, nil
}

// failure turns an answer into an APIError when it reports one. A body that
// carries an "error" string is a failure whatever the status.
func failure(op string, status int, raw []byte) *APIError {
	var body struct {
		Error   json.RawMessage `json:"error"`
		Code    string          `json:"code"`
		Message json.RawMessage `json:"message"`
	}
	_ = json.Unmarshal(raw, &body)
	errText := jsonText(body.Error)
	if status < 400 && !(errText != "" && isString(body.Error)) {
		return nil
	}
	msg := jsonText(body.Message)
	if msg == "" {
		msg = errText
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &APIError{Status: status, Code: body.Code, Message: truncate(msg, 200), Op: op}
}

// do sends one authenticated request to base+path and decodes the JSON answer
// into out (when not nil). A refused token is replaced once and the request
// repeated.
func (c *Client) do(ctx context.Context, op, method, base, path string, body, out any) error {
	for attempt := 0; ; attempt++ {
		token, err := c.serviceToken(ctx)
		if err != nil {
			return err
		}
		status, raw, err := c.send(ctx, op, method, base+path, token, body)
		if err != nil {
			return err
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			c.forgetToken()
			continue
		}
		if apiErr := failure(op, status, raw); apiErr != nil {
			return apiErr
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("courthive: %s: unexpected answer: %w", op, err)
		}
		return nil
	}
}

func isString(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '"'
}

// jsonText renders a JSON string, or a list of strings, as plain text.
func jsonText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, "; ")
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func hasCode(err error, code string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// Version returns the tournament engine version the server runs. It needs no
// authority and doubles as a reachability check.
func (c *Client) Version(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.serverURL+"/factory/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("courthive: reaching the tournament server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &APIError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode), Op: "reading the version"}
	}
	var out struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&out); err != nil {
		return "", fmt.Errorf("courthive: reading the version: %w", err)
	}
	return out.Version, nil
}

// EnsureProvisioner returns the id of the provisioner named ProvisionerName,
// creating it when the server has none, and makes sure the service account
// represents it (which is what lets the service account's token use the
// provisioner interface). created reports whether the provisioner was made by
// this call. Safe to repeat.
func (c *Client) EnsureProvisioner(ctx context.Context) (id string, created bool, err error) {
	var list struct {
		Provisioners []struct {
			ProvisionerID string `json:"provisionerId"`
			Name          string `json:"name"`
			IsActive      bool   `json:"isActive"`
		} `json:"provisioners"`
	}
	if err := c.do(ctx, "listing provisioners", http.MethodGet, c.serverURL, "/admin/provisioners", nil, &list); err != nil {
		return "", false, err
	}
	for _, p := range list.Provisioners {
		if p.Name == ProvisionerName {
			if !p.IsActive {
				return "", false, fmt.Errorf("courthive: the %q provisioner exists but is switched off on the tournament server", ProvisionerName)
			}
			return p.ProvisionerID, false, c.ensureRepresentative(ctx, p.ProvisionerID)
		}
	}
	var made struct {
		Provisioner struct {
			ProvisionerID string `json:"provisionerId"`
		} `json:"provisioner"`
	}
	err = c.do(ctx, "creating the provisioner", http.MethodPost, c.serverURL, "/admin/provisioners",
		map[string]string{"name": ProvisionerName}, &made)
	if err != nil {
		return "", false, err
	}
	if made.Provisioner.ProvisionerID == "" {
		return "", false, errors.New("courthive: creating the provisioner: the server returned no id")
	}
	return made.Provisioner.ProvisionerID, true, c.ensureRepresentative(ctx, made.Provisioner.ProvisionerID)
}

// ensureRepresentative assigns the service account to the provisioner when it
// is not already assigned, then drops the held token: the identity service
// puts the provisioner into the token at sign-in.
func (c *Client) ensureRepresentative(ctx context.Context, provisionerID string) error {
	path := "/admin/provisioners/" + url.PathEscape(provisionerID) + "/users"
	var current json.RawMessage
	if err := c.do(ctx, "listing the provisioner's representatives", http.MethodGet, c.serverURL, path, nil, &current); err != nil {
		return err
	}
	if bytes.Contains(current, []byte(`"`+ServiceAccountEmail+`"`)) {
		return nil
	}
	err := c.do(ctx, "assigning the service account to the provisioner", http.MethodPost, c.serverURL, path,
		map[string]string{"email": ServiceAccountEmail}, nil)
	if err != nil {
		return err
	}
	c.forgetToken()
	return nil
}

// Provider is a tournament provider as the provisioner sees it.
type Provider struct {
	ID           string `json:"providerId"`
	Abbreviation string `json:"organisationAbbreviation"`
	Name         string `json:"organisationName"`
	Managed      bool   `json:"managed"`
	Relationship string `json:"relationship"`
}

// Providers lists every provider on the tournament server, marking those the
// app's provisioner manages. The provisioner's own listing shows only the
// providers it already manages, so the rest (made by hand, or by an earlier
// installation) come from the administrators' listing, which the service
// account may read.
func (c *Client) Providers(ctx context.Context) ([]Provider, error) {
	var managed struct {
		Providers []Provider `json:"providers"`
	}
	if err := c.do(ctx, "listing providers", http.MethodGet, c.serverURL, "/provisioner/providers", nil, &managed); err != nil {
		return nil, err
	}
	out := managed.Providers
	seen := map[string]bool{}
	for i := range out {
		out[i].Managed = true
		seen[out[i].ID] = true
	}
	var all struct {
		Providers []struct {
			Key   string `json:"key"`
			Value struct {
				ID           string `json:"organisationId"`
				Abbreviation string `json:"organisationAbbreviation"`
				Name         string `json:"organisationName"`
			} `json:"value"`
		} `json:"providers"`
	}
	if err := c.do(ctx, "listing every provider", http.MethodPost, c.serverURL, "/provider/allproviders", map[string]any{}, &all); err != nil {
		return nil, err
	}
	for _, p := range all.Providers {
		id := p.Value.ID
		if id == "" {
			id = p.Key
		}
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Provider{ID: id, Abbreviation: p.Value.Abbreviation, Name: p.Value.Name})
	}
	return out, nil
}

// EnsureProvider returns the id of the provider with the given abbreviation,
// owned by the app's provisioner. A provider that already exists (for example
// one made by hand before the bundle) is adopted, not duplicated. created
// reports whether it was made by this call.
func (c *Client) EnsureProvider(ctx context.Context, provisionerID, abbreviation, name string) (id string, created bool, err error) {
	abbreviation = strings.TrimSpace(abbreviation)
	if abbreviation == "" {
		return "", false, errors.New("courthive: the provider needs an abbreviation")
	}
	find := func() (*Provider, error) {
		providers, err := c.Providers(ctx)
		if err != nil {
			return nil, err
		}
		for i := range providers {
			if strings.EqualFold(providers[i].Abbreviation, abbreviation) {
				return &providers[i], nil
			}
		}
		return nil, nil
	}
	adopt := func(p *Provider) (string, bool, error) {
		if p.Managed {
			return p.ID, false, nil
		}
		err := c.do(ctx, "taking ownership of the provider", http.MethodPost, c.serverURL, "/admin/provisioners/"+url.PathEscape(provisionerID)+"/providers",
			map[string]string{"providerId": p.ID, "relationship": "owner"}, nil)
		if err != nil {
			return "", false, err
		}
		return p.ID, false, nil
	}

	existing, err := find()
	if err != nil {
		return "", false, err
	}
	if existing != nil {
		return adopt(existing)
	}
	var made struct {
		ProviderID string `json:"providerId"`
	}
	err = c.do(ctx, "creating the provider", http.MethodPost, c.serverURL, "/provisioner/providers",
		map[string]string{"organisationAbbreviation": abbreviation, "organisationName": name}, &made)
	if hasCode(err, codeAbbreviationExists) {
		// Made between the listing and the create, or hidden from the listing.
		existing, findErr := find()
		if findErr != nil {
			return "", false, findErr
		}
		if existing == nil {
			return "", false, err
		}
		return adopt(existing)
	}
	if err != nil {
		return "", false, err
	}
	if made.ProviderID == "" {
		return "", false, errors.New("courthive: creating the provider: the server returned no id")
	}
	return made.ProviderID, true, nil
}

// Member is a tournament account that belongs to the provider.
type Member struct {
	UserID     string `json:"userId"`
	Email      string `json:"email"`
	Role       string `json:"providerRole"`
	ExternalID string `json:"externalId"`
	Provider   string `json:"ssoProvider"`
}

// Members lists the accounts that currently belong to the provider.
func (c *Client) Members(ctx context.Context, providerID string) ([]Member, error) {
	var out struct {
		Users []Member `json:"users"`
	}
	path := "/provisioner/users?providerId=" + url.QueryEscape(providerID)
	if err := c.do(ctx, "listing tournament accounts", http.MethodGet, c.serverURL, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

// Account describes the tournament account wanted for one person.
type Account struct {
	// ExternalID is the stable identifier on the app's side (the user id).
	ExternalID string
	// Email is the account address; use AccountEmail.
	Email string
	// Role is RoleProviderAdmin or RoleDirector.
	Role string
	// UserID is the tournament server's id for the account when the app
	// already knows it (from an earlier call); empty for a first creation.
	UserID string
}

// ErrAccountUnknown is returned by EnsureAccount when the account exists on
// the tournament server, is not a member of the provider, and the caller did
// not supply its id. The server offers no lookup for that case; the app keeps
// the id it was given at creation.
var ErrAccountUnknown = errors.New("courthive: the tournament account exists but its id is not known to the app")

// EnsureAccount makes sure the person has a password-less tournament account
// with the given role at the provider and the global roles tournament
// management needs. It returns the tournament server's id for the account,
// which the caller stores. Safe to repeat.
func (c *Client) EnsureAccount(ctx context.Context, providerID string, a Account) (userID string, err error) {
	if a.Role != RoleProviderAdmin && a.Role != RoleDirector {
		return "", fmt.Errorf("courthive: unknown tournament role %q", a.Role)
	}
	if a.ExternalID == "" || a.Email == "" {
		return "", errors.New("courthive: a tournament account needs an external id and an address")
	}
	userID = a.UserID

	var made struct {
		UserID string `json:"userId"`
	}
	createErr := c.do(ctx, "creating the tournament account", http.MethodPost, c.serverURL, "/provisioner/users",
		map[string]string{
			"providerId":   providerID,
			"externalId":   a.ExternalID,
			"email":        a.Email,
			"providerRole": a.Role,
			"ssoProvider":  SSOProvider,
		}, &made)
	switch {
	case createErr == nil:
		userID = made.UserID
	case hasCode(createErr, codeSSOIdentityExists), hasCode(createErr, codeEmailExists):
		if userID == "" {
			members, err := c.Members(ctx, providerID)
			if err != nil {
				return "", err
			}
			for _, m := range members {
				if m.Provider == SSOProvider && m.ExternalID == a.ExternalID {
					userID = m.UserID
					break
				}
			}
		}
		if userID == "" {
			return "", ErrAccountUnknown
		}
		// Membership may have been removed, or the role may differ: set it.
		err := c.do(ctx, "setting the tournament role", http.MethodPut, c.serverURL, "/provisioner/users/"+url.PathEscape(userID)+"/providers/"+url.PathEscape(providerID),
			map[string]string{"providerRole": a.Role}, nil)
		if err != nil {
			return "", lastAdministrator(err)
		}
	default:
		return "", createErr
	}
	if userID == "" {
		return "", errors.New("courthive: creating the tournament account: the server returned no id")
	}
	if err := c.setRoles(ctx, a.Email, userRoles); err != nil {
		return "", err
	}
	return userID, nil
}

// RemoveMembership removes a person's membership of one provider. The
// tournament server reads membership on every request, so this takes effect
// at once, including for sessions that already exist. Removing someone who is
// not a member is not an error. Safe to repeat. A person left in no provider
// should also have their global roles cleared (ClearRoles).
func (c *Client) RemoveMembership(ctx context.Context, providerID, userID string) error {
	if userID == "" || providerID == "" {
		return errors.New("courthive: removing a tournament membership needs the account's id and the provider's")
	}
	err := c.do(ctx, "removing the tournament membership", http.MethodDelete, c.serverURL, "/provisioner/users/"+url.PathEscape(userID)+"/providers/"+url.PathEscape(providerID), nil, nil)
	if err != nil {
		return lastAdministrator(err)
	}
	return nil
}

// ClearRoles removes the global roles tournament management needs from the
// account with this address: the last step of ending a person's tournament
// access, after their memberships are removed. Safe to repeat.
func (c *Client) ClearRoles(ctx context.Context, email string) error {
	if email == "" {
		return errors.New("courthive: clearing tournament roles needs the account's address")
	}
	return c.setRoles(ctx, email, []string{})
}

func (c *Client) setRoles(ctx context.Context, email string, roles []string) error {
	return c.do(ctx, "setting tournament account roles", http.MethodPost, c.identityURL, "/auth/modify",
		map[string]any{"email": email, "roles": roles}, nil)
}

func lastAdministrator(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
		return ErrLastAdministrator
	}
	return err
}

// HandoverToken asks the tournament server for a single-use token that signs
// the person in to tournament management. It expires in a minute and can be
// exchanged once. The caller must already have decided that the person is
// entitled; this call does not check.
func (c *Client) HandoverToken(ctx context.Context, providerID, externalID string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	err := c.do(ctx, "requesting a hand-over token", http.MethodPost, c.serverURL, "/auth/sso/generate",
		map[string]string{"externalId": externalID, "ssoProvider": SSOProvider, "providerId": providerID}, &out)
	if err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("courthive: requesting a hand-over token: the server returned none")
	}
	return out.Token, nil
}
