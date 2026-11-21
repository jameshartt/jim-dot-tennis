// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
)

func TestAuditLogPage(t *testing.T) {
	e := newUsersTestEnv(t, "")
	h := NewAuditLogHandler(NewService(e.env.db, "", e.env.holder, ""), findTemplatesPathAdmin(t))
	// Enough entries for two pages (two from creating the admin already).
	for i := 0; i < auditLogPageSize; i++ {
		if _, err := e.svc.SetRole(context.Background(), accounts.CLIActor("user role"), e.admin.ID, models.RoleAdmin); err != nil {
			t.Fatal(err)
		}
	}
	serve := func(method, target string, role models.Role) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, nil)
		ctx := config.WithBranding(req.Context(), e.env.holder.Branding())
		ctx = context.WithValue(ctx, auth.UserContextKey, models.User{ID: e.admin.ID, Username: e.admin.Username, Role: role})
		rec := httptest.NewRecorder()
		h.HandleAuditLog(rec, req.WithContext(ctx))
		return rec
	}

	rec := serve(http.MethodGet, auditLogPath, models.RoleAdmin)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if n := strings.Count(body, `data-testid="audit-entry"`); n != auditLogPageSize {
		t.Errorf("%d entries on page 1, want %d", n, auditLogPageSize)
	}
	if !strings.Contains(body, `data-testid="audit-log-older"`) || strings.Contains(body, `data-testid="audit-log-newer"`) {
		t.Error("page 1 should link to older entries only")
	}
	// Newest first: page 1 is all the later role changes; the account's
	// creation (the oldest entry) is on page 2.
	if strings.Count(body, `data-action="user.role_change"`) != auditLogPageSize || strings.Contains(body, `data-action="user.create"`) {
		t.Error("page 1 is not the newest entries")
	}
	if strings.Contains(body, "<form") {
		t.Error("the audit log page has a form: entries must not be editable")
	}

	page2 := serve(http.MethodGet, auditLogPath+"?page=2", models.RoleAdmin).Body.String()
	if !strings.Contains(page2, `data-action="user.create"`) || !strings.Contains(page2, `data-testid="audit-log-newer"`) {
		t.Errorf("page 2 lacks the oldest entry or the newer link:\n%s", page2)
	}
	if strings.Contains(page2, fmt.Sprintf(`?page=%d`, 3)) {
		t.Error("page 2 links to a page 3 that does not exist")
	}

	// Read-only and administrators only.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := serve(method, auditLogPath, models.RoleAdmin); rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", method, rec.Code)
		}
	}
	if rec := serve(http.MethodGet, auditLogPath, models.RoleCaptain); rec.Code != http.StatusForbidden {
		t.Errorf("captain: %d, want 403", rec.Code)
	}
}
