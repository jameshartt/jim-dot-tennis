// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func TestFeatureSwitchesSaveAndGateRoutes(t *testing.T) {
	e := newSettingsTestEnv(t)
	t.Setenv("MY_TENNIS_ENABLED", "")
	e = newSettingsTestEnvOn(t, e.db)
	h := &Handler{service: e.handler.service}
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	status := func(gate func(http.HandlerFunc) http.HandlerFunc) int {
		rec := httptest.NewRecorder()
		gate(ok)(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		return rec.Code
	}

	// Existing instance: tournaments and wrapped on.
	if status(h.tournamentsOnly) != http.StatusOK || status(h.wrappedOnly) != http.StatusOK {
		t.Fatal("tournaments and wrapped should default to on")
	}

	fields := validSettingsFields()
	fields["feature_tournaments"] = "off"
	fields["feature_club_wrapped"] = "off"
	fields["feature_my_tennis"] = "on"
	if rec := e.do(settingsForm(t, fields), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	if status(h.tournamentsOnly) != http.StatusNotFound || status(h.wrappedOnly) != http.StatusNotFound {
		t.Error("switched-off routes should return 404 without a restart")
	}
	if !e.holder.ClubSettings().MyTennisEnabled() {
		t.Error("My Tennis should now be on")
	}

	// The page shows the saved state.
	body := e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin).Body.String()
	if !strings.Contains(body, `id="feature_my_tennis" name="feature_my_tennis" value="on" checked`) ||
		strings.Contains(body, `id="feature_tournaments" name="feature_tournaments" value="on" checked`) {
		t.Errorf("feature checkboxes do not reflect the stored state")
	}

	// A form that omits the switches leaves them alone.
	if rec := e.do(settingsForm(t, validSettingsFields()), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	if status(h.tournamentsOnly) != http.StatusNotFound {
		t.Error("an omitted switch was reset")
	}
}

func TestMyTennisSwitchOverriddenByEnv(t *testing.T) {
	e := newSettingsTestEnv(t)
	t.Setenv("MY_TENNIS_ENABLED", "1")
	e = newSettingsTestEnvOn(t, e.db)

	body := e.do(httptest.NewRequest(http.MethodGet, settingsPath, nil), models.RoleAdmin).Body.String()
	if !strings.Contains(body, "<code>MY_TENNIS_ENABLED</code>") {
		t.Error("page does not show the env override")
	}

	// The disabled checkbox is not submitted: saving must not store a value.
	fields := validSettingsFields()
	fields["feature_my_tennis"] = "off" // even a forged value is ignored
	if rec := e.do(settingsForm(t, fields), models.RoleAdmin); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", rec.Code)
	}
	if v, found, _ := repository.NewSettingsRepository(e.db).GetString(context.Background(), repository.SettingFeatureMyTennis); found {
		t.Errorf("stored %q despite env override", v)
	}
	if !e.holder.ClubSettings().MyTennisEnabled() {
		t.Error("env should keep My Tennis on")
	}
}
