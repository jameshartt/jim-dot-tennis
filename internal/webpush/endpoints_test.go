// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"net/http"
	"testing"
)

func TestAllowedPushEndpoint(t *testing.T) {
	ok := []string{
		"https://fcm.googleapis.com/fcm/send/abc:def",
		"https://android.googleapis.com/gcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://web.push.apple.com/QK2mv-abc",
		"https://wns2-par02p.notify.windows.com/w/?token=abc",
		"https://FCM.googleapis.com/fcm/send/x",
	}
	for _, e := range ok {
		if !allowedPushEndpoint(e) {
			t.Errorf("allowedPushEndpoint(%q) = false, want true", e)
		}
	}
	bad := []string{
		"",
		"http://fcm.googleapis.com/fcm/send/abc",
		"https://fcm.googleapis.com:8443/fcm/send/abc",
		"https://user@fcm.googleapis.com/fcm/send/abc",
		"https://evil.example/fcm.googleapis.com",
		"https://fcm.googleapis.com.evil.example/x",
		"https://evilpush.apple.com/x",
		"https://courthive-server:8383/admin",
		"https://127.0.0.1/x",
		"https://jim.tennis/collect",
		"javascript:alert(1)",
	}
	for _, e := range bad {
		if allowedPushEndpoint(e) {
			t.Errorf("allowedPushEndpoint(%q) = true, want false", e)
		}
	}
}

func TestSubscribeRejectsUnknownPushService(t *testing.T) {
	s, mux := newTestMux(t)
	body := `{"endpoint":"https://collector.example/x","keys":{"p256dh":"k","auth":"a"}}`
	if rec := serve(mux, http.MethodPost, "/api/push/subscribe", "", body, false); rec.Code != http.StatusBadRequest {
		t.Fatalf("subscribe to unknown host: status %d, want 400", rec.Code)
	}
	subs, err := s.GetAllSubscriptions()
	if err != nil || len(subs) != 0 {
		t.Fatalf("subscriptions after rejected subscribe = %d (err %v), want 0", len(subs), err)
	}

	body = `{"endpoint":"https://fcm.googleapis.com/fcm/send/abc","keys":{"p256dh":"k","auth":"a"}}`
	if rec := serve(mux, http.MethodPost, "/api/push/subscribe", "", body, false); rec.Code != http.StatusCreated {
		t.Fatalf("subscribe to FCM: status %d, want 201", rec.Code)
	}
}
