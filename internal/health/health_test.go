// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jim-dot-tennis/internal/database"
)

type fake struct {
	st  database.SchemaStatus
	err error
}

func (f fake) ReadSchemaStatus(context.Context) (database.SchemaStatus, error) { return f.st, f.err }

func TestHandlerStates(t *testing.T) {
	cases := []struct {
		name string
		f    fake
		code int
	}{
		{"current", fake{st: database.SchemaStatus{Version: 5, Migrated: true}}, 200},
		{"unreachable", fake{err: errors.New("boom /secret/path")}, 503},
		{"never migrated", fake{}, 503},
		{"dirty", fake{st: database.SchemaStatus{Version: 5, Dirty: true, Migrated: true}}, 503},
		{"behind", fake{st: database.SchemaStatus{Version: 4, Migrated: true}}, 503},
		{"ahead", fake{st: database.SchemaStatus{Version: 6, Migrated: true}}, 503},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		Handler(c.f, 5)(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != c.code {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.code)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s: response leaks the underlying error: %q", c.name, rec.Body.String())
		}
	}
}

func TestVersionHandlerShowsVersionOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	VersionHandler("1.2.3")(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "1.2.3" {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}
