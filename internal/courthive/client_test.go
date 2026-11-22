// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package courthive

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef0123456789abcdef"

// fakeServer imitates the parts of the tournament server the client uses,
// including its habit of reporting some failures with HTTP 201. It verifies
// every token the way the real server does: signature, expiry, audience.
type fakeServer struct {
	t      *testing.T
	mu     sync.Mutex
	secret string

	provisioners map[string]string // name -> id
	providers    map[string]*Provider
	users        map[string]*fakeUser // email -> user
	calls        []string
	lastClaims   map[string]any
}

type fakeUser struct {
	id, email, externalID string
	roles                 []string
	membership            map[string]string // providerID -> role
}

func newFake(t *testing.T) (*fakeServer, *httptest.Server) {
	f := &fakeServer{t: t, secret: testSecret, provisioners: map[string]string{}, providers: map[string]*Provider{}, users: map[string]*fakeUser{}}
	srv := httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeServer) verify(r *http.Request) (map[string]any, bool) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	mac := hmac.New(sha256.New, []byte(f.secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return nil, false
	}
	var header map[string]string
	raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
	_ = json.Unmarshal(raw, &header)
	if header["alg"] != "HS256" {
		return nil, false
	}
	var claims map[string]any
	raw, _ = base64.RawURLEncoding.DecodeString(parts[1])
	if json.Unmarshal(raw, &claims) != nil {
		return nil, false
	}
	exp, _ := claims["exp"].(float64)
	if time.Now().Unix() >= int64(exp) || claims["aud"] != "admin" {
		return nil, false
	}
	return claims, true
}

func hasRole(claims map[string]any, role string) bool {
	roles, _ := claims["roles"].([]any)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
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
	claims, ok := f.verify(r)
	if !ok {
		reply(401, map[string]any{"message": "Unauthorized", "statusCode": 401})
		return
	}
	f.lastClaims = claims
	if claims["email"] != ServiceAccountEmail || !hasRole(claims, "superadmin") {
		reply(403, map[string]any{"message": "Forbidden resource", "error": "Forbidden", "statusCode": 403})
		return
	}
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	str := func(k string) string { s, _ := body[k].(string); return s }
	isProvisioner := hasRole(claims, "provisioner")
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
		list := []Provider{}
		for _, p := range f.providers {
			list = append(list, *p)
		}
		reply(200, map[string]any{"providers": list})
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

func newClient(t *testing.T, url, secret string) *Client {
	t.Helper()
	c, err := New(url, secret)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestNewRejectsWeakSecretAndAllowsNone(t *testing.T) {
	if _, err := New("http://courthive-server:8383", "short"); err == nil {
		t.Fatal("a short signing secret must be refused")
	}
	if _, err := New("", testSecret); err == nil {
		t.Fatal("an empty address must be refused")
	}
	c, err := New("http://courthive-server:8383", "")
	if err != nil {
		t.Fatalf("no secret is a valid, unconfigured client: %v", err)
	}
	if c.Configured() {
		t.Fatal("a client with no secret must not report itself configured")
	}
	if _, _, err := c.EnsureProvisioner(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
	if _, err := c.HandoverToken(context.Background(), "p", "q", "1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestTokenClaimsAreShortLivedAndScoped(t *testing.T) {
	c := newClient(t, "http://x", testSecret)
	at := time.Unix(1_800_000_000, 0)
	c.now = func() time.Time { return at }

	decode := func(token string) map[string]any {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("token has %d parts", len(parts))
		}
		raw, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims map[string]any
		if err := json.Unmarshal(raw, &claims); err != nil {
			t.Fatal(err)
		}
		return claims
	}

	admin, err := c.sign("")
	if err != nil {
		t.Fatal(err)
	}
	claims := decode(admin)
	if claims["email"] != ServiceAccountEmail || claims["aud"] != "admin" {
		t.Fatalf("unexpected identity claims: %v", claims)
	}
	if got := int64(claims["exp"].(float64)) - int64(claims["iat"].(float64)); got != 60 {
		t.Fatalf("token lifetime is %d seconds, want 60", got)
	}
	if _, has := claims["provisionerIds"]; has {
		t.Fatal("an administration token must not carry the provisioner claim")
	}
	// Nothing about a jim-dot-tennis user or session belongs in these tokens.
	for k := range claims {
		switch k {
		case "email", "roles", "aud", "iat", "exp":
		default:
			t.Fatalf("unexpected claim %q", k)
		}
	}

	prov, err := c.sign("prov-1")
	if err != nil {
		t.Fatal(err)
	}
	claims = decode(prov)
	ids, _ := claims["provisionerIds"].([]any)
	if len(ids) != 1 || ids[0] != "prov-1" || !hasRole(claims, "provisioner") {
		t.Fatalf("provisioner token claims wrong: %v", claims)
	}
}

func TestWrongSecretIsRefusedByTheServer(t *testing.T) {
	_, srv := newFake(t)
	other := newClient(t, srv.URL, strings.Repeat("z", 48)) // another instance's secret
	_, _, err := other.EnsureProvisioner(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("want a 401 from the server, got %v", err)
	}
}

func TestExpiredTokenIsRefusedByTheServer(t *testing.T) {
	_, srv := newFake(t)
	c := newClient(t, srv.URL, testSecret)
	c.now = func() time.Time { return time.Now().Add(-2 * time.Minute) }
	_, _, err := c.EnsureProvisioner(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("want a 401 for an expired token, got %v", err)
	}
}

func TestSetUpIsRepeatable(t *testing.T) {
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testSecret)
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
	c := newClient(t, srv.URL, testSecret)
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
	c := newClient(t, srv.URL, testSecret)
	ctx := context.Background()
	pid, _, err := c.EnsureProvisioner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prov, _, err := c.EnsureProvider(ctx, pid, "CLUBA", "Club A")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.EnsureAccount(ctx, pid, prov, Account{ExternalID: InstanceExternalID, Email: AccountEmail(InstanceExternalID), Role: RoleProviderAdmin}); err != nil {
		t.Fatal(err)
	}
	return f, c, pid, prov
}

func TestAccountLifecycle(t *testing.T) {
	f, c, pid, prov := setUp(t)
	ctx := context.Background()
	acct := Account{ExternalID: "7", Email: AccountEmail("Jo.Bloggs"), Role: RoleProviderAdmin}
	if acct.Email != "jo.bloggs@jim-dot-tennis.invalid" {
		t.Fatalf("AccountEmail = %q", acct.Email)
	}

	uid, err := c.EnsureAccount(ctx, pid, prov, acct)
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
	uid2, err := c.EnsureAccount(ctx, pid, prov, acct)
	if err != nil || uid2 != uid {
		t.Fatalf("repeat EnsureAccount = %q, %v", uid2, err)
	}

	// Demote to director.
	acct.Role, acct.UserID = RoleDirector, uid
	if _, err := c.EnsureAccount(ctx, pid, prov, acct); err != nil {
		t.Fatal(err)
	}
	if u.membership[prov] != RoleDirector {
		t.Fatalf("role after demotion = %q", u.membership[prov])
	}

	token, err := c.HandoverToken(ctx, pid, prov, "7")
	if err != nil || token == "" {
		t.Fatalf("HandoverToken = %q, %v", token, err)
	}

	// Remove: membership and roles both go.
	if err := c.RemoveAccount(ctx, prov, uid, acct.Email); err != nil {
		t.Fatal(err)
	}
	if _, member := u.membership[prov]; member || len(u.roles) != 0 {
		t.Fatalf("after removal: membership %v roles %v", u.membership, u.roles)
	}
	// Removing again is not an error.
	if err := c.RemoveAccount(ctx, prov, uid, acct.Email); err != nil {
		t.Fatalf("second removal: %v", err)
	}

	// Coming back without the stored id cannot be resolved; with it, it can.
	if _, err := c.EnsureAccount(ctx, pid, prov, Account{ExternalID: "7", Email: acct.Email, Role: RoleDirector}); !errors.Is(err, ErrAccountUnknown) {
		t.Fatalf("want ErrAccountUnknown, got %v", err)
	}
	if _, err := c.EnsureAccount(ctx, pid, prov, Account{ExternalID: "7", Email: acct.Email, Role: RoleProviderAdmin, UserID: uid}); err != nil {
		t.Fatal(err)
	}
	if u.membership[prov] != RoleProviderAdmin || len(u.roles) != 3 {
		t.Fatalf("after return: membership %v roles %v", u.membership, u.roles)
	}
}

func TestLastAdministratorCannotBeRemoved(t *testing.T) {
	f, srv := newFake(t)
	c := newClient(t, srv.URL, testSecret)
	ctx := context.Background()
	pid, _, _ := c.EnsureProvisioner(ctx)
	prov, _, _ := c.EnsureProvider(ctx, pid, "CLUBA", "Club A")
	// No permanent instance administrator in this test.
	uid, err := c.EnsureAccount(ctx, pid, prov, Account{ExternalID: "1", Email: AccountEmail("only"), Role: RoleProviderAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveAccount(ctx, prov, uid, AccountEmail("only")); !errors.Is(err, ErrLastAdministrator) {
		t.Fatalf("want ErrLastAdministrator, got %v", err)
	}
	if f.users[AccountEmail("only")].membership[prov] != RoleProviderAdmin {
		t.Fatal("a refused removal must leave the membership in place")
	}
}

func TestHandoverForUnknownPersonFails(t *testing.T) {
	_, c, pid, prov := setUp(t)
	if _, err := c.HandoverToken(context.Background(), pid, prov, "999"); err == nil {
		t.Fatal("a hand-over token for an unknown person must fail")
	}
}

func TestErrorsNeverContainTheToken(t *testing.T) {
	_, srv := newFake(t)
	c := newClient(t, srv.URL, strings.Repeat("z", 48))
	_, _, err := c.EnsureProvisioner(context.Background())
	if err == nil {
		t.Fatal("expected a failure")
	}
	token, _ := c.sign("")
	for _, part := range strings.Split(token, ".") {
		if strings.Contains(err.Error(), part) {
			t.Fatal("the error text contains token material")
		}
	}
	if strings.Contains(err.Error(), strings.Repeat("z", 48)) {
		t.Fatal("the error text contains the secret")
	}

	// A server that cannot be reached: the error names the address, not the header.
	down := newClient(t, "http://127.0.0.1:1", testSecret)
	_, _, err = down.EnsureProvisioner(context.Background())
	if err == nil || strings.Contains(err.Error(), "Bearer") {
		t.Fatalf("unreachable server error = %v", err)
	}
}

func TestInvalidRoleIsRefusedBeforeAnyCall(t *testing.T) {
	f, c, pid, prov := setUp(t)
	before := len(f.calls)
	if _, err := c.EnsureAccount(context.Background(), pid, prov, Account{ExternalID: "2", Email: AccountEmail("x"), Role: "superadmin"}); err == nil {
		t.Fatal("an unknown role must be refused")
	}
	if len(f.calls) != before {
		t.Fatal("no request should be sent for an invalid role")
	}
}
