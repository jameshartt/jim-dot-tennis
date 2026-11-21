// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package players

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/models"
)

func TestMyTennisAndWrappedReturn404WhenOff(t *testing.T) {
	// A service without club settings reports the defaults: My Tennis off.
	svc := &Service{}
	ctx := context.WithValue(context.Background(), auth.PlayerContextKey, models.Player{ID: "p1"})

	req := httptest.NewRequest(http.MethodGet, "/my-profile/Some_Token", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	NewProfileHandler(svc, "").HandleProfile(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("My Tennis off: status %d, want 404", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/my-profile/Some_Token", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	NewProfileHandler(svc, "").HandleProfile(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("My Tennis off, POST: status %d, want 404", rec.Code)
	}
}
