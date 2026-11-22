// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package tournaments sets up the club on the bundled tournament stack
// (WI-145): the app's provisioner, the club's provider, the permanent
// "instance" provider administrator, the stored ids and the calendar entry
// that tournament sync uses. One entry point, Service.Provision, used by the
// settings page (when Tournaments is switched on), the tournaments page and
// `jim-dot-tennis tournaments provision`. Every step is safe to repeat, so a
// run that stops part-way is finished by the next one.
//
// No person gets a tournament credential here; people reach tournament
// management through the hand-over (WI-152). The only secret involved is the
// service account's password, which stays inside the courthive client.
//
// See docs/courthive_bundle_design.md, section 4.
package tournaments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"jim-dot-tennis/internal/accounts"
	"jim-dot-tennis/internal/courthive"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/normalize"
	"jim-dot-tennis/internal/repository"
)

// ActionSetup is the account audit log action for a set-up run.
const ActionSetup = "tournaments.setup"

// Client is the part of *courthive.Client that set-up uses.
type Client interface {
	Configured() bool
	Version(ctx context.Context) (string, error)
	EnsureProvisioner(ctx context.Context) (id string, created bool, err error)
	EnsureProvider(ctx context.Context, provisionerID, abbreviation, name string) (id string, created bool, err error)
	EnsureAccount(ctx context.Context, providerID string, a courthive.Account) (string, error)
	Members(ctx context.Context, providerID string) ([]courthive.Member, error)
	Providers(ctx context.Context) ([]courthive.Provider, error)
	RemoveAccount(ctx context.Context, providerID, userID, email string) error
	HandoverToken(ctx context.Context, providerID, externalID string) (string, error)
}

// ErrNotConfigured: the bundled tournament stack is not part of this
// deployment (COURTHIVE_SERVICE_PASSWORD is empty).
var ErrNotConfigured = errors.New("the bundled tournament stack is not switched on in this deployment (COURTHIVE_SERVICE_PASSWORD is empty)")

// ErrNoAbbreviation: there is no abbreviation to set the club up with.
var ErrNoAbbreviation = errors.New("the club needs a tournament abbreviation")

// AbbreviationFixedError: the club is set up with another abbreviation, and
// changing it would leave the club's tournaments behind.
type AbbreviationFixedError struct{ Current string }

func (e *AbbreviationFixedError) Error() string {
	return fmt.Sprintf("the club's tournament abbreviation is %s and cannot be changed once tournaments are set up", e.Current)
}

// StepError is a set-up run that stopped at a step. Its text is plain words
// and the reason given by the tournament stack; it never holds a token or the
// password.
type StepError struct {
	Step    string // the step's label
	Problem string // what went wrong, in plain words, with what to do
	Err     error
}

func (e *StepError) Error() string { return e.Step + ": " + e.Problem }
func (e *StepError) Unwrap() error { return e.Err }

// Outcome is what a step found or did.
type Outcome string

// Step outcomes.
const (
	OutcomeChecked Outcome = "checked" // looked, nothing to make
	OutcomeExisted Outcome = "existed" // already there; nothing changed
	OutcomeMade    Outcome = "made"    // made (or completed) by this run
	OutcomeFailed  Outcome = "failed"
)

// Step is one step of a set-up run.
type Step struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Outcome Outcome `json:"outcome"`
	Detail  string  `json:"detail,omitempty"`
}

// OutcomeText is the outcome in words, for pages and the command line.
func (s Step) OutcomeText() string {
	switch s.Outcome {
	case OutcomeChecked:
		return "Done"
	case OutcomeExisted:
		return "Already there"
	case OutcomeMade:
		return "Made"
	case OutcomeFailed:
		return "Failed"
	}
	return string(s.Outcome)
}

// UnmanagedAccount is a tournament account in the club's provider that
// jim-dot-tennis did not make and does not manage. Set-up changes nothing
// for it.
type UnmanagedAccount struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// RoleText is the account's provider role in words.
func (a UnmanagedAccount) RoleText() string {
	switch a.Role {
	case courthive.RoleProviderAdmin:
		return "Tournament administrator (every tournament)"
	case courthive.RoleDirector:
		return "Tournament director (own tournaments)"
	case "":
		return "No role"
	}
	return a.Role
}

// Result is the outcome of one set-up run.
type Result struct {
	RanAt         time.Time          `json:"ran_at"`
	By            string             `json:"by"`
	OK            bool               `json:"ok"`
	Abbreviation  string             `json:"abbreviation"`
	ProviderName  string             `json:"provider_name"`
	EngineVersion string             `json:"engine_version,omitempty"`
	Steps         []Step             `json:"steps"`
	FailedStep    string             `json:"failed_step,omitempty"` // label of the step that failed
	Problem       string             `json:"problem,omitempty"`     // plain words
	Detail        string             `json:"detail,omitempty"`      // the reason the tournament stack gave
	Unmanaged     []UnmanagedAccount `json:"unmanaged,omitempty"`
}

// MadeAnything reports whether the run made or completed anything.
func (r *Result) MadeAnything() bool {
	for _, s := range r.Steps {
		if s.Outcome == OutcomeMade {
			return true
		}
	}
	return false
}

// Request is what to set the club up as.
type Request struct {
	// Abbreviation is the provider abbreviation the administrator confirmed.
	// Empty means the stored one (a run that stopped part-way). Once the club
	// is set up it must be empty or the same.
	Abbreviation string
	// Name is the provider's name: the club's name.
	Name string
}

// Status is the stored set-up state.
type Status struct {
	Configured    bool   // the bundled tournament stack is part of this deployment
	SetUp         bool   // the club's provider id is stored
	Abbreviation  string // fixed once SetUp; before that, the one last tried
	ProvisionerID string
	ProviderID    string
	Last          *Result // the last run, nil if there has been none
}

// Service runs and reports the club's tournament set-up.
type Service struct {
	db        *database.DB
	client    Client
	settings  repository.SettingsRepository
	providers repository.TournamentProviderRepository
	audit     *accounts.Service
	now       func() time.Time

	mu sync.Mutex // one run at a time in this process
}

// NewService returns the set-up service. client may be nil or unconfigured:
// then Provision returns ErrNotConfigured and Status says so.
func NewService(db *database.DB, client Client) *Service {
	return &Service{
		db:        db,
		client:    client,
		settings:  repository.NewSettingsRepository(db),
		providers: repository.NewTournamentProviderRepository(db),
		audit:     accounts.New(db),
		now:       time.Now,
	}
}

// Configured reports whether the bundled tournament stack is part of this
// deployment.
func (s *Service) Configured() bool {
	return s != nil && s.client != nil && s.client.Configured()
}

// EngineVersion asks the tournament server for its engine version, which
// also shows it can be reached. It needs no sign-in.
func (s *Service) EngineVersion(ctx context.Context) (string, error) {
	if s == nil || s.client == nil {
		return "", ErrNotConfigured
	}
	return s.client.Version(ctx)
}

type stored struct {
	provisionerID, providerID, abbreviation, instanceID string
}

func (s *Service) load(ctx context.Context) (stored, error) {
	var st stored
	for key, dst := range map[string]*string{
		repository.SettingTournamentProvisionerID:     &st.provisionerID,
		repository.SettingTournamentProviderID:        &st.providerID,
		repository.SettingTournamentProviderAbbr:      &st.abbreviation,
		repository.SettingTournamentInstanceAccountID: &st.instanceID,
	} {
		v, _, err := s.settings.GetString(ctx, key)
		if err != nil {
			return st, err
		}
		*dst = v
	}
	return st, nil
}

// Status returns the stored set-up state and the last run.
func (s *Service) Status(ctx context.Context) (Status, error) {
	st, err := s.load(ctx)
	if err != nil {
		return Status{}, err
	}
	out := Status{
		Configured:    s.Configured(),
		SetUp:         st.providerID != "",
		Abbreviation:  st.abbreviation,
		ProvisionerID: st.provisionerID,
		ProviderID:    st.providerID,
	}
	raw, ok, err := s.settings.GetString(ctx, repository.SettingTournamentSetupLast)
	if err != nil {
		return out, err
	}
	if ok && raw != "" {
		var last Result
		if json.Unmarshal([]byte(raw), &last) == nil {
			out.Last = &last
		}
	}
	return out, nil
}

// abbreviationPattern is what a provider abbreviation may be.
var abbreviationPattern = regexp.MustCompile(`^[A-Z0-9]{2,12}$`)

// NormalizeAbbreviation trims and upper-cases s and checks it is 2 to 12
// letters and digits.
func NormalizeAbbreviation(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !abbreviationPattern.MatchString(s) {
		return "", errors.New("the tournament abbreviation must be 2 to 12 letters or digits, with no spaces (for example STANNS)")
	}
	return s, nil
}

// dropWords are left out of an abbreviation made from a club's name.
var dropWords = map[string]bool{
	"tennis": true, "club": true, "lawn": true, "tc": true, "ltc": true,
	"the": true, "and": true, "of": true,
}

// DeriveAbbreviation makes an abbreviation from a club's name: its
// distinctive words run together ("St Ann's Tennis Club" gives STANNS), or
// their initials when that is longer than 10 characters.
func DeriveAbbreviation(name string) string {
	var all, keep []string
	for _, w := range strings.Fields(normalize.ForComparison(name)) {
		var b strings.Builder
		for _, r := range strings.ToUpper(w) {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		if b.Len() == 0 {
			continue
		}
		all = append(all, b.String())
		if !dropWords[strings.ToLower(b.String())] {
			keep = append(keep, b.String())
		}
	}
	if len(keep) == 0 {
		keep = all
	}
	joined := strings.Join(keep, "")
	switch {
	case len(joined) >= 2 && len(joined) <= 10:
		return joined
	case len(keep) >= 2:
		var initials strings.Builder
		for _, w := range keep {
			initials.WriteByte(w[0])
		}
		if initials.Len() >= 2 {
			return truncate(initials.String(), 12)
		}
	case len(joined) > 10:
		return joined[:10]
	}
	return "CLUB"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// DefaultAbbreviation is the abbreviation to offer: the stored one (fixed
// once set up, otherwise the one last tried), else that of the only provider
// already registered in the app (an instance that pointed at CourtHive by
// hand, so its tournaments are kept), else one made from the club's name.
func (s *Service) DefaultAbbreviation(ctx context.Context, clubName string) string {
	if st, err := s.load(ctx); err == nil && st.abbreviation != "" {
		return st.abbreviation
	}
	if list, err := s.providers.FindAll(ctx); err == nil && len(list) == 1 {
		if a, err := NormalizeAbbreviation(list[0].ProviderAbbr); err == nil {
			return a
		}
	}
	return DeriveAbbreviation(clubName)
}

// Step keys and labels.
const (
	stepReach       = "reach"
	stepProvisioner = "provisioner"
	stepProvider    = "provider"
	stepInstance    = "instance"
	stepRegister    = "register"
)

var stepLabels = map[string]string{
	stepReach:       "Reach the tournament server",
	stepProvisioner: "Connect this app to the tournament server",
	stepProvider:    "The club on the tournament server",
	stepInstance:    "Permanent tournament administrator",
	stepRegister:    "The club's tournament calendar in this app",
}

// Provision sets the club up on the tournament stack, or finishes or checks
// an earlier set-up. It returns the run's result (also stored, and recorded
// in the account audit log) and, when a step failed, a *StepError. Before
// any step it can return ErrNotConfigured, ErrNoAbbreviation, an
// *AbbreviationFixedError or an invalid-abbreviation error, with a nil result.
func (s *Service) Provision(ctx context.Context, actor accounts.Actor, req Request) (*Result, error) {
	if !s.Configured() {
		return nil, ErrNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	abbr := st.abbreviation
	if req.Abbreviation != "" {
		a, err := NormalizeAbbreviation(req.Abbreviation)
		if err != nil {
			return nil, err
		}
		if st.providerID != "" && !strings.EqualFold(a, st.abbreviation) {
			return nil, &AbbreviationFixedError{Current: st.abbreviation}
		}
		if st.providerID == "" {
			abbr = a
		}
	}
	if abbr == "" {
		return nil, ErrNoAbbreviation
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = abbr
	}

	res := &Result{RanAt: s.now().UTC(), By: actorName(actor), Abbreviation: abbr, ProviderName: name}
	done := func(key string, outcome Outcome, detail string) {
		res.Steps = append(res.Steps, Step{Key: key, Label: stepLabels[key], Outcome: outcome, Detail: detail})
	}
	fail := func(key string, err error) (*Result, error) {
		res.Steps = append(res.Steps, Step{Key: key, Label: stepLabels[key], Outcome: OutcomeFailed})
		res.FailedStep = stepLabels[key]
		res.Problem = explain(key, err)
		res.Detail = err.Error()
		s.finish(ctx, actor, res)
		return res, &StepError{Step: res.FailedStep, Problem: res.Problem, Err: err}
	}
	save := func(key, value, current string) error {
		if value == current {
			return nil
		}
		return s.settings.SetString(ctx, key, value)
	}

	// The abbreviation is stored before anything is made, so a run that stops
	// part-way (even after the provider is made) is retried with the same
	// one, and the provider is adopted rather than made twice.
	if err := save(repository.SettingTournamentProviderAbbr, abbr, st.abbreviation); err != nil {
		return nil, err
	}

	// 1. The tournament server answers.
	version, err := s.client.Version(ctx)
	if err != nil {
		return fail(stepReach, err)
	}
	res.EngineVersion = version
	done(stepReach, OutcomeChecked, "Tournament engine "+version)

	// 2. The app's provisioner, represented by the service account (this is
	// where the app first signs in, so a wrong password stops here).
	provisionerID, created, err := s.client.EnsureProvisioner(ctx)
	if err != nil {
		return fail(stepProvisioner, err)
	}
	if err := save(repository.SettingTournamentProvisionerID, provisionerID, st.provisionerID); err != nil {
		return fail(stepProvisioner, err)
	}
	done(stepProvisioner, outcome(created), "")

	// 3. The club's provider (adopted if one with the abbreviation exists).
	providerID, created, err := s.client.EnsureProvider(ctx, provisionerID, abbr, name)
	if err != nil {
		return fail(stepProvider, err)
	}
	if err := save(repository.SettingTournamentProviderID, providerID, st.providerID); err != nil {
		return fail(stepProvider, err)
	}
	switch {
	case created:
		done(stepProvider, OutcomeMade, abbr+", "+name)
	case st.providerID != providerID:
		done(stepProvider, OutcomeMade, abbr+": found on the tournament server and taken on by this app")
	default:
		done(stepProvider, OutcomeExisted, abbr)
	}

	// 4. The permanent "instance" administrator, so a person can always be
	// removed. The member list also gives the accounts the app does not manage.
	members, err := s.client.Members(ctx, providerID)
	if err != nil {
		return fail(stepInstance, err)
	}
	instanceThere := false
	for _, m := range members {
		if m.Provider == courthive.SSOProvider && m.ExternalID == courthive.InstanceExternalID && m.Role == courthive.RoleProviderAdmin {
			instanceThere = true
		}
		if m.Provider != courthive.SSOProvider {
			res.Unmanaged = append(res.Unmanaged, UnmanagedAccount{Email: m.Email, Role: m.Role})
		}
	}
	instance := courthive.Account{
		ExternalID: courthive.InstanceExternalID,
		Email:      courthive.AccountEmail(courthive.InstanceExternalID),
		Role:       courthive.RoleProviderAdmin,
		UserID:     st.instanceID,
	}
	instanceID, err := s.client.EnsureAccount(ctx, providerID, instance)
	if errors.Is(err, courthive.ErrAccountUnknown) {
		// The account exists on the server but is in no provider this app can
		// see it through, and its id was not stored (for example the app's
		// database was replaced, or the club's provider was made again).
		// Look for it in the other providers the app owns.
		if instance.UserID, err = s.findInstanceAccount(ctx); err == nil {
			instanceID, err = s.client.EnsureAccount(ctx, providerID, instance)
		}
	}
	if err != nil {
		return fail(stepInstance, err)
	}
	if err := save(repository.SettingTournamentInstanceAccountID, instanceID, st.instanceID); err != nil {
		return fail(stepInstance, err)
	}
	done(stepInstance, outcome(!instanceThere), "Never used to sign in")

	// 5. The provider in the app's own list, so calendar sync works now.
	madeRow, err := s.register(ctx, abbr, name)
	if err != nil {
		return fail(stepRegister, err)
	}
	done(stepRegister, outcome(madeRow), "")

	res.OK = true
	s.finish(ctx, actor, res)
	return res, nil
}

// findInstanceAccount finds the "instance" account's id through the member
// lists of the providers the app's provisioner owns. ErrAccountUnknown when
// it is in none of them.
func (s *Service) findInstanceAccount(ctx context.Context) (string, error) {
	providers, err := s.client.Providers(ctx)
	if err != nil {
		return "", err
	}
	for _, p := range providers {
		if !p.Managed {
			continue
		}
		members, err := s.client.Members(ctx, p.ID)
		if err != nil {
			return "", err
		}
		for _, m := range members {
			if m.Provider == courthive.SSOProvider && m.ExternalID == courthive.InstanceExternalID && m.UserID != "" {
				return m.UserID, nil
			}
		}
	}
	return "", courthive.ErrAccountUnknown
}

func outcome(made bool) Outcome {
	if made {
		return OutcomeMade
	}
	return OutcomeExisted
}

// register makes sure tournament_providers has a row for abbr, matching an
// existing one whatever its case.
func (s *Service) register(ctx context.Context, abbr, name string) (made bool, err error) {
	list, err := s.providers.FindAll(ctx)
	if err != nil {
		return false, err
	}
	for _, p := range list {
		if strings.EqualFold(strings.TrimSpace(p.ProviderAbbr), abbr) {
			return false, nil
		}
	}
	if err := s.providers.Create(ctx, &models.TournamentProvider{Name: name, ProviderAbbr: abbr}); err != nil {
		return false, err
	}
	return true, nil
}

// finish stores the run's result and records it in the audit log. Neither
// failing stops the run's result being returned; both are logged.
func (s *Service) finish(ctx context.Context, actor accounts.Actor, res *Result) {
	if raw, err := json.Marshal(res); err == nil {
		if err := s.settings.SetString(ctx, repository.SettingTournamentSetupLast, string(raw)); err != nil {
			log.Printf("Tournament set-up: could not store the outcome: %v", err)
		}
	}
	if s.audit == nil {
		return
	}
	detail := "ok abbreviation=" + res.Abbreviation
	if !res.OK {
		detail = "failed abbreviation=" + res.Abbreviation + " step=" + stepKey(res)
	}
	var made []string
	for _, st := range res.Steps {
		if st.Outcome == OutcomeMade {
			made = append(made, st.Key)
		}
	}
	if len(made) > 0 {
		detail += " made=" + strings.Join(made, ",")
	} else if res.OK {
		detail += " made=nothing"
	}
	if len(res.Unmanaged) > 0 {
		detail += fmt.Sprintf(" unmanaged_accounts=%d", len(res.Unmanaged))
	}
	if err := s.audit.RecordEvent(ctx, actor, ActionSetup, detail); err != nil {
		log.Printf("Tournament set-up: could not record it in the audit log: %v", err)
	}
}

func stepKey(res *Result) string {
	for _, st := range res.Steps {
		if st.Outcome == OutcomeFailed {
			return st.Key
		}
	}
	return ""
}

func actorName(a accounts.Actor) string {
	switch {
	case a.Username != "":
		return a.Username
	case a.Kind == accounts.ActorCLI:
		return "the server command line"
	}
	return ""
}

// explain says in plain words what a failed step means and what to do.
func explain(key string, err error) string {
	var apiErr *courthive.APIError
	isAPI := errors.As(err, &apiErr)
	signIn := strings.Contains(err.Error(), "identity service")
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return "The tournament stack took too long to answer. It may still be starting: wait a minute, then try again."
	case key == stepReach:
		return "The tournament server did not answer. It may still be starting: wait a minute, then try again."
	case errors.Is(err, courthive.ErrAccountUnknown):
		return "The permanent tournament administrator already exists on the tournament server, but outside the club's place there, and this app cannot find it. This happens when the tournament software was set up from another copy of this app's database. See the tournaments section of the operations guide."
	case signIn && isAPI && (apiErr.Status == 401 || apiErr.Status == 403):
		return "The tournament sign-in service did not accept this app's service password. Check that COURTHIVE_SERVICE_PASSWORD is the password the tournament stack was set up with, restart the app, then try again."
	case signIn && !isAPI:
		return "The tournament sign-in service did not answer. It may still be starting: wait a minute, then try again."
	case signIn:
		return "The tournament sign-in service refused the request. Try again; if it keeps happening, see the operations guide."
	case key == stepRegister:
		return "This app could not save the club's tournament calendar. Try again."
	case isAPI:
		return "The tournament server refused this step. Try again; if it keeps happening, see the operations guide."
	case strings.HasPrefix(err.Error(), "courthive:"):
		return "The tournament stack stopped answering during this step. Wait a minute, then try again."
	}
	return "This app could not save the set-up. Try again."
}
