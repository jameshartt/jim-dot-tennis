// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// WI-134 F3: /api/push/subscribe is open to anyone, so what it stores is bounded.
func TestSubscribeStoresBoundedData(t *testing.T) {
	s := newTestService(t)
	mux := http.NewServeMux()
	s.SetupHandlers(mux, stubAdminGate)
	post := func(body, ua string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/push/subscribe", strings.NewReader(body))
		r.Header.Set("User-Agent", ua)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		return rec.Code
	}
	endpoint := "https://fcm.googleapis.com/fcm/send/abc"
	ok := `{"endpoint":"` + endpoint + `","playerToken":"A_B_C_D","keys":{"p256dh":"k","auth":"a"}}`
	if code := post(ok, strings.Repeat("U", 50000)); code != http.StatusCreated {
		t.Fatalf("ordinary subscription: status %d", code)
	}
	var ua int
	if err := s.db.Get(&ua, `SELECT MAX(LENGTH(user_agent)) FROM push_subscriptions`); err != nil || ua > maxUserAgentBytes {
		t.Errorf("stored user agent is %d bytes (%v), limit %d", ua, err, maxUserAgentBytes)
	}
	for name, body := range map[string]string{
		"huge body":   `{"endpoint":"` + endpoint + `","junk":"` + strings.Repeat("x", 1<<20) + `"}`,
		"long token":  `{"endpoint":"` + endpoint + `","playerToken":"` + strings.Repeat("t", 2000) + `","keys":{"p256dh":"k","auth":"a"}}`,
		"long key":    `{"endpoint":"` + endpoint + `","keys":{"p256dh":"` + strings.Repeat("k", 2000) + `","auth":"a"}}`,
		"long suffix": `{"endpoint":"` + endpoint + strings.Repeat("e", 4000) + `","keys":{"p256dh":"k","auth":"a"}}`,
	} {
		if code := post(body, "ua"); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, code)
		}
	}
	var n int
	if err := s.db.Get(&n, `SELECT COUNT(*) FROM push_subscriptions`); err != nil || n != 1 {
		t.Errorf("%d subscriptions stored (%v), want 1", n, err)
	}
}

// WI-134 F12: no part of the VAPID private key is logged or returned, and a
// player's token (their link) is never logged.
func TestNoKeyMaterialOrTokensInLogsOrResponses(t *testing.T) {
	s := newTestService(t)
	logs := captureLog(t)
	if _, _, err := s.GenerateVAPIDKeys(); err != nil {
		t.Fatal(err)
	}
	if err := s.ListVAPIDKeys(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.SetupHandlers(mux, stubAdminGate)
	r := httptest.NewRequest(http.MethodPost, "/api/vapid-reset", nil)
	r.Header.Set("X-Test-Admin", "1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("vapid reset: status %d", rec.Code)
	}
	var private string
	if err := s.db.Get(&private, `SELECT private_key FROM vapid_keys LIMIT 1`); err != nil || len(private) < 20 {
		t.Fatalf("private key: %q, %v", private, err)
	}
	if strings.Contains(rec.Body.String(), "privateKey") {
		t.Errorf("the reset response still has a privateKey field: %s", rec.Body.String())
	}
	if err := s.ListVAPIDKeys(); err != nil {
		t.Fatal(err)
	}

	const token = "Sabalenka_Djokovic_Gauff_Sinner"
	s.db.MustExec(`INSERT INTO push_subscriptions (endpoint, p256dh, auth, user_agent, player_token, created_at)
		VALUES ('https://fcm.googleapis.com/fcm/send/secret-endpoint-id', 'not-a-key', 'not-a-secret', 'ua', ?, CURRENT_TIMESTAMP)`, token)
	_, _ = s.SendToPlayer(token, map[string]interface{}{"title": "hi"})
	r = httptest.NewRequest(http.MethodPost, "/api/push/test-player", strings.NewReader(`{"playerToken":"`+token+`"}`))
	mux.ServeHTTP(httptest.NewRecorder(), r)

	out := logs.String()
	for what, secret := range map[string]string{
		"the start of the private key": private[:10],
		"the end of the private key":   private[len(private)-10:],
		"a player token":               token,
		"a push endpoint":              "secret-endpoint-id",
	} {
		if strings.Contains(out, secret) || strings.Contains(rec.Body.String(), secret) {
			t.Errorf("%s appears in the log or the response", what)
		}
	}
	if strings.Contains(out, "Private Key") {
		t.Error("a private key line is still logged")
	}
}
