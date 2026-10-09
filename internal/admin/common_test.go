// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The fallback page is only ever rendered when a template failed to load. It
// must report a server error so broken templates show up in logs/monitoring
// instead of masquerading as a successful "coming soon" page.
func TestRenderFallbackHTMLIsServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	renderFallbackHTML(rec, "Teams", "Teams", "Teams page coming soon", "/admin/league/dashboard")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
