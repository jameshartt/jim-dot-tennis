// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package courthive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPassword = "0123456789abcdef0123456789abcdef0123456789abcdef"

// fakeServer imitates the parts of the tournament server and the identity
// service the client uses, including the habit of reporting some failures
// with a success status. Sign-in issues a token that records whether the
// service account represented a provisioner at that moment, as the real
// identity service does.
type fakeServer struct {
	t        *testing.T
	mu       sync.Mutex
	password string
	lifetime time.Duration

	logins          int
	tokens          map[string]bool // token -> carries the provisioner claim
	representatives map[string]bool // provisioner id -> service account assigned
	provisioners    map[string]string
	providers       map[string]*Provider
	users           map[string]*fakeUser // email -> user
	calls           []string
}

type fakeUser struct {
	id, email, externalID string
	roles                 []string
	membership            map[string]string // providerID -> role
}

func newFake(t *testing.T) (*fakeServer, *httptest.Server) {
	f := &fakeServer{t: t, password: testPassword, lifetime: 4 * time.Hour, tokens: map[string]bool{}, representatives: map[string]bool{},
		provisioners: map[string]string{}, providers: map[string]*Provider{}, users: map[string]*fakeUser{}}
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	return f, srv
}

// issue makes a token in the three-part form with an expiry in its payload.
func (f *fakeServer) issue() string {
	f.logins++
	enc := base64.RawURLEncoding
	payload, _ := json.Marshal(map[string]any{"email": ServiceAccountEmail, "exp": time.Now().Add(f.lifetime).Unix(), "n": f.logins})
	token := enc.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." + enc.EncodeToString(payload) + "." + enc.EncodeToString([]byte(fmt.Sprintf("sig%d", f.logins)))
	f.tokens[token] = len(f.representatives) > 0
	return token
}

func (f *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	if r.URL.Path == "/factory/version" {
		reply(200, map[string]string{"version": "7.1.1"})
		return
	}
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	str := func(k string) string { s, _ := body[k].(string); return s }
	if r.URL.Path == "/auth/login" {
		if str("email") != ServiceAccountEmail || str("password") != f.password {
			reply(401, map[string]any{"message": "Unauthorized", "statusCode": 401})
			return
		}
		reply(200, map[string]any{"token": f.issue(), "refreshToken": "r"})
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	isProvisioner, ok := f.tokens[token]
	if !ok {
		reply(401, map[string]any{"message": "Unauthorized", "statusCode": 401})
		return
	}
	path := r.URL.Path

	switch {
	case path == "/admin/provisioners" && r.Method == http.MethodGet:
		list := []map[string]any{}
		for name, id := range f.provisioners {
			list = append(list, map[string]any{"provisionerId": id, "name": name, "isActive": true})
		}
		reply(200, map[string]any{"success": true, "provisioners": list})
	case path == "/admin/provisioners" && r.Method == http.MethodPost:
		id := "prov-" + str("name")
		f.provisioners[str("name")] = id
		reply(201, map[string]any{"success": true, "provisioner": map[string]any{"provisionerId": id, "name": str("name")}})
	case strings.HasPrefix(path, "/admin/provisioners/") && strings.HasSuffix(path, "/users"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/admin/provisioners/"), "/users")
		if r.Method == http.MethodPost {
			f.representatives[id] = true
			reply(201, map[string]any{"success": true})
			return
		}
		list := []map[string]any{}
		if f.representatives[id] {
			list = append(list, map[string]any{"email": ServiceAccountEmail})
		}
		reply(200, map[string]any{"success": true, "users": list})
	case strings.HasPrefix(path, "/admin/provisioners/") && strings.HasSuffix(path, "/providers") && r.Method == http.MethodPost:
		for _, p := range f.providers {
			if p.ID == str("providerId") {
				p.Managed, p.Relationship = true, str("relationship")
			}
		}
		reply(201, map[string]any{"success": true})
	case strings.HasPrefix(path, "/provisioner/") && !isProvisioner && !strings.Contains(path, "/providers/"):
		reply(401, map[string]any{"message": "Provisioner API key required", "statusCode": 401})
	case path == "/provisioner/providers" && r.Method == http.MethodGet:
		// As the real server: only the providers the provisioner manages.
		list := []Provider{}
		for _, p := range f.providers {
			if p.Managed {
				list = append(list, *p)
			}
		}
		reply(200, map[string]any{"providers": list})
	case path == "/provider/allproviders" && r.Method == http.MethodPost:
		list := []map[string]any{}
		for _, p := range f.providers {
			list = append(list, map[string]any{"key": p.ID, "value": map[string]any{
				"organisationId": p.ID, "organisationAbbreviation": p.Abbreviation, "organisationName": p.Name}})
		}
		reply(200, map[string]any{"success": true, "providers": list})
	case path == "/provisioner/providers" && r.Method == http.MethodPost:
		abbr := str("organisationAbbreviation")
		if _, exists := f.providers[abbr]; exists {
			reply(201, map[string]any{"error": "organisationAbbreviation already exists", "code": "ABBREVIATION_EXISTS"})
			return
		}
		f.providers[abbr] = &Provider{ID: "provider-" + abbr, Abbreviation: abbr, Name: str("organisationName"), Managed: true, Relationship: "owner"}
		reply(201, map[string]any{"success": true, "providerId": "provider-" + abbr, "organisationAbbreviation": abbr})
	case path == "/provisioner/users" && r.Method == http.MethodGet:
		providerID := r.URL.Query().Get("providerId")
		list := []Member{}
		for _, u := range f.users {
			if role, ok := u.membership[providerID]; ok {
				list = append(list, Member{UserID: u.id, Email: u.email, Role: role, ExternalID: u.externalID, Provider: SSOProvider})
			}
		}
		reply(200, map[string]any{"users": list})
	case path == "/provisioner/users" && r.Method == http.MethodPost:
		for _, u := range f.users {
			if u.externalID == str("externalId") {
				reply(201, map[string]any{"error": "SSO identity already exists for this provider", "code": "SSO_IDENTITY_EXISTS"})
				return
			}
		}
		if _, exists := f.users[str("email")]; exists {
			reply(201, map[string]any{"error": "Email already registered", "code": "EMAIL_EXISTS"})
			return
		}
		u := &fakeUser{id: "user-" + str("externalId"), email: str("email"), externalID: str("externalId"), membership: map[string]string{str("providerId"): str("providerRole")}}
		f.users[u.email] = u
		reply(201, map[string]any{"success": true, "userId": u.id})
	case strings.HasPrefix(path, "/provisioner/users/") && strings.Contains(path, "/providers/"):
		seg := strings.Split(strings.TrimPrefix(path, "/provisioner/users/"), "/providers/")
		var user *fakeUser
		for _, u := range f.users {
			if u.id == seg[0] {
				user = u
			}
		}
		if user == nil {
			reply(404, map[string]any{"message": "Not found", "statusCode": 404})
			return
		}
		admins := 0
		for _, u := range f.users {
			if u.membership[seg[1]] == RoleProviderAdmin {
				admins++
			}
		}
		losingAdmin := user.membership[seg[1]] == RoleProviderAdmin && (r.Method == http.MethodDelete || str("providerRole") != RoleProviderAdmin)
		if losingAdmin && admins <= 1 {
			reply(409, map[string]any{"message": "Provider must retain at least one PROVIDER_ADMIN", "error": "Conflict", "statusCode": 409})
			return
		}
		if r.Method == http.MethodDelete {
			delete(user.membership, seg[1])
			reply(200, map[string]any{"success": true})
			return
		}
		user.membership[seg[1]] = str("providerRole")
		reply(200, map[string]any{"userId": user.id, "providerRole": str("providerRole")})
	case path == "/auth/modify":
		u, ok := f.users[str("email")]
		if !ok {
			reply(200, map[string]any{"error": "User not found"})
			return
		}
		u.roles = nil
		for _, r := range body["roles"].([]any) {
			u.roles = append(u.roles, r.(string))
		}
		reply(200, map[string]any{"success": true})
	case path == "/auth/sso/generate":
		if !isProvisioner {
			reply(401, map[string]any{"message": "Provisioner API key required", "statusCode": 401})
			return
		}
		for _, u := range f.users {
			if u.externalID == str("externalId") {
				reply(200, map[string]any{"token": "one-time-" + u.externalID, "expiresIn": 60})
				return
			}
		}
		reply(200, map[string]any{"error": "SSO identity not found — create the user first via POST /provisioner/users"})
	default:
		reply(404, map[string]any{"message": "Cannot " + r.Method + " " + path, "statusCode": 404})
	}
}

func newClient(t *testing.T, url, password string) *Client {
	t.Helper()
	c, err := New(Config{ServerURL: url, IdentityURL: url, ServicePassword: password})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewRejectsWeakPasswordAndAllowsNone(t *testing.T) {
	if _, err := New(Config{ServerURL: "http://a:1", IdentityURL: "http://b:2", ServicePassword: "short"}); err == nil {
		t.Fatal("a short service password must be refused")
	}
	if _, err := New(Config{IdentityURL: "http://b:2", ServicePassword: testPassword}); err == nil {
		t.Fatal("a missing server address must be refused")
	}
	if _, err := New(Config{ServerURL: "http://a:1", IdentityURL: "not a url", ServicePassword: testPassword}); err == nil {
		t.Fatal("a bad identity address must be refused")
	}
	c, err := New(Config{ServerURL: "http://a:1", IdentityURL: "http://b:2"})
	if err != nil {
		t.Fatalf("no password is a valid, unconfigured client: %v", err)
	}
	if c.Configured() {
		t.Fatal("a client with no password must not report itself configured")
	}
	if _, _, err := c.EnsureProvisioner(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
	if _, err := c.HandoverToken(context.Background(), "q", "1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestWrongPasswordIsRefused(t *testing.T) {
	f, srv := newFake(t)
	other := newClient(t, srv.URL, strings.Repeat("z", 48)) // another instance's password
	_, _, err := other.EnsureProvisioner(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("want a 401 from sign-in, got %v", err)
	}
	if len(f.tokens) != 0 {
		t.Fatal("no token should have been issued")
	}
}

func TestTokenIsReusedThenReplaced(t *testing.T) {
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testPassword)
	ctx := context.Background()
	if _, _, err := c.EnsureProvisioner(ctx); err != nil {
		t.Fatal(err)
	}
	// The provisioner interface needs a token issued after the service
	// account was assigned, so the first call after set-up signs in again.
	if _, err := c.Providers(ctx); err != nil {
		t.Fatalf("the token held from before the assignment must have been dropped: %v", err)
	}
	after := f.logins
	if after != 2 {
		t.Fatalf("signed in %d times, want 2 (once before the assignment, once after)", after)
	}
	for i := 0; i < 5; i++ {
		if _, err := c.Providers(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if f.logins != after {
		t.Fatalf("the token was not reused: %d sign-ins", f.logins)
	}

	// Near expiry: a fresh sign-in.
	c.now = func() time.Time { return time.Now().Add(4*time.Hour - time.Minute) }
	if _, err := c.Providers(ctx); err != nil {
		t.Fatal(err)
	}
	// (Listing providers is two requests; with the clock held near expiry
	// each of them signs in afresh.)
	if f.logins <= after {
		t.Fatalf("an expiring token must be replaced: %d sign-ins", f.logins)
	}
	c.now = time.Now
	before := f.logins

	// Refused by the server (for example after its keys changed): one retry.
	f.mu.Lock()
	f.tokens = map[string]bool{}
	f.mu.Unlock()
	if _, err := c.Providers(ctx); err != nil {
		t.Fatalf("a refused token should be replaced once: %v", err)
	}
	if f.logins != before+1 {
		t.Fatalf("sign-ins = %d", f.logins)
	}
}

func TestSetUpIsRepeatable(t *testing.T) {
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testPassword)
	ctx := context.Background()

	if v, err := c.Version(ctx); err != nil || v != "7.1.1" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	pid, created, err := c.EnsureProvisioner(ctx)
	if err != nil || !created || pid == "" {
		t.Fatalf("first EnsureProvisioner = %q, %v, %v", pid, created, err)
	}
	pid2, created, err := c.EnsureProvisioner(ctx)
	if err != nil || created || pid2 != pid {
		t.Fatalf("second EnsureProvisioner = %q, %v, %v", pid2, created, err)
	}
	prov, created, err := c.EnsureProvider(ctx, pid, "CLUBA", "Club A")
	if err != nil || !created {
		t.Fatalf("first EnsureProvider = %q, %v, %v", prov, created, err)
	}
	prov2, created, err := c.EnsureProvider(ctx, pid, "cluba", "Club A")
	if err != nil || created || prov2 != prov {
		t.Fatalf("second EnsureProvider = %q, %v, %v", prov2, created, err)
	}
	if len(f.providers) != 1 || len(f.provisioners) != 1 {
		t.Fatalf("repeat created duplicates: %d providers, %d provisioners", len(f.providers), len(f.provisioners))
	}
}

func TestEnsureProviderAdoptsOneMadeByHand(t *testing.T) {
	f, srv := newFake(t)
	f.providers["STANNS"] = &Provider{ID: "provider-old", Abbreviation: "STANNS", Name: "St Ann's", Managed: false}
	c := newClient(t, srv.URL, testPassword)
	ctx := context.Background()
	pid, _, err := c.EnsureProvisioner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	id, created, err := c.EnsureProvider(ctx, pid, "STANNS", "St Ann's")
	if err != nil || created || id != "provider-old" {
		t.Fatalf("EnsureProvider = %q, %v, %v; want the existing provider adopted", id, created, err)
	}
	if !f.providers["STANNS"].Managed || f.providers["STANNS"].Relationship != "owner" {
		t.Fatal("the existing provider was not taken into ownership")
	}
}

func setUp(t *testing.T) (*fakeServer, *Client, string, string) {
	t.Helper()
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testPassword)
	ctx := context.Background()
	pid, _, err := c.EnsureProvisioner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prov, _, err := c.EnsureProvider(ctx, pid, "CLUBA", "Club A")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.EnsureAccount(ctx, prov, Account{ExternalID: InstanceExternalID, Email: AccountEmail(InstanceExternalID), Role: RoleProviderAdmin}); err != nil {
		t.Fatal(err)
	}
	return f, c, pid, prov
}

func TestAccountLifecycle(t *testing.T) {
	f, c, _, prov := setUp(t)
	ctx := context.Background()
	acct := Account{ExternalID: "7", Email: AccountEmail("Jo.Bloggs"), Role: RoleProviderAdmin}
	if acct.Email != "jo.bloggs@jim-dot-tennis.invalid" {
		t.Fatalf("AccountEmail = %q", acct.Email)
	}

	uid, err := c.EnsureAccount(ctx, prov, acct)
	if err != nil || uid == "" {
		t.Fatalf("EnsureAccount = %q, %v", uid, err)
	}
	u := f.users[acct.Email]
	if strings.Join(u.roles, ",") != "client,generate,score" {
		t.Fatalf("roles = %v; the calendar is refused without client", u.roles)
	}
	if u.membership[prov] != RoleProviderAdmin {
		t.Fatalf("membership = %v", u.membership)
	}

	// Repeat with no stored id: found through the membership list.
	uid2, err := c.EnsureAccount(ctx, prov, acct)
	if err != nil || uid2 != uid {
		t.Fatalf("repeat EnsureAccount = %q, %v", uid2, err)
	}

	// Demote to director.
	acct.Role, acct.UserID = RoleDirector, uid
	if _, err := c.EnsureAccount(ctx, prov, acct); err != nil {
		t.Fatal(err)
	}
	if u.membership[prov] != RoleDirector {
		t.Fatalf("role after demotion = %q", u.membership[prov])
	}

	token, err := c.HandoverToken(ctx, prov, "7")
	if err != nil || token == "" {
		t.Fatalf("HandoverToken = %q, %v", token, err)
	}

	// Remove: membership and roles both go.
	if err := removeAccount(ctx, c, prov, uid, acct.Email); err != nil {
		t.Fatal(err)
	}
	if _, member := u.membership[prov]; member || len(u.roles) != 0 {
		t.Fatalf("after removal: membership %v roles %v", u.membership, u.roles)
	}
	// Removing again is not an error.
	if err := removeAccount(ctx, c, prov, uid, acct.Email); err != nil {
		t.Fatalf("second removal: %v", err)
	}

	// Coming back without the stored id cannot be resolved; with it, it can.
	if _, err := c.EnsureAccount(ctx, prov, Account{ExternalID: "7", Email: acct.Email, Role: RoleDirector}); !errors.Is(err, ErrAccountUnknown) {
		t.Fatalf("want ErrAccountUnknown, got %v", err)
	}
	if _, err := c.EnsureAccount(ctx, prov, Account{ExternalID: "7", Email: acct.Email, Role: RoleProviderAdmin, UserID: uid}); err != nil {
		t.Fatal(err)
	}
	if u.membership[prov] != RoleProviderAdmin || len(u.roles) != 3 {
		t.Fatalf("after return: membership %v roles %v", u.membership, u.roles)
	}
}

func TestLastAdministratorCannotBeRemoved(t *testing.T) {
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testPassword)
	ctx := context.Background()
	pid, _, _ := c.EnsureProvisioner(ctx)
	prov, _, _ := c.EnsureProvider(ctx, pid, "CLUBA", "Club A")
	// No permanent instance administrator in this test.
	uid, err := c.EnsureAccount(ctx, prov, Account{ExternalID: "1", Email: AccountEmail("only"), Role: RoleProviderAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveMembership(ctx, prov, uid); !errors.Is(err, ErrLastAdministrator) {
		t.Fatalf("want ErrLastAdministrator, got %v", err)
	}
	if f.users[AccountEmail("only")].membership[prov] != RoleProviderAdmin {
		t.Fatal("a refused removal must leave the membership in place")
	}
}

func TestHandoverForUnknownPersonFails(t *testing.T) {
	_, c, _, prov := setUp(t)
	if _, err := c.HandoverToken(context.Background(), prov, "999"); err == nil {
		t.Fatal("a hand-over token for an unknown person must fail")
	}
}

func TestErrorsNeverContainSecrets(t *testing.T) {
	f, srv := newFake(t)
	wrong := strings.Repeat("z", 48)
	c := newClient(t, srv.URL, wrong)
	_, _, err := c.EnsureProvisioner(context.Background())
	if err == nil || strings.Contains(err.Error(), wrong) {
		t.Fatalf("sign-in failure must not quote the password: %v", err)
	}

	good := newClient(t, srv.URL, testPassword)
	if _, _, err := good.EnsureProvisioner(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, err = good.HandoverToken(context.Background(), "provider-x", "999")
	if err == nil {
		t.Fatal("expected a failure")
	}
	f.mu.Lock()
	for token := range f.tokens {
		if strings.Contains(err.Error(), token) {
			t.Fatal("the error text contains a token")
		}
	}
	f.mu.Unlock()
	if strings.Contains(err.Error(), testPassword) {
		t.Fatal("the error text contains the password")
	}

	// A service that cannot be reached: the error names the address only.
	down := newClient(t, "http://127.0.0.1:1", testPassword)
	_, _, err = down.EnsureProvisioner(context.Background())
	if err == nil || strings.Contains(err.Error(), "Bearer") || strings.Contains(err.Error(), testPassword) {
		t.Fatalf("unreachable service error = %v", err)
	}
}

func TestInvalidRoleIsRefusedBeforeAnyCall(t *testing.T) {
	f, c, _, prov := setUp(t)
	before := len(f.calls)
	if _, err := c.EnsureAccount(context.Background(), prov, Account{ExternalID: "2", Email: AccountEmail("x"), Role: "superadmin"}); err == nil {
		t.Fatal("an unknown role must be refused")
	}
	if len(f.calls) != before {
		t.Fatal("no request should be sent for an invalid role")
	}
}

// removeAccount ends a person's tournament access at their only provider:
// the membership, then the global roles.
func removeAccount(ctx context.Context, c *Client, providerID, userID, email string) error {
	if err := c.RemoveMembership(ctx, providerID, userID); err != nil {
		return err
	}
	return c.ClearRoles(ctx, email)
}
