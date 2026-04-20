// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.
//
// Captain notes are an ADMIN-SESSION-ONLY surface.
//
// This handler is registered under /admin/league/* which is wrapped by
// authMiddleware.RequireAuth + RequireRole("admin") in handler.go. It MUST
// NEVER be mounted on any token-authenticated route (/my-availability/{token},
// /my-profile/{token}, etc.) — the stored notes contain sensitive planning
// context ('no-nos') that players must not read back about themselves.
// Sprint 017 WI-107 adds an E2E regression proving this.

package admin

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/models"
)

// CaptainNotesHandler owns CRUD + rendering for captain_player_notes.
type CaptainNotesHandler struct {
	service     *Service
	templateDir string
}

// NewCaptainNotesHandler constructs the handler.
func NewCaptainNotesHandler(service *Service, templateDir string) *CaptainNotesHandler {
	return &CaptainNotesHandler{service: service, templateDir: templateDir}
}

// HandleNotes routes /admin/league/captain-notes/... based on verb + suffix.
// Shape:
//
//	GET  /admin/league/captain-notes?player_id=pid  → popover list + add form
//	POST /admin/league/captain-notes                 → create
//	POST /admin/league/captain-notes/{id}            → update (if kind/body in form)
//	POST /admin/league/captain-notes/{id}/delete     → delete
func (h *CaptainNotesHandler) HandleNotes(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromContext(r)
	if err != nil {
		logAndError(w, "Unauthorized", err, http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/league/captain-notes")
	path = strings.TrimSuffix(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "":
		h.listForPlayer(w, r)
	case r.Method == http.MethodPost && path == "":
		h.create(w, r, user)
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/delete"):
		h.delete(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/delete"))
	case r.Method == http.MethodPost && path != "":
		h.update(w, r, strings.TrimPrefix(path, "/"))
	default:
		http.NotFound(w, r)
	}
}

func (h *CaptainNotesHandler) listForPlayer(w http.ResponseWriter, r *http.Request) {
	playerID := r.URL.Query().Get("player_id")
	if playerID == "" {
		http.Error(w, "player_id required", http.StatusBadRequest)
		return
	}
	h.renderPopover(w, r, playerID)
}

func (h *CaptainNotesHandler) create(w http.ResponseWriter, r *http.Request, user *models.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	playerID := r.FormValue("player_id")
	kind := models.CaptainNoteKind(r.FormValue("kind"))
	body := strings.TrimSpace(r.FormValue("body"))
	if playerID == "" || body == "" {
		http.Error(w, "player_id and body required", http.StatusBadRequest)
		return
	}
	if kind != models.CaptainNoteKindPartnership && kind != models.CaptainNoteKindGeneral {
		kind = models.CaptainNoteKindGeneral
	}
	note := &models.CaptainPlayerNote{
		PlayerID:     playerID,
		AuthorUserID: user.ID,
		Kind:         kind,
		Body:         body,
	}
	if err := h.service.captainNoteRepository.Create(r.Context(), note); err != nil {
		logAndError(w, "create captain note failed", err, http.StatusInternalServerError)
		return
	}
	h.respondAfterMutation(w, r, playerID)
}

func (h *CaptainNotesHandler) update(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := strconv.ParseUint(rawID, 10, 32)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	note, err := h.service.captainNoteRepository.FindByID(r.Context(), uint(id))
	if err != nil || note == nil {
		http.NotFound(w, r)
		return
	}
	note.Body = strings.TrimSpace(r.FormValue("body"))
	if k := r.FormValue("kind"); k != "" {
		kind := models.CaptainNoteKind(k)
		if kind == models.CaptainNoteKindPartnership || kind == models.CaptainNoteKindGeneral {
			note.Kind = kind
		}
	}
	if err := h.service.captainNoteRepository.Update(r.Context(), note); err != nil {
		logAndError(w, "update captain note failed", err, http.StatusInternalServerError)
		return
	}
	h.respondAfterMutation(w, r, note.PlayerID)
}

func (h *CaptainNotesHandler) delete(w http.ResponseWriter, r *http.Request, rawID string) {
	id, err := strconv.ParseUint(rawID, 10, 32)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	note, err := h.service.captainNoteRepository.FindByID(r.Context(), uint(id))
	if err != nil || note == nil {
		http.NotFound(w, r)
		return
	}
	playerID := note.PlayerID
	if err := h.service.captainNoteRepository.Delete(r.Context(), uint(id)); err != nil {
		logAndError(w, "delete captain note failed", err, http.StatusInternalServerError)
		return
	}
	h.respondAfterMutation(w, r, playerID)
}

// respondAfterMutation returns the popover partial for HTMX callers and
// redirects non-HTMX callers back to the referring page (player edit form).
func (h *CaptainNotesHandler) respondAfterMutation(w http.ResponseWriter, r *http.Request, playerID string) {
	if r.Header.Get("HX-Request") == "true" {
		h.renderPopover(w, r, playerID)
		return
	}
	dest := r.Referer()
	if dest == "" {
		dest = "/admin/league/players/" + playerID
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

// CaptainNotePopoverView is the template data for the popover partial.
type CaptainNotePopoverView struct {
	Player *models.Player
	Notes  []captainNoteRow
}

type captainNoteRow struct {
	Note       models.CaptainPlayerNote
	AuthorName string
}

func (h *CaptainNotesHandler) renderPopover(w http.ResponseWriter, r *http.Request, playerID string) {
	player, err := h.service.playerRepository.FindByID(r.Context(), playerID)
	if err != nil || player == nil {
		http.NotFound(w, r)
		return
	}
	notes, err := h.service.captainNoteRepository.ListByPlayer(r.Context(), playerID)
	if err != nil {
		logAndError(w, "list captain notes failed", err, http.StatusInternalServerError)
		return
	}

	rows := make([]captainNoteRow, 0, len(notes))
	for _, n := range notes {
		rows = append(rows, captainNoteRow{Note: n, AuthorName: h.authorDisplay(r, n.AuthorUserID)})
	}

	tmpl, err := parseTemplate(h.templateDir, "admin/partials/captain_notes_popover.html")
	if err != nil {
		logAndError(w, "template parse failed", err, http.StatusInternalServerError)
		return
	}
	if err := renderTemplate(w, tmpl, CaptainNotePopoverView{Player: player, Notes: rows}); err != nil {
		log.Printf("captain notes popover render failed: %v", err)
	}
}

// authorDisplay resolves users.id → a short display string for the UI. We
// don't have a user-name query on the service surface, so fall back to the
// numeric id when we can't resolve one.
func (h *CaptainNotesHandler) authorDisplay(r *http.Request, userID int64) string {
	var username string
	row := h.service.db.QueryRowxContext(r.Context(), `SELECT username FROM users WHERE id = ?`, userID)
	if err := row.Scan(&username); err == nil && username != "" {
		return username
	}
	return "admin #" + strconv.FormatInt(userID, 10)
}
