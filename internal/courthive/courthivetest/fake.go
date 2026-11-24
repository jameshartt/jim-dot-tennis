// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package courthivetest is a stand-in for the bundled tournament stack (the
// tournament server and the identity service on one address), for tests of
// code that uses internal/courthive from other packages. It imitates the
// calls the client makes, including reporting some failures with a success
// status, and lets a test make any one call fail. Only tests import it.
package courthivetest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Password is the service password the fake accepts unless a test sets
// another.
const Password = "fake-service-password-0123456789abcdef0123456789"

// Calls a test can make fail (Server.FailNext).
const (
	CallVersion          = "version"
	CallSignIn           = "sign-in"
	CallListProvisioners = "list-provisioners"
	CallMakeProvisioner  = "make-provisioner"
	CallRepresentatives  = "representatives"
	CallAssign           = "assign-representative"
	CallTakeOwnership    = "take-ownership"
	CallListProviders    = "list-providers"
	CallListAllProviders = "list-all-providers"
	CallMakeProvider     = "make-provider"
	CallMembers          = "members"
	CallMakeAccount      = "make-account"
	CallSetRole          = "set-role"
	CallSetGlobalRoles   = "set-global-roles"
	CallRemove           = "remove-membership"
	CallHandover         = "hand-over-token"
	CallCalendar         = "calendar"
)

// Provider is a provider held by the fake.
type Provider struct {
	ID, Abbreviation, Name string
	Managed                bool
}

// User is a tournament account held by the fake.
type User struct {
	ID, Email, ExternalID, SSOProvider string
	Roles                              []string
	Membership                         map[string]string // provider id -> role
}

// Server is the fake tournament stack.
type Server struct {
	*httptest.Server
	Password string // the service password it accepts

	mu           sync.Mutex
	logins       int
	tokens       map[string]bool // token -> issued after the service account represented a provisioner
	provisioners map[string]string
	reps         map[string]bool
	providers    map[string]*Provider // by abbreviation
	users        []*User
	calendars    map[string][]CalendarEntry // by abbreviation
	fail         map[string]int             // call -> status to answer with, once
	calls        map[string]int
	handovers    int
}

// New starts a fake, closed when the test ends.
func New(t testing.TB) *Server {
	s := &Server{Password: Password, tokens: map[string]bool{}, provisioners: map[string]string{}, reps: map[string]bool{},
		providers: map[string]*Provider{}, fail: map[string]int{}, calls: map[string]int{}}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

// FailNext makes the next request of the given call answer with status.
func (s *Server) FailNext(call string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail[call] = status
}

// Calls is how many times the call was made.
func (s *Server) Calls(call string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[call]
}

// AddProvider puts a provider on the server, as if made before the app (by
// hand); managed says whether the app's provisioner already owns it.
func (s *Server) AddProvider(abbreviation, name string, managed bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "provider-" + abbreviation
	s.providers[abbreviation] = &Provider{ID: id, Abbreviation: abbreviation, Name: name, Managed: managed}
	return id
}

// CalendarEntry is a tournament in a provider's public calendar.
type CalendarEntry struct {
	ID, Name, StartDate, EndDate string
}

// SetCalendar sets the tournaments the provider's public calendar lists.
func (s *Server) SetCalendar(abbreviation string, entries ...CalendarEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calendars == nil {
		s.calendars = map[string][]CalendarEntry{}
	}
	s.calendars[abbreviation] = entries
}

// AddAccount puts a tournament account in a provider, as if made in
// CourtHive itself (ssoProvider "") or by another system.
func (s *Server) AddAccount(providerID, email, role, ssoProvider, externalID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users = append(s.users, &User{ID: "user-" + email, Email: email, ExternalID: externalID, SSOProvider: ssoProvider,
		Membership: map[string]string{providerID: role}})
}

// RemoveMembership takes the account out of a provider, as if done by hand
// on the tournament server (no last-administrator check).
func (s *Server) RemoveMembership(userID, providerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.ID == userID {
			delete(u.Membership, providerID)
		}
	}
}

// Providers returns a copy of the providers.
func (s *Server) Providers() []Provider {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Provider
	for _, p := range s.providers {
		out = append(out, *p)
	}
	return out
}

// Provisioners returns how many provisioners exist.
func (s *Server) Provisioners() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.provisioners)
}

// Users returns a copy of the accounts.
func (s *Server) Users() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []User
	for _, u := range s.users {
		c := *u
		c.Membership = map[string]string{}
		for k, v := range u.Membership {
			c.Membership[k] = v
		}
		c.Roles = append([]string(nil), u.Roles...)
		out = append(out, c)
	}
	return out
}

// Tokens returns every token the fake issued, so a test can check none of
// them appears in output.
func (s *Server) Tokens() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for t := range s.tokens {
		out = append(out, t)
	}
	return out
}

func (s *Server) issue() string {
	s.logins++
	enc := base64.RawURLEncoding
	payload, _ := json.Marshal(map[string]any{"email": "service", "exp": time.Now().Add(4 * time.Hour).Unix(), "n": s.logins})
	token := enc.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." + enc.EncodeToString(payload) + "." + enc.EncodeToString([]byte(fmt.Sprintf("sig%d", s.logins)))
	s.tokens[token] = len(s.reps) > 0
	return token
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	// refuse answers as the tournament server does when it refuses a request.
	refuse := func(status int, message string) {
		reply(status, map[string]any{"message": message, "statusCode": status})
	}
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	str := func(k string) string { v, _ := body[k].(string); return v }
	path := r.URL.Path
	call := s.classify(r.Method, path)
	s.calls[call]++
	if status, ok := s.fail[call]; ok {
		delete(s.fail, call)
		refuse(status, http.StatusText(status))
		return
	}

	switch call {
	case CallVersion:
		reply(200, map[string]string{"version": "7.1.1"})
		return
	case CallCalendar:
		// Public: the club's calendar (empty unless a test sets it).
		if _, ok := s.providers[str("providerAbbr")]; !ok {
			reply(200, map[string]any{"success": false, "error": "provider not found"})
			return
		}
		list := []any{}
		for _, e := range s.calendars[str("providerAbbr")] {
			list = append(list, map[string]any{"tournamentId": e.ID, "tournament": map[string]any{
				"tournamentName": e.Name, "startDate": e.StartDate, "endDate": e.EndDate}})
		}
		reply(200, map[string]any{"success": true, "calendar": map[string]any{"tournaments": list}})
		return
	case CallSignIn:
		if str("password") != s.Password {
			refuse(401, "Unauthorized")
			return
		}
		reply(200, map[string]any{"token": s.issue()})
		return
	}
	isProvisioner, ok := s.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	if !ok {
		refuse(401, "Unauthorized")
		return
	}
	if strings.HasPrefix(path, "/provisioner/") && !isProvisioner {
		refuse(401, "Provisioner API key required")
		return
	}

	switch call {
	case CallListProvisioners:
		list := []map[string]any{}
		for name, id := range s.provisioners {
			list = append(list, map[string]any{"provisionerId": id, "name": name, "isActive": true})
		}
		reply(200, map[string]any{"provisioners": list})
	case CallMakeProvisioner:
		id := "prov-" + str("name")
		s.provisioners[str("name")] = id
		reply(201, map[string]any{"provisioner": map[string]any{"provisionerId": id}})
	case CallRepresentatives:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/admin/provisioners/"), "/users")
		list := []map[string]any{}
		if s.reps[id] {
			list = append(list, map[string]any{"email": "service@jim-dot-tennis.invalid"})
		}
		reply(200, map[string]any{"users": list})
	case CallAssign:
		s.reps[strings.TrimSuffix(strings.TrimPrefix(path, "/admin/provisioners/"), "/users")] = true
		reply(201, map[string]any{"success": true})
	case CallTakeOwnership:
		for _, p := range s.providers {
			if p.ID == str("providerId") {
				p.Managed = true
			}
		}
		reply(201, map[string]any{"success": true})
	case CallListProviders:
		// As the real server: only the providers the provisioner manages.
		list := []map[string]any{}
		for _, p := range s.providers {
			if p.Managed {
				list = append(list, map[string]any{"providerId": p.ID, "organisationAbbreviation": p.Abbreviation, "organisationName": p.Name, "managed": true})
			}
		}
		reply(200, map[string]any{"providers": list})
	case CallListAllProviders:
		list := []map[string]any{}
		for _, p := range s.providers {
			list = append(list, map[string]any{"key": p.ID, "value": map[string]any{
				"organisationId": p.ID, "organisationAbbreviation": p.Abbreviation, "organisationName": p.Name}})
		}
		reply(200, map[string]any{"success": true, "providers": list})
	case CallMakeProvider:
		abbr := str("organisationAbbreviation")
		if _, exists := s.providers[abbr]; exists {
			reply(201, map[string]any{"error": "organisationAbbreviation already exists", "code": "ABBREVIATION_EXISTS"})
			return
		}
		s.providers[abbr] = &Provider{ID: "provider-" + abbr, Abbreviation: abbr, Name: str("organisationName"), Managed: true}
		reply(201, map[string]any{"providerId": "provider-" + abbr})
	case CallMembers:
		pid := r.URL.Query().Get("providerId")
		list := []map[string]any{}
		for _, u := range s.users {
			if role, ok := u.Membership[pid]; ok {
				list = append(list, map[string]any{"userId": u.ID, "email": u.Email, "providerRole": role, "externalId": u.ExternalID, "ssoProvider": u.SSOProvider})
			}
		}
		reply(200, map[string]any{"users": list})
	case CallMakeAccount:
		for _, u := range s.users {
			if u.SSOProvider == str("ssoProvider") && u.ExternalID == str("externalId") {
				reply(201, map[string]any{"error": "SSO identity already exists", "code": "SSO_IDENTITY_EXISTS"})
				return
			}
			if u.Email == str("email") {
				reply(201, map[string]any{"error": "Email already registered", "code": "EMAIL_EXISTS"})
				return
			}
		}
		u := &User{ID: "user-" + str("externalId"), Email: str("email"), ExternalID: str("externalId"), SSOProvider: str("ssoProvider"),
			Membership: map[string]string{str("providerId"): str("providerRole")}}
		s.users = append(s.users, u)
		reply(201, map[string]any{"userId": u.ID})
	case CallSetRole:
		seg := strings.Split(strings.TrimPrefix(path, "/provisioner/users/"), "/providers/")
		for _, u := range s.users {
			if u.ID == seg[0] {
				u.Membership[seg[1]] = str("providerRole")
				reply(200, map[string]any{"userId": u.ID})
				return
			}
		}
		refuse(404, "Not found")
	case CallRemove:
		seg := strings.Split(strings.TrimPrefix(path, "/provisioner/users/"), "/providers/")
		for _, u := range s.users {
			if u.ID == seg[0] {
				admins := 0
				for _, other := range s.users {
					if other.Membership[seg[1]] == "PROVIDER_ADMIN" {
						admins++
					}
				}
				if u.Membership[seg[1]] == "PROVIDER_ADMIN" && admins <= 1 {
					refuse(409, "Provider must retain at least one PROVIDER_ADMIN")
					return
				}
				delete(u.Membership, seg[1])
				reply(200, map[string]any{"success": true})
				return
			}
		}
		reply(200, map[string]any{"success": true})
	case CallHandover:
		for _, u := range s.users {
			if u.SSOProvider == str("ssoProvider") && u.ExternalID == str("externalId") {
				if _, member := u.Membership[str("providerId")]; member {
					s.handovers++
					reply(200, map[string]any{"token": "handover-" + u.ExternalID + "-" + strconv.Itoa(s.handovers), "expiresIn": 60})
					return
				}
			}
		}
		reply(200, map[string]any{"error": "SSO identity not found for a provider this provisioner manages", "code": "SSO_IDENTITY_NOT_FOUND"})
	case CallSetGlobalRoles:
		for _, u := range s.users {
			if u.Email == str("email") {
				u.Roles = nil
				for _, role := range body["roles"].([]any) {
					u.Roles = append(u.Roles, role.(string))
				}
				reply(200, map[string]any{"success": true})
				return
			}
		}
		reply(200, map[string]any{"error": "User not found"})
	default:
		refuse(404, "Cannot "+r.Method+" "+path)
	}
}

func (s *Server) classify(method, path string) string {
	switch {
	case path == "/factory/version":
		return CallVersion
	case path == "/auth/login":
		return CallSignIn
	case path == "/provider/calendar":
		return CallCalendar
	case path == "/admin/provisioners" && method == http.MethodGet:
		return CallListProvisioners
	case path == "/admin/provisioners":
		return CallMakeProvisioner
	case strings.HasPrefix(path, "/admin/provisioners/") && strings.HasSuffix(path, "/users") && method == http.MethodGet:
		return CallRepresentatives
	case strings.HasPrefix(path, "/admin/provisioners/") && strings.HasSuffix(path, "/users"):
		return CallAssign
	case strings.HasPrefix(path, "/admin/provisioners/") && strings.HasSuffix(path, "/providers"):
		return CallTakeOwnership
	case path == "/provisioner/providers" && method == http.MethodGet:
		return CallListProviders
	case path == "/provider/allproviders":
		return CallListAllProviders
	case path == "/provisioner/providers":
		return CallMakeProvider
	case path == "/provisioner/users" && method == http.MethodGet:
		return CallMembers
	case path == "/provisioner/users":
		return CallMakeAccount
	case strings.HasPrefix(path, "/provisioner/users/") && strings.Contains(path, "/providers/") && method == http.MethodDelete:
		return CallRemove
	case strings.HasPrefix(path, "/provisioner/users/") && strings.Contains(path, "/providers/"):
		return CallSetRole
	case path == "/auth/sso/generate":
		return CallHandover
	case path == "/auth/modify":
		return CallSetGlobalRoles
	}
	return "other"
}
