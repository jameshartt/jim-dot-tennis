// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Retention for the scheduled cleanup.
const (
	// LoginAttemptRetention is how long login attempts are kept. Throttling
	// looks back 15 minutes; the rest is for administrators reading the
	// sessions page.
	LoginAttemptRetention = 90 * 24 * time.Hour
	// FinishedLinkRetention is how long an invite or recovery link is kept
	// once it can no longer be used (used, revoked or expired), counted from
	// when it was issued. Links live for at most 48 hours; the audit log
	// keeps the record of issuing and using them for good.
	FinishedLinkRetention = 30 * 24 * time.Hour
	// CleanupFirstDelay and CleanupInterval schedule the cleanup.
	CleanupFirstDelay = time.Minute
	CleanupInterval   = time.Hour
)

// CleanupResult counts what one cleanup run deleted.
type CleanupResult struct {
	Sessions      int64
	LoginAttempts int64
	Links         int64
}

// Cleanup deletes what has no further use, as of now:
//   - sessions that are invalid (signed out, revoked), past their sliding
//     expiry, or past the absolute lifetime cap;
//   - login attempts older than LoginAttemptRetention;
//   - invite and recovery links that are used, revoked or expired and were
//     issued more than FinishedLinkRetention ago.
//
// The account audit log is never cleaned up (retention: keep).
//
// Timestamps are compared as stored text. Every one of these columns is
// written by Go as "YYYY-MM-DD HH:MM:SS..." in the server's time zone (or by
// SQLite's CURRENT_TIMESTAMP, UTC), so the comparison is off by at most the
// zone offset: harmless here, where nothing is decided by the hour (sign-in
// itself checks expiry exactly, in Go).
func (s *Service) Cleanup(ctx context.Context, now time.Time) (CleanupResult, error) {
	var res CleanupResult
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("cleanup: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	absoluteCutoff := time.Time{} // no absolute cap: matches nothing
	if s.config.AbsoluteSessionDuration > 0 {
		absoluteCutoff = now.Add(-s.config.AbsoluteSessionDuration)
	}
	steps := []struct {
		count *int64
		query string
		args  []any
	}{
		{&res.Sessions, `DELETE FROM sessions WHERE is_valid = FALSE OR expires_at < ? OR created_at < ?`,
			[]any{now, absoluteCutoff}},
		{&res.LoginAttempts, `DELETE FROM login_attempts WHERE created_at < ?`,
			[]any{now.Add(-LoginAttemptRetention)}},
		{&res.Links, `DELETE FROM account_links
			WHERE (used_at IS NOT NULL OR revoked_at IS NOT NULL OR expires_at < ?) AND created_at < ?`,
			[]any{now, now.Add(-FinishedLinkRetention)}},
	}
	for _, step := range steps {
		r, err := tx.ExecContext(ctx, step.query, step.args...)
		if err != nil {
			return res, fmt.Errorf("cleanup: %w", err)
		}
		*step.count, _ = r.RowsAffected()
	}
	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("cleanup: commit: %w", err)
	}
	return res, nil
}

// StartCleanup runs Cleanup after firstDelay and then every interval, until
// ctx is done or the returned stop function is called (stop waits for a run
// in progress to finish).
func (s *Service) StartCleanup(ctx context.Context, firstDelay, interval time.Duration) (stop func()) {
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
			res, err := s.Cleanup(ctx, time.Now())
			switch {
			case err != nil && ctx.Err() == nil:
				log.Printf("Scheduled cleanup failed: %v", err)
			case res.Sessions+res.LoginAttempts+res.Links > 0:
				log.Printf("Scheduled cleanup deleted %d sessions, %d login attempts, %d finished links",
					res.Sessions, res.LoginAttempts, res.Links)
			}
			timer.Reset(interval)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
