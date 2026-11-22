// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"jim-dot-tennis/internal/branding"
	"jim-dot-tennis/internal/config"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/secrets"
	"jim-dot-tennis/internal/services"
	"jim-dot-tennis/internal/tournaments"
)

// Settings page routes.
const (
	settingsPath       = "/admin/league/settings"
	settingsLookupPath = "/admin/league/settings/bhplta-lookup"
)

// settingsMaxBody bounds a settings form post: the logo plus the text fields.
const settingsMaxBody = branding.MaxLogoBytes + 256<<10

// bhpltaLookupFunc finds a club's BHPLTA name and id from its club code.
type bhpltaLookupFunc func(ctx context.Context, clubCode string) (services.BHPLTAClubIdentity, error)

// SettingsHandler serves the club settings page: display name, logo, colours,
// BHPLTA identifiers and the public URL. Only the admin role may use it, even
// if the surrounding /admin/league routes are later opened to other roles.
type SettingsHandler struct {
	service     *Service
	templateDir string
	lookup      bhpltaLookupFunc
}

// NewSettingsHandler creates the settings handler.
func NewSettingsHandler(service *Service, templateDir string) *SettingsHandler {
	return &SettingsHandler{
		service:     service,
		templateDir: templateDir,
		lookup:      services.NewBHPLTAClubLookup().Lookup,
	}
}

// SettingsPageData is the data for templates/admin/settings.html.
type SettingsPageData struct {
	User    *models.User
	Message string
	Error   string

	ClubName    string // the home club's name (the display name's fallback)
	DisplayName string

	LogoURL      string // effective logo shown on pages ("" for none)
	LogoUploaded bool   // a logo has been uploaded
	LogoEnv      string // env var overriding the logo, if set
	LogoKnown    bool   // the logo is the known club's bundled logo (none uploaded, no env)
	LogoMaxMB    int

	PrimaryColour   string
	SecondaryColour string

	BHPLTAClubCode    string
	BHPLTAClubCodeEnv string
	BHPLTAClubName    string
	BHPLTAClubID      string
	BHPLTAFromKnown   bool // name/id come from the built-in list, not stored

	PublicURL    string
	PublicURLEnv string

	Features       config.Features // effective feature switches
	MyTennisEnvVar string          // env var overriding the My Tennis switch, if set
	WrappedCode    string          // Club Wrapped access code for members ("" if unavailable)
	WrappedCodeEnv string          // env var overriding the generated code, if set

	AdminPasskeys adminPasskeysView // "Require passkeys for administrators"

	TournamentSetup tournamentSetupView // the club's tournament set-up (WI-145)
}

// HandleSettings serves GET and POST /admin/league/settings and POST .../bhplta-lookup.
func (h *SettingsHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	cs := h.service.clubSettings
	if cs == nil {
		http.Error(w, "Settings are not available", http.StatusInternalServerError)
		return
	}

	switch {
	case r.URL.Path == settingsPath && r.Method == http.MethodGet:
		h.render(w, r, user, h.pageData(r, cs), http.StatusOK)
	case r.URL.Path == settingsPath && r.Method == http.MethodPost:
		h.save(w, r, user, cs)
	case r.URL.Path == settingsLookupPath && r.Method == http.MethodPost:
		h.bhpltaLookup(w, r, cs)
	case r.URL.Path == settingsAdminPasskeysPath && r.Method == http.MethodPost:
		h.setAdminPasskeys(w, r, user)
	case r.URL.Path == settingsPath || r.URL.Path == settingsLookupPath || r.URL.Path == settingsAdminPasskeysPath:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	default:
		http.NotFound(w, r)
	}
}

// requireAdmin allows only the admin role.
func (h *SettingsHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, err := getUserFromContext(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if user.Role != models.RoleAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, false
	}
	return user, true
}

func (h *SettingsHandler) pageData(r *http.Request, cs *config.ClubSettings) SettingsPageData {
	club := config.GetHomeClub(r.Context())
	d := SettingsPageData{
		Message:         r.URL.Query().Get("message"),
		DisplayName:     cs.Stored(repository.SettingClubDisplayName),
		LogoURL:         cs.LogoPath(club),
		LogoUploaded:    cs.Stored(repository.SettingClubLogoFile) != "",
		LogoMaxMB:       branding.MaxLogoBytes >> 20,
		PrimaryColour:   cs.PrimaryColour(),
		SecondaryColour: cs.SecondaryColour(),
		BHPLTAClubCode:  cs.Stored(repository.SettingBHPLTAClubCode),
		BHPLTAClubName:  cs.Stored(repository.SettingBHPLTAClubName),
		BHPLTAClubID:    cs.Stored(repository.SettingBHPLTAClubID),
		PublicURL:       cs.PublicURL(),
		Features:        cs.Features(),
	}
	if club != nil {
		d.ClubName = club.Name
	}
	d.LogoEnv, _ = cs.EnvOverride(repository.SettingClubLogoFile)
	if known, ok := config.KnownClubLogo(club); ok && d.LogoEnv == "" && !d.LogoUploaded && d.LogoURL == known {
		d.LogoKnown = true
	}
	d.BHPLTAClubCodeEnv, _ = cs.EnvOverride(repository.SettingBHPLTAClubCode)
	d.PublicURLEnv, _ = cs.EnvOverride(repository.SettingPublicURL)
	d.MyTennisEnvVar, _ = cs.EnvOverride(repository.SettingFeatureMyTennis)
	d.AdminPasskeys = h.adminPasskeysView(r, cs)
	d.TournamentSetup = h.service.tournamentSetupData(r)
	d.TournamentSetup.Outcome = setupOutcome(r.URL.Query().Get("tournament_setup"))
	d.TournamentSetup.Problem = r.URL.Query().Get("setup_problem")
	if code, fromEnv, err := secrets.NewWrappedAccess(h.service.db).Code(r.Context()); err != nil {
		log.Printf("Club Wrapped access code unavailable: %v", err)
	} else if fromEnv {
		d.WrappedCodeEnv = secrets.EnvWrappedAccess
	} else {
		d.WrappedCode = code
	}
	if d.BHPLTAClubName == "" && d.BHPLTAClubID == "" {
		if k, ok := config.KnownBHPLTAClub(club); ok {
			d.BHPLTAClubName, d.BHPLTAClubID, d.BHPLTAFromKnown = k.Name, strconv.Itoa(k.ID), true
		}
	}
	return d
}

func (h *SettingsHandler) render(w http.ResponseWriter, r *http.Request, user *models.User, data SettingsPageData, status int) {
	data.User = user
	tmpl, err := parseTemplate(h.templateDir, "admin/settings.html")
	if err != nil {
		logAndError(w, "Failed to parse template", err, http.StatusInternalServerError)
		return
	}
	// The page shows the instance's settings and its Club Wrapped code.
	w.Header().Set("Cache-Control", "no-store")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := renderTemplate(w, tmpl, data); err != nil {
		log.Printf("Failed to render settings page: %v", err)
	}
}

// sentence capitalises the first letter of a validation message for display.
func sentence(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formError is a validation problem to show on the page.
type formError struct{ msg string }

func (e formError) Error() string { return e.msg }

// save validates the whole form first and stores nothing unless all of it is valid.
func (h *SettingsHandler) save(w http.ResponseWriter, r *http.Request, user *models.User, cs *config.ClubSettings) {
	r.Body = http.MaxBytesReader(w, r.Body, settingsMaxBody)
	if err := r.ParseMultipartForm(1 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		var tooBig *http.MaxBytesError
		msg := "The form could not be read."
		if errors.As(err, &tooBig) || r.ContentLength > settingsMaxBody {
			msg = sentence(branding.ErrLogoTooLarge.Error()) + "."
		}
		data := h.pageData(r, cs)
		data.Error = msg
		h.render(w, r, user, data, http.StatusBadRequest)
		return
	}

	// Switching Tournaments on sets the club up on the bundled tournament
	// stack, when this deployment has one (WI-145).
	wasOn := cs.TournamentsEnabled()
	values, logo, err := h.parseForm(r, cs)
	if err == nil {
		err = h.publicURLChangeRefused(r, cs, values)
	}
	turningOn := err == nil && !wasOn && values[repository.SettingFeatureTournaments] == "true"
	var abbreviation string
	if turningOn && err == nil {
		abbreviation, err = h.tournamentAbbreviation(r, cs)
	}
	if err != nil {
		data := h.pageData(r, cs)
		data.Error = sentence(err.Error())
		h.render(w, r, user, data, http.StatusBadRequest)
		return
	}

	removeLogo := r.FormValue("remove_logo") == "on"
	var newLogo string
	switch {
	case logo != nil:
		name, err := branding.Store(cs.Dir(), logo)
		if err != nil {
			logAndError(w, "Failed to store the logo", err, http.StatusInternalServerError)
			return
		}
		newLogo = name
		values[repository.SettingClubLogoFile] = name
		values[repository.SettingClubLogoSize] = fmt.Sprintf("%dx%d", logo.Width, logo.Height)
	case removeLogo:
		values[repository.SettingClubLogoFile] = ""
		values[repository.SettingClubLogoSize] = ""
	}

	if err := cs.Save(r.Context(), values); err != nil {
		logAndError(w, "Failed to save settings", err, http.StatusInternalServerError)
		return
	}
	if logo != nil || removeLogo {
		if err := branding.RemoveOtherLogos(cs.Dir(), newLogo); err != nil {
			log.Printf("Warning: could not remove old logos: %v", err)
		}
	}
	log.Printf("Club settings updated by %s", user.Username)
	target := settingsPath + "?message=" + url.QueryEscape("Settings saved.")
	if turningOn {
		outcome, problem := setupExternal, ""
		if h.service.tournamentSetup.Configured() {
			outcome, problem = h.service.runTournamentSetup(r, user, abbreviation)
		}
		target += "&tournament_setup=" + outcome
		if problem != "" {
			target += "&setup_problem=" + url.QueryEscape(problem)
		}
		target += "#tournament-setup"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// tournamentAbbreviation is the club's tournament code to set up with when
// Tournaments is being switched on: the one on the form (checked), else the
// default. "" when there is nothing to choose (no bundled stack, or the club
// is already set up and its code is fixed).
func (h *SettingsHandler) tournamentAbbreviation(r *http.Request, cs *config.ClubSettings) (string, error) {
	setup := h.service.tournamentSetup
	if !setup.Configured() {
		return "", nil
	}
	st, err := setup.Status(r.Context())
	if err != nil || st.SetUp {
		return "", nil
	}
	if v := strings.TrimSpace(r.FormValue("tournament_abbreviation")); v != "" {
		a, err := tournaments.NormalizeAbbreviation(v)
		if err != nil {
			return "", formError{sentence(err.Error()) + "."}
		}
		return a, nil
	}
	return setup.DefaultAbbreviation(r.Context(), clubNameForTournaments(r, cs)), nil
}

// parseForm validates the submitted fields and returns the values to store.
// Fields overridden by the environment are left as they are (their inputs
// are disabled and not submitted).
func (h *SettingsHandler) parseForm(r *http.Request, cs *config.ClubSettings) (map[string]string, *branding.Logo, error) {
	values := map[string]string{}
	var err error

	if values[repository.SettingClubDisplayName], err = config.NormalizeDisplayName(r.FormValue("display_name")); err != nil {
		return nil, nil, err
	}
	if values[repository.SettingClubPrimaryColour], err = config.NormalizeColour(r.FormValue("primary_colour")); err != nil {
		return nil, nil, formError{"Primary " + err.Error()}
	}
	if values[repository.SettingClubSecondaryColour], err = config.NormalizeColour(r.FormValue("secondary_colour")); err != nil {
		return nil, nil, formError{"Secondary " + err.Error()}
	}
	if _, env := cs.EnvOverride(repository.SettingBHPLTAClubCode); !env {
		if values[repository.SettingBHPLTAClubCode], err = config.NormalizeBHPLTAText("BHPLTA club code", r.FormValue("bhplta_club_code")); err != nil {
			return nil, nil, err
		}
	}
	if values[repository.SettingBHPLTAClubName], err = config.NormalizeBHPLTAText("BHPLTA club name", r.FormValue("bhplta_club_name")); err != nil {
		return nil, nil, err
	}
	if values[repository.SettingBHPLTAClubID], err = config.NormalizeBHPLTAClubID(r.FormValue("bhplta_club_id")); err != nil {
		return nil, nil, err
	}
	if _, env := cs.EnvOverride(repository.SettingPublicURL); !env {
		if values[repository.SettingPublicURL], err = config.NormalizePublicURL(r.FormValue("public_url")); err != nil {
			return nil, nil, err
		}
		if v := values[repository.SettingPublicURL]; config.Production() && !strings.HasPrefix(v, "https://") {
			return nil, nil, fmt.Errorf("the public URL must start with https:// on a production instance, and cannot be cleared")
		}
	}
	// Feature switches: each is a hidden "off" input followed by a checkbox
	// "on", so a form that omits a switch leaves it as it is. A switch set by
	// the environment is not submitted and is left alone.
	for _, f := range []struct{ field, key string }{
		{"feature_tournaments", repository.SettingFeatureTournaments},
		{"feature_club_wrapped", repository.SettingFeatureClubWrapped},
		{"feature_my_tennis", repository.SettingFeatureMyTennis},
	} {
		if _, env := cs.EnvOverride(f.key); env {
			continue
		}
		if vals, present := r.Form[f.field]; present && len(vals) > 0 {
			values[f.key] = strconv.FormatBool(slices.Contains(vals, "on"))
		}
	}

	logo, err := readLogo(r)
	if err != nil {
		return nil, nil, err
	}
	return values, logo, nil
}

// readLogo returns the validated uploaded logo, or nil when none was chosen.
func readLogo(r *http.Request) (*branding.Logo, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	f, _, err := r.FormFile("logo")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, formError{"The logo upload could not be read."}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, branding.MaxLogoBytes+1))
	if err != nil {
		return nil, formError{"The logo upload could not be read."}
	}
	if len(data) == 0 {
		return nil, nil // an empty file input
	}
	logo, err := branding.ValidateLogo(data)
	if err != nil {
		return nil, formError{err.Error() + "."}
	}
	return logo, nil
}

// bhpltaLookup asks BHPLTA for the club's name and id using the effective
// club code, and stores them.
func (h *SettingsHandler) bhpltaLookup(w http.ResponseWriter, r *http.Request, cs *config.ClubSettings) {
	redirect := func(param, msg string) {
		http.Redirect(w, r, settingsPath+"?"+param+"="+url.QueryEscape(msg)+"#bhplta", http.StatusSeeOther)
	}
	id, err := h.lookup(r.Context(), cs.BHPLTAClubCode())
	if err != nil {
		log.Printf("BHPLTA club lookup failed: %v", err)
		redirect("message", "Could not look up the club on BHPLTA: "+err.Error()+". Enter the name and id by hand (see below).")
		return
	}
	name, err := config.NormalizeBHPLTAText("BHPLTA club name", id.Name)
	if err != nil {
		redirect("message", "BHPLTA returned an unexpected club name. Enter the name and id by hand.")
		return
	}
	if err := cs.Save(r.Context(), map[string]string{
		repository.SettingBHPLTAClubName: name,
		repository.SettingBHPLTAClubID:   strconv.Itoa(id.ID),
	}); err != nil {
		logAndError(w, "Failed to save settings", err, http.StatusInternalServerError)
		return
	}
	redirect("message", fmt.Sprintf("Found on BHPLTA: %s (club id %d). Saved.", name, id.ID))
}
