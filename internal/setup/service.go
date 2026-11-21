// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package setup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"

	"jim-dot-tennis/internal/auth"
	"jim-dot-tennis/internal/database"
	"jim-dot-tennis/internal/repository"
	"jim-dot-tennis/internal/services"
)

// app_settings keys used by the setup flow. Only hashes of secrets are stored.
const (
	settingCodeHash        = "setup_code_hash"         // SHA-256 of the active setup code
	settingCodeIssuedAt    = "setup_code_issued_at"    // when the active code was generated
	settingCodeRedeemedAt  = "setup_code_redeemed_at"  // when the last code was used
	settingTicketHash      = "setup_ticket_hash"       // SHA-256 of the browser ticket from a redeemed code
	settingTicketExpiresAt = "setup_ticket_expires_at" // when that ticket stops working
	settingAdminUserID     = "setup_admin_user_id"     // users.id of the first administrator
	settingAdminCreatedAt  = "setup_admin_created_at"  // when the first administrator was created
)

const (
	// ticketLifetime is how long a redeemed code keeps working in the browser
	// that redeemed it, so the flow survives the browser being closed.
	ticketLifetime = 24 * time.Hour
	// doneWindow is how long after completion the confirmation page is shown.
	doneWindow = 30 * time.Minute
)

// Errors returned by the setup service.
var (
	// ErrUnavailable: the step does not apply to this instance any more (for
	// code entry and administrator creation: an administrator exists). The
	// handler answers 404.
	ErrUnavailable = errors.New("setup: step not available")
	// ErrThrottled: too many wrong codes recently.
	ErrThrottled = errors.New("setup: too many wrong codes")
	// ErrWrongCode: the code does not match.
	ErrWrongCode = errors.New("setup: wrong setup code")
	// ErrNoActiveCode: no code is waiting to be used (already used, or none issued).
	ErrNoActiveCode = errors.New("setup: no active setup code")
	// ErrTicketInvalid: the browser has not redeemed the current code.
	ErrTicketInvalid = errors.New("setup: setup code not entered in this browser")
	// ErrUnknownClub: the chosen club is not a known BHPLTA club.
	ErrUnknownClub = errors.New("setup: unknown club")
)

// InputError is a problem with what the person typed, shown next to the field.
type InputError struct {
	Field   string
	Message string
}

func (e *InputError) Error() string { return e.Field + ": " + e.Message }

// Service runs the first-run setup flow: setup code, first administrator,
// home club.
type Service struct {
	state       *State
	db          *database.DB
	provisioner HomeClubProvisioner
	throttle    *codeThrottle
	now         func() time.Time

	// mu serialises the state-changing steps so a double submit or two
	// browsers racing cannot both redeem a code or both create an administrator.
	mu sync.Mutex

	// jobMu guards job, the club step running in the background.
	jobMu sync.Mutex
	job   *ClubJob

	// demoActive mirrors the demo data flag in app_settings (see demo.go).
	demoActive atomic.Bool
}

// clubJobTimeout bounds a background club step (reading BHPLTA included).
const clubJobTimeout = 3 * time.Minute

// ClubJob is the club step running (or finished) in the background: reading
// BHPLTA takes longer than a page request should.
type ClubJob struct {
	Slug     string
	ClubName string
	Running  bool
	Step     int    // pages read so far (progress)
	Total    int    // pages to read
	What     string // what is being read now
	Started  time.Time
	Finished time.Time
	Result   *ProvisionResult // on success
	Err      error            // on failure
}

// NewService returns the setup flow over state. provisioner builds the home
// club's data in the club step (see HomeClubProvisioner); nil means
// NewClubRowProvisioner.
func NewService(state *State, provisioner HomeClubProvisioner) *Service {
	if provisioner == nil {
		provisioner = NewClubRowProvisioner(state.db)
	}
	s := &Service{
		state:       state,
		db:          state.db,
		provisioner: provisioner,
		throttle:    newCodeThrottle(),
		now:         time.Now,
	}
	s.loadDemoFlag(context.Background())
	return s
}

// State returns the setup state the service drives.
func (s *Service) State() *State { return s.state }

// IssueCode generates a new setup code if the instance has no administrator
// (with or without a home club), replacing any earlier code and invalidating any
// browser that redeemed one. It returns the code for the caller to show once
// (the startup log); only its hash is stored. It returns "" when no code is
// needed.
func (s *Service) IssueCode(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Status(ctx).NeedsFirstAdmin() {
		return "", nil
	}
	code, err := newSetupCode()
	if err != nil {
		return "", fmt.Errorf("setup: generate code: %w", err)
	}
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		st := repository.NewSettingsRepository(tx)
		if err := st.SetString(ctx, settingCodeHash, hashSecret(normalizeCode(code))); err != nil {
			return err
		}
		if err := st.SetTime(ctx, settingCodeIssuedAt, s.now()); err != nil {
			return err
		}
		for _, k := range []string{settingCodeRedeemedAt, settingTicketHash, settingTicketExpiresAt} {
			if err := st.Delete(ctx, k); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	s.throttle.reset()
	return code, nil
}

// LogCode prints a setup code to the server log, once, at startup. This is
// the only place a code is ever shown.
func LogCode(code string) {
	log.Printf("\n"+
		"==================================================\n"+
		"  SETUP CODE: %s\n"+
		"  This app has no administrator yet. Open it in a\n"+
		"  browser and enter this code to create the first\n"+
		"  administrator. It works once; restarting the app\n"+
		"  prints a new code and cancels this one.\n"+
		"==================================================", code)
}

// RedeemCode checks code from client (an address used for throttling). On a
// match the code is used up and a browser ticket is returned; the ticket, not
// the code, carries the person through the administrator step.
func (s *Service) RedeemCode(ctx context.Context, code, client string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Status(ctx).NeedsFirstAdmin() {
		return "", ErrUnavailable
	}
	now := s.now()
	// An address that has used up its own wrong codes gets nothing checked.
	if s.throttle.clientBlocked(client, now) {
		return "", ErrThrottled
	}
	settings := repository.NewSettingsRepository(s.db)
	stored, ok, err := settings.GetString(ctx, settingCodeHash)
	if err != nil {
		return "", err
	}
	if !ok || stored == "" {
		return "", ErrNoActiveCode
	}
	// The code is compared (in constant time, by hash) before the global
	// limit is looked at, and the right code is accepted whatever that limit
	// says. The global limit used to refuse every code, right or wrong, once
	// 30 wrong ones had arrived from anywhere, so a stranger sending 30 wrong
	// codes every 15 minutes could stop the owner ever claiming their own
	// instance.
	//
	// Letting the right code through does mean guesses spread over many
	// addresses are each still checked (5 per address per window). That is
	// safe because the code is 60 random bits: at a million guesses a second,
	// far more than this server could answer, the expected time to hit it is
	// about 18,000 years, and the code is replaced at every restart. The
	// limits are there to keep the noise down, not to make the code strong.
	// Wrong codes are counted either way, so the per-address limit holds
	// during a flood too.
	if !secretMatches(normalizeCode(code), stored) {
		s.throttle.fail(client, now)
		if s.throttle.globalBlocked(now) {
			return "", ErrThrottled
		}
		return "", ErrWrongCode
	}

	ticket, err := newTicket()
	if err != nil {
		return "", fmt.Errorf("setup: generate ticket: %w", err)
	}
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		st := repository.NewSettingsRepository(tx)
		if err := st.Delete(ctx, settingCodeHash); err != nil {
			return err
		}
		if err := st.SetTime(ctx, settingCodeRedeemedAt, now); err != nil {
			return err
		}
		if err := st.SetString(ctx, settingTicketHash, hashSecret(ticket)); err != nil {
			return err
		}
		return st.SetTime(ctx, settingTicketExpiresAt, now.Add(ticketLifetime))
	})
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// CodeWasRedeemed reports whether the most recent setup code has been used.
func (s *Service) CodeWasRedeemed(ctx context.Context) bool {
	_, ok, err := repository.NewSettingsRepository(s.db).GetTime(ctx, settingCodeRedeemedAt)
	return err == nil && ok
}

// TicketValid reports whether ticket is the live ticket from the current
// code and the instance still needs its first administrator.
func (s *Service) TicketValid(ctx context.Context, ticket string) bool {
	if ticket == "" || !s.state.Status(ctx).NeedsFirstAdmin() {
		return false
	}
	ok, err := ticketValid(ctx, s.db, ticket, s.now())
	return err == nil && ok
}

func ticketValid(ctx context.Context, db sqlx.ExtContext, ticket string, now time.Time) (bool, error) {
	st := repository.NewSettingsRepository(db)
	stored, ok, err := st.GetString(ctx, settingTicketHash)
	if err != nil || !ok {
		return false, err
	}
	expires, ok, err := st.GetTime(ctx, settingTicketExpiresAt)
	if err != nil || !ok {
		return false, err
	}
	if !now.Before(expires) {
		return false, nil
	}
	return secretMatches(ticket, stored), nil
}

// usernamePattern: letters, digits and . _ - @, starting with a letter or digit.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{2,63}$`)

// CreateAdmin creates the first administrator for the browser holding
// ticket. The account and the settings recording it are written in one
// transaction, which also retires the ticket; it fails with ErrUnavailable if
// an administrator already exists. Existing captain or player accounts are
// left as they are (an imported or restored database may have them); holding
// the setup code from the server log is what proves the right to administer
// the instance, and those accounts gain nothing from it.
func (s *Service) CreateAdmin(ctx context.Context, ticket, username, password string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.state.Status(ctx).NeedsFirstAdmin() {
		return 0, ErrUnavailable
	}
	if ok, err := ticketValid(ctx, s.db, ticket, s.now()); err != nil {
		return 0, err
	} else if !ok {
		return 0, ErrTicketInvalid
	}

	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return 0, &InputError{Field: "username", Message: "Use 3 to 64 letters or numbers. You can also use . _ - and @ (but not spaces)."}
	}
	if err := auth.ValidatePassword(password, username); err != nil {
		return 0, &InputError{Field: "password", Message: strings.TrimPrefix(err.Error(), auth.ErrPasswordPolicy.Error()+": ")}
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("setup: hash password: %w", err)
	}

	var userID int64
	err = s.inTx(ctx, func(tx *sqlx.Tx) error {
		// Re-check inside the transaction: the ticket is still the live one,
		// there is still no administrator and the username is free.
		if ok, err := ticketValid(ctx, tx, ticket, s.now()); err != nil {
			return err
		} else if !ok {
			return ErrTicketInvalid
		}
		var admins, taken int
		if err := tx.GetContext(ctx, &admins, `SELECT COUNT(*) FROM users WHERE role = 'admin'`); err != nil {
			return fmt.Errorf("setup: count administrators: %w", err)
		}
		if admins > 0 {
			return ErrUnavailable
		}
		if err := tx.GetContext(ctx, &taken, `SELECT COUNT(*) FROM users WHERE username = ? COLLATE NOCASE`, username); err != nil {
			return fmt.Errorf("setup: check username: %w", err)
		}
		if taken > 0 {
			return &InputError{Field: "username", Message: "That username is already used by another account here. Choose a different one."}
		}
		now := s.now()
		res, err := tx.ExecContext(ctx, `
			INSERT INTO users (username, password_hash, role, is_active, created_at, last_login_at)
			VALUES (?, ?, 'admin', TRUE, ?, ?)`, username, hash, now, now)
		if err != nil {
			return fmt.Errorf("setup: create administrator: %w", err)
		}
		if userID, err = res.LastInsertId(); err != nil {
			return fmt.Errorf("setup: create administrator: %w", err)
		}
		st := repository.NewSettingsRepository(tx)
		if err := st.SetUint(ctx, settingAdminUserID, uint(userID)); err != nil {
			return err
		}
		if err := st.SetTime(ctx, settingAdminCreatedAt, now); err != nil {
			return err
		}
		for _, k := range []string{settingTicketHash, settingTicketExpiresAt, settingCodeHash} {
			if err := st.Delete(ctx, k); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return userID, nil
}

// Clubs returns the BHPLTA clubs offered in the club step.
func (s *Service) Clubs() []ClubChoice {
	return services.KnownClubSlugs
}

// ChooseClub provisions the chosen BHPLTA club as the home club, records
// setup as complete in the same transaction, and makes it effective
// immediately. It is only available while an administrator exists and no
// home club is in effect; the handler checks the caller is a signed-in
// administrator.
func (s *Service) ChooseClub(ctx context.Context, slug string) (*ProvisionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status(ctx) != StatusNeedsClub {
		return nil, ErrUnavailable
	}
	var choice *ClubChoice
	for _, c := range s.Clubs() {
		if c.Slug == slug {
			choice = &c
			break
		}
	}
	if choice == nil {
		return nil, ErrUnknownClub
	}
	completeCalled := false
	res, err := s.provisioner.ProvisionHomeClub(ctx, *choice, func(ctx context.Context, tx *sqlx.Tx, clubID uint) error {
		completeCalled = true
		return WriteCompletion(ctx, tx, clubID)
	})
	if err != nil {
		return nil, err
	}
	if !completeCalled {
		return nil, errors.New("setup: provisioner did not record completion")
	}
	if err := s.state.Activate(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// ErrClubJobRunning: the club step is already running.
var ErrClubJobRunning = errors.New("setup: the club is already being set up")

// StartChooseClub runs ChooseClub for slug in the background, so the page can
// show progress while BHPLTA is read. It checks what it can up front
// (ErrUnavailable, ErrUnknownClub, ErrClubJobRunning); later failures are
// reported in ClubJob.
func (s *Service) StartChooseClub(ctx context.Context, slug string) error {
	if s.state.Status(ctx) != StatusNeedsClub {
		return ErrUnavailable
	}
	var choice *ClubChoice
	for _, c := range s.Clubs() {
		if c.Slug == slug {
			choice = &c
			break
		}
	}
	if choice == nil {
		return ErrUnknownClub
	}

	s.jobMu.Lock()
	if s.job != nil && s.job.Running {
		s.jobMu.Unlock()
		return ErrClubJobRunning
	}
	job := &ClubJob{Slug: slug, ClubName: choice.DisplayName, Running: true, Started: s.now()}
	s.job = job
	s.jobMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), clubJobTimeout)
		defer cancel()
		ctx = WithProgress(ctx, func(step, total int, what string) {
			s.jobMu.Lock()
			job.Step, job.Total, job.What = step, total, what
			s.jobMu.Unlock()
		})
		res, err := s.ChooseClub(ctx, slug)
		if err != nil {
			log.Printf("setup: choose club %q: %v", slug, err)
		} else {
			log.Printf("setup: home club set to %q (club ID %d, created %v); setup complete", res.ClubName, res.ClubID, res.CreatedClub)
		}
		s.jobMu.Lock()
		job.Running, job.Finished, job.Result, job.Err = false, s.now(), res, err
		s.jobMu.Unlock()
	}()
	return nil
}

// ClubJob returns a copy of the latest background club step, if any.
func (s *Service) ClubJob() (ClubJob, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.job == nil {
		return ClubJob{}, false
	}
	return *s.job, true
}

// RecentlyCompleted reports whether setup was completed within the last
// doneWindow (the confirmation page is only shown then).
func (s *Service) RecentlyCompleted(ctx context.Context) bool {
	if s.state.Status(ctx) != StatusReady {
		return false
	}
	at, ok, err := repository.NewSettingsRepository(s.db).GetTime(ctx, repository.SettingSetupCompletedAt)
	if err != nil || !ok {
		return false
	}
	return s.now().Sub(at) < doneWindow
}

func (s *Service) inTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("setup: begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("setup: commit: %w", err)
	}
	return nil
}
