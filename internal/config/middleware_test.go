// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jim-dot-tennis/internal/repository"
)

func TestHomeClubMiddlewareReadsLiveHolder(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv(t)
	h, err := NewHomeClubHolder(ctx, e.clubs, e.settings)
	if err != nil {
		t.Fatal(err)
	}

	var gotID uint
	handler := HomeClubMiddleware(h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = GetHomeClubID(r.Context())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if gotID != 0 {
		t.Fatalf("before setup: home club ID = %d, want 0", gotID)
	}

	e.addClub(t, 9, "Later Club")
	if err := e.settings.SetUint(ctx, repository.SettingHomeClubID, 9); err != nil {
		t.Fatal(err)
	}
	// No Refresh: an unresolved holder retries on the next request.
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if gotID != 9 {
		t.Fatalf("after storing the setting: home club ID = %d, want 9", gotID)
	}
}
