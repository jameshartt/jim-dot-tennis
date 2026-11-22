// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package backup

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"jim-dot-tennis/internal/database"
)

// Scheduling. The server looks every SchedulerCheckEvery whether the newest
// backup is older than the interval and takes one if so, so a restart does
// not cause an extra backup (and does not push older ones out early), and a
// server that was down when one was due takes it soon after starting.
const (
	SchedulerFirstDelay = 2 * time.Minute
	SchedulerCheckEvery = time.Hour
	// dueSlack lets a backup taken a little early (an hourly check just
	// before the interval is up) count, so the time of day does not drift.
	dueSlack = 5 * time.Minute
)

// Status is what the scheduler last did, for the backups page.
type Status struct {
	LastAttempt time.Time
	LastSuccess time.Time
	LastError   string // "" after a success
}

// Service is the server's backups: the configuration, the database and the
// schedule's status.
type Service struct {
	db  *database.DB
	cfg Config
	now func() time.Time

	mu     sync.Mutex
	status Status
}

// NewService returns the backup service for the server's database.
func NewService(db *database.DB, cfg Config) *Service {
	return &Service{db: db, cfg: cfg, now: time.Now}
}

// Config returns the configuration in use.
func (s *Service) Config() Config { return s.cfg }

// Status returns what the schedule last did.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// DB is the database the service backs up.
func (s *Service) DB() *database.DB { return s.db }

// Start runs the schedule until ctx is cancelled or the returned stop is
// called; stop waits for a backup in progress to end (it is cancelled, and
// leaves no partial file behind). With no interval configured it does
// nothing.
func (s *Service) Start(ctx context.Context, firstDelay, checkEvery time.Duration) (stop func()) {
	if s.cfg.Interval <= 0 {
		log.Printf("Scheduled backups are off (%s=off)", EnvInterval)
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTimer(firstDelay)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			s.RunIfDue(ctx)
			timer.Reset(checkEvery)
		}
	}()
	log.Printf("Scheduled backups: every %s into %s, keeping %d", s.cfg.Interval, s.cfg.Dir, max(s.cfg.Keep, 1))
	return func() {
		cancel()
		<-done
	}
}

// RunIfDue takes a scheduled backup if the newest regular backup is older
// than the interval (or there is none) and reports whether it took one.
func (s *Service) RunIfDue(ctx context.Context) bool {
	if !s.due() {
		return false
	}
	s.mu.Lock()
	s.status.LastAttempt = s.now()
	s.mu.Unlock()
	res, err := Create(ctx, s.db, s.cfg, KindScheduled)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if ctx.Err() != nil {
			return false // shutting down
		}
		s.status.LastError = err.Error()
		log.Printf("Scheduled backup failed: %v", err)
		return false
	}
	s.status.LastSuccess = s.now()
	s.status.LastError = ""
	log.Printf("Scheduled backup made: %s (%d bytes)", filepath.Base(res.Path), res.Size)
	return true
}

// due reports whether a scheduled backup is due: the newest regular backup
// that can be believed (Newest: not dated in the future, manifest readable)
// is older than the interval, or there is none.
func (s *Service) due() bool {
	now := s.now()
	a, ok, err := Newest(s.cfg.Dir, now, false)
	if err != nil || !ok {
		return true
	}
	return now.Sub(a.CreatedAt) >= s.cfg.Interval-dueSlack
}
