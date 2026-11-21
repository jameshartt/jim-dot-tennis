// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"net/http"
	"strconv"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/models"
)

// auditLogPath is the account audit log page (administrators only).
const auditLogPath = "/admin/league/audit-log"

// auditLogPageSize is how many entries one page shows.
const auditLogPageSize = 50

// AuditLogHandler serves the account audit log: read-only, newest first, in
// pages. Nothing on the page or behind any route changes or deletes an entry.
type AuditLogHandler struct {
	accounts    *accounts.Service
	templateDir string
}

// NewAuditLogHandler creates the audit log page handler.
func NewAuditLogHandler(service *Service, templateDir string) *AuditLogHandler {
	return &AuditLogHandler{accounts: service.accounts, templateDir: templateDir}
}

// auditLogData is what templates/admin/audit_log.html renders.
type auditLogData struct {
	User     *models.User
	Entries  []accounts.AuditEntry
	Total    int
	Page     int
	Pages    int
	PrevPage int // 0: none
	NextPage int // 0: none
}

// HandleAuditLog serves GET /admin/league/audit-log[?page=N].
func (h *AuditLogHandler) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != auditLogPath {
		http.NotFound(w, r)
		return
	}
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Administrators only, whatever the route group allows.
	if user.Role != models.RoleAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	entries, total, err := h.accounts.AuditPage(r.Context(), auditLogPageSize, (page-1)*auditLogPageSize)
	if err != nil {
		logAndError(w, "Failed to load the audit log", err, http.StatusInternalServerError)
		return
	}
	pages := (total + auditLogPageSize - 1) / auditLogPageSize
	if pages < 1 {
		pages = 1
	}
	data := auditLogData{User: user, Entries: entries, Total: total, Page: page, Pages: pages}
	if page > 1 {
		data.PrevPage = min(page-1, pages)
	}
	if page < pages {
		data.NextPage = page + 1
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/audit_log.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if err := renderTemplate(w, tmpl, data); err != nil {
		logAndError(w, "Failed to render template", err, http.StatusInternalServerError)
	}
}
