// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package courthive is the app's client for the bundled CourtHive tournament
// server. jim-dot-tennis is the identity source: it creates tournament
// accounts, decides who belongs to the club's provider and hands signed-in
// people over to tournament management with single-use tokens. The tournament
// server holds nothing that gives access to the app.
//
// The client authenticates by signing its own short-lived tokens with the
// tournament server's signing secret, which is generated on the club's own
// server at install and shared with the app through the environment. No API
// key or password is stored. Tokens are never logged and never returned to
// callers, with one exception: the single-use hand-over token, which exists to
// be put in a redirect.
//
// See docs/courthive_bundle_design.md, section 4.
package courthive

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ServiceAccountEmail is the tournament server account the app acts as.
	// The deployment creates it once, with a random password that is thrown
	// away; nobody signs in as it.
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
	// addresses. The tournament server needs an email-shaped key; these can
	// never receive mail.
	AccountDomain = "jim-dot-tennis.invalid"

	// RoleProviderAdmin sees and manages every tournament of the provider.
	RoleProviderAdmin = "PROVIDER_ADMIN"
	// RoleDirector sees only tournaments they own or are assigned to.
	RoleDirector = "DIRECTOR"

	// tokenLifetime is how long a token the app signs stays valid. Each
	// request signs a fresh one.
	tokenLifetime = 60 * time.Second

	// minSecretLength rejects obviously weak signing secrets. The deployment
	// generates 64 hex characters.
	minSecretLength = 32

	maxResponseBytes = 1 << 20
)

// userRoles are the tournament server's global roles a person needs for
// tournament management to work. Accounts made through the provisioner
// interface start with none, and the calendar is then refused.
var userRoles = []string{"client", "generate", "score"}

// ErrNotConfigured is returned when the tournament server's signing secret
// was not given to the app (tournaments are not switched on in the deployment).
var ErrNotConfigured = errors.New("courthive: the tournament server's signing secret is not configured")

// ErrLastAdministrator is returned when the tournament server refuses to
// remove or demote a provider's only administrator.
var ErrLastAdministrator = errors.New("courthive: the provider must keep at least one administrator")

// APIError is a failure reported by the tournament server.
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
// HTTP 201, so the body is always inspected.
const (
	codeAbbreviationExists = "ABBREVIATION_EXISTS"
	codeSSOIdentityExists  = "SSO_IDENTITY_EXISTS"
	codeEmailExists        = "EMAIL_EXISTS"
)

// Client talks to one tournament server.
type Client struct {
	baseURL string
	secret  []byte
	http    *http.Client
	now     func() time.Time
}

// New returns a client for the tournament server at baseURL (the internal
// address, for example http://courthive-server:8383). An empty secret gives a
// client whose calls return ErrNotConfigured; a secret that is set but too
// short is an error, so a weak value never goes unnoticed.
func New(baseURL, signingSecret string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("courthive: no server address")
	}
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("courthive: server address: %w", err)
	}
	signingSecret = strings.TrimSpace(signingSecret)
	if signingSecret != "" && len(signingSecret) < minSecretLength {
		return nil, fmt.Errorf("courthive: the signing secret is shorter than %d characters", minSecretLength)
	}
	return &Client{
		baseURL: baseURL,
		secret:  []byte(signingSecret),
		http:    &http.Client{Timeout: 15 * time.Second},
		now:     time.Now,
	}, nil
}

// Configured reports whether the client can act on the tournament server.
func (c *Client) Configured() bool { return c != nil && len(c.secret) > 0 }

// AccountEmail is the tournament account address for a jim-dot-tennis username.
func AccountEmail(username string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@" + AccountDomain
}

// sign returns a token valid for tokenLifetime, signed as the service account.
// With a provisioner id the token also reaches the provisioner interface.
func (c *Client) sign(provisionerID string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	now := c.now()
	claims := map[string]any{
		"email": ServiceAccountEmail,
		"roles": []string{"superadmin", "client"},
		"aud":   "admin",
		"iat":   now.Unix(),
		"exp":   now.Add(tokenLifetime).Unix(),
	}
	if provisionerID != "" {
		claims["roles"] = []string{"superadmin", "client", "provisioner"}
		claims["provisionerIds"] = []string{provisionerID}
	}
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	signingInput := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(signingInput))
	return signingInput + "." + enc.EncodeToString(mac.Sum(nil)), nil
}

// do sends one request and decodes the JSON answer into out (when not nil).
// provisionerID selects the kind of token; pass "" for administration calls.
// A body that carries an "error" field is a failure whatever the status.
func (c *Client) do(ctx context.Context, op, method, path, provisionerID string, body, out any) error {
	token, err := c.sign(provisionerID)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("courthive: %s: %w", op, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("courthive: %s: %w", op, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// The transport error can quote the URL but never the header.
		return fmt.Errorf("courthive: %s: %w", op, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("courthive: %s: reading the answer: %w", op, err)
	}

	var failure struct {
		Error   json.RawMessage `json:"error"`
		Code    string          `json:"code"`
		Message json.RawMessage `json:"message"`
	}
	_ = json.Unmarshal(raw, &failure)
	errText := jsonText(failure.Error)
	if resp.StatusCode >= 400 || errText != "" && isString(failure.Error) {
		msg := jsonText(failure.Message)
		if msg == "" {
			msg = errText
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return &APIError{Status: resp.StatusCode, Code: failure.Code, Message: truncate(msg, 200), Op: op}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("courthive: %s: unexpected answer: %w", op, err)
	}
	return nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/factory/version", nil)
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
// creating it when the server has none. created reports whether it was made
// by this call.
func (c *Client) EnsureProvisioner(ctx context.Context) (id string, created bool, err error) {
	var list struct {
		Provisioners []struct {
			ProvisionerID string `json:"provisionerId"`
			Name          string `json:"name"`
			IsActive      bool   `json:"isActive"`
		} `json:"provisioners"`
	}
	if err := c.do(ctx, "listing provisioners", http.MethodGet, "/admin/provisioners", "", nil, &list); err != nil {
		return "", false, err
	}
	for _, p := range list.Provisioners {
		if p.Name == ProvisionerName {
			if !p.IsActive {
				return "", false, fmt.Errorf("courthive: the %q provisioner exists but is switched off on the tournament server", ProvisionerName)
			}
			return p.ProvisionerID, false, nil
		}
	}
	var made struct {
		Provisioner struct {
			ProvisionerID string `json:"provisionerId"`
		} `json:"provisioner"`
	}
	err = c.do(ctx, "creating the provisioner", http.MethodPost, "/admin/provisioners", "",
		map[string]string{"name": ProvisionerName}, &made)
	if err != nil {
		return "", false, err
	}
	if made.Provisioner.ProvisionerID == "" {
		return "", false, errors.New("courthive: creating the provisioner: the server returned no id")
	}
	return made.Provisioner.ProvisionerID, true, nil
}

// Provider is a tournament provider as the provisioner sees it.
type Provider struct {
	ID           string `json:"providerId"`
	Abbreviation string `json:"organisationAbbreviation"`
	Name         string `json:"organisationName"`
	Managed      bool   `json:"managed"`
	Relationship string `json:"relationship"`
}

// Providers lists the providers on the tournament server, marking those the
// app's provisioner manages.
func (c *Client) Providers(ctx context.Context, provisionerID string) ([]Provider, error) {
	var out struct {
		Providers []Provider `json:"providers"`
	}
	if err := c.do(ctx, "listing providers", http.MethodGet, "/provisioner/providers", provisionerID, nil, &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
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
		providers, err := c.Providers(ctx, provisionerID)
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
		err := c.do(ctx, "taking ownership of the provider", http.MethodPost,
			"/admin/provisioners/"+url.PathEscape(provisionerID)+"/providers", "",
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
	err = c.do(ctx, "creating the provider", http.MethodPost, "/provisioner/providers", provisionerID,
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
func (c *Client) Members(ctx context.Context, provisionerID, providerID string) ([]Member, error) {
	var out struct {
		Users []Member `json:"users"`
	}
	path := "/provisioner/users?providerId=" + url.QueryEscape(providerID)
	if err := c.do(ctx, "listing tournament accounts", http.MethodGet, path, provisionerID, nil, &out); err != nil {
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
func (c *Client) EnsureAccount(ctx context.Context, provisionerID, providerID string, a Account) (userID string, err error) {
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
	createErr := c.do(ctx, "creating the tournament account", http.MethodPost, "/provisioner/users", provisionerID,
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
			members, err := c.Members(ctx, provisionerID, providerID)
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
		err := c.do(ctx, "setting the tournament role", http.MethodPut,
			"/provisioner/users/"+url.PathEscape(userID)+"/providers/"+url.PathEscape(providerID), "",
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

// RemoveAccount ends a person's tournament access: it removes their
// membership of the provider and their global roles. The tournament server
// reads membership on every request, so this takes effect at once, including
// for sessions that already exist. Removing someone who is not a member is
// not an error. Safe to repeat.
func (c *Client) RemoveAccount(ctx context.Context, providerID, userID, email string) error {
	if userID == "" || email == "" {
		return errors.New("courthive: removing a tournament account needs its id and address")
	}
	err := c.do(ctx, "removing the tournament membership", http.MethodDelete,
		"/provisioner/users/"+url.PathEscape(userID)+"/providers/"+url.PathEscape(providerID), "", nil, nil)
	if err != nil {
		return lastAdministrator(err)
	}
	return c.setRoles(ctx, email, []string{})
}

func (c *Client) setRoles(ctx context.Context, email string, roles []string) error {
	return c.do(ctx, "setting tournament account roles", http.MethodPost, "/auth/modify", "",
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
func (c *Client) HandoverToken(ctx context.Context, provisionerID, providerID, externalID string) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	err := c.do(ctx, "requesting a hand-over token", http.MethodPost, "/auth/sso/generate", provisionerID,
		map[string]string{"externalId": externalID, "ssoProvider": SSOProvider, "providerId": providerID}, &out)
	if err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("courthive: requesting a hand-over token: the server returned none")
	}
	return out.Token, nil
}
