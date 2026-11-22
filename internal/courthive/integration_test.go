// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

//go:build integration

package courthive

// Runs the client against a real tournament server and identity service.
// Needs COURTHIVE_IT_SERVER, COURTHIVE_IT_IDENTITY and COURTHIVE_IT_PASSWORD.
// With COURTHIVE_IT_TOKEN_DIR set it also leaves two fresh hand-over tokens
// there (files handover-1 and handover-2) for a browser test to use within a
// minute. Prints no secrets.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func post(t *testing.T, url, bearer string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func calendars(t *testing.T, server, token string) (int, int) {
	status, out := post(t, server+"/provider/my-calendars", token, map[string]any{})
	list, _ := out["calendars"].([]any)
	return status, len(list)
}

func TestAgainstRealServices(t *testing.T) {
	server, identity, password := os.Getenv("COURTHIVE_IT_SERVER"), os.Getenv("COURTHIVE_IT_IDENTITY"), os.Getenv("COURTHIVE_IT_PASSWORD")
	if server == "" || identity == "" || password == "" {
		t.Skip("COURTHIVE_IT_* not set")
	}
	ctx := context.Background()
	c, err := New(Config{ServerURL: server, IdentityURL: identity, ServicePassword: password})
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("engine version %s", v)

	pid, created, err := c.EnsureProvisioner(ctx)
	if err != nil {
		t.Fatalf("EnsureProvisioner: %v", err)
	}
	t.Logf("provisioner created=%v", created)
	if again, created2, err := c.EnsureProvisioner(ctx); err != nil || created2 || again != pid {
		t.Fatalf("repeat EnsureProvisioner: created=%v same=%v err=%v", created2, again == pid, err)
	}
	prov, created, err := c.EnsureProvider(ctx, pid, "CLUBA", "Club A Tennis")
	if err != nil {
		t.Fatalf("EnsureProvider: %v", err)
	}
	t.Logf("provider created=%v", created)
	if again, created2, err := c.EnsureProvider(ctx, pid, "CLUBA", "Club A Tennis"); err != nil || created2 || again != prov {
		t.Fatalf("repeat EnsureProvider: created=%v err=%v", created2, err)
	}
	if _, err := c.EnsureAccount(ctx, prov, Account{ExternalID: InstanceExternalID, Email: AccountEmail(InstanceExternalID), Role: RoleProviderAdmin}); err != nil {
		t.Fatalf("instance account: %v", err)
	}

	acct := Account{ExternalID: "it-7", Email: AccountEmail("it.organiser"), Role: RoleProviderAdmin}
	uid, err := c.EnsureAccount(ctx, prov, acct)
	if err != nil {
		t.Fatalf("EnsureAccount: %v", err)
	}
	if again, err := c.EnsureAccount(ctx, prov, acct); err != nil || again != uid {
		t.Fatalf("repeat EnsureAccount: same=%v err=%v", again == uid, err)
	}

	// Hand over, as the browser would.
	handover, err := c.HandoverToken(ctx, prov, acct.ExternalID)
	if err != nil {
		t.Fatalf("HandoverToken: %v", err)
	}
	status, session := post(t, server+"/auth/sso/login-with-token", "", map[string]string{"token": handover})
	access, _ := session["accessToken"].(string)
	refresh, _ := session["refreshToken"].(string)
	if status != 200 || access == "" || refresh == "" {
		t.Fatalf("exchange: status %d, access=%v refresh=%v error=%v", status, access != "", refresh != "", session["error"])
	}
	if _, replay := post(t, server+"/auth/sso/login-with-token", "", map[string]string{"token": handover}); replay["accessToken"] != nil {
		t.Fatal("a hand-over token was accepted twice")
	}
	if status, n := calendars(t, server, access); status != 200 || n != 1 {
		t.Fatalf("calendar with a fresh session: status %d, %d calendars", status, n)
	}
	// The session the server issued must be renewable at the identity service.
	status, renewed := post(t, identity+"/auth/refresh", "", map[string]string{"refreshToken": refresh})
	renewedAccess, _ := renewed["token"].(string)
	t.Logf("refresh at the identity service: status %d, token issued=%v", status, renewedAccess != "")
	if status != 200 || renewedAccess == "" {
		t.Fatalf("refresh failed: %v", renewed["message"])
	}
	if status, n := calendars(t, server, renewedAccess); status != 200 || n != 1 {
		t.Fatalf("calendar after refresh: status %d, %d calendars", status, n)
	}
	// Password sign-in is not possible for a person's account.
	if status, _ := post(t, identity+"/auth/login", "", map[string]string{"email": acct.Email, "password": ""}); status != 401 {
		t.Fatalf("password sign-in for a person's account: status %d, want 401", status)
	}

	// Remove: the existing session loses the club's tournaments at once.
	if err := c.RemoveAccount(ctx, prov, uid, acct.Email); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	status, n := calendars(t, server, renewedAccess)
	t.Logf("after removal, existing session: status %d, %d calendars", status, n)
	if status == 200 && n != 0 {
		t.Fatal("a removed person still sees the club's calendar")
	}
	if _, err := c.HandoverToken(ctx, prov, acct.ExternalID); err == nil {
		t.Fatal("a hand-over token was issued for a removed person")
	} else {
		t.Logf("hand-over for a removed person refused: %v", err)
	}
	if err := c.RemoveAccount(ctx, prov, uid, acct.Email); err != nil {
		t.Fatalf("second RemoveAccount: %v", err)
	}
	// Return, as a director this time.
	acct.Role, acct.UserID = RoleDirector, uid
	if _, err := c.EnsureAccount(ctx, prov, acct); err != nil {
		t.Fatalf("return: %v", err)
	}
	if _, err := c.EnsureAccount(ctx, prov, Account{ExternalID: acct.ExternalID, Email: acct.Email, Role: RoleProviderAdmin, UserID: uid}); err != nil {
		t.Fatalf("promotion: %v", err)
	}

	// The instance administrator is the last one standing if everyone else goes.
	members, err := c.Members(ctx, prov)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d members", len(members))
	for _, m := range members {
		if m.ExternalID == InstanceExternalID {
			if err := c.RemoveAccount(ctx, prov, m.UserID, m.Email); err == nil {
				// put it back: the test must leave the instance usable
				_, _ = c.EnsureAccount(ctx, prov, Account{ExternalID: InstanceExternalID, Email: m.Email, Role: RoleProviderAdmin, UserID: m.UserID})
			} else if !errors.Is(err, ErrLastAdministrator) {
				t.Logf("removing the instance administrator: %v", err)
			}
		}
	}

	if dir := os.Getenv("COURTHIVE_IT_TOKEN_DIR"); dir != "" {
		for _, name := range []string{"handover-1", "handover-2"} {
			token, err := c.HandoverToken(ctx, prov, acct.ExternalID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(token), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
