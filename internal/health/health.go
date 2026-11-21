// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package health serves /healthz, the endpoint the container health check
// calls, and /version.
package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"jim-dot-tennis/internal/database"
)

// SchemaReader is the part of the database the health check needs.
type SchemaReader interface {
	ReadSchemaStatus(ctx context.Context) (database.SchemaStatus, error)
}

// Handler answers 200 only when the database can be read and its migrations
// are clean and at latest (the highest migration this build ships). It is
// cheap: one small query. The response says which check failed, never why in
// detail (the endpoint is reachable by anyone who can reach the app).
func Handler(db SchemaReader, latest uint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		st, err := db.ReadSchemaStatus(ctx)
		switch {
		case err != nil:
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		case !st.Migrated || st.Dirty:
			http.Error(w, "migrations not complete", http.StatusServiceUnavailable)
		case st.Version != latest:
			http.Error(w, fmt.Sprintf("migrations not current (database %d, expected %d)", st.Version, latest), http.StatusServiceUnavailable)
		default:
			fmt.Fprintln(w, "ok")
		}
	}
}

// VersionHandler serves the version number alone as plain text. The commit and
// build date are deliberately not public: they tell an attacker exactly which
// code is running; the command line (jim-dot-tennis version) has them.
func VersionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, version)
	}
}
