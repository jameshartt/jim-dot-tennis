// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package render

import (
	"errors"
	"html/template"
	"net/http/httptest"
	"testing"
)

func TestCacheParsesOnceAndRetriesErrors(t *testing.T) {
	c := &Cache{byKey: map[string]*template.Template{}}
	calls := 0
	failing := true
	parse := func() (*template.Template, error) {
		calls++
		if failing {
			return nil, errors.New("bad template")
		}
		return template.New("x").Parse("ok")
	}

	if _, err := c.Get("page", parse); err == nil {
		t.Fatal("expected the parse error")
	}
	failing = false
	first, err := c.Get("page", parse)
	if err != nil {
		t.Fatalf("parse after fix: %v", err)
	}
	second, _ := c.Get("page", parse)
	if first != second || calls != 2 {
		t.Fatalf("want one failed and one cached parse, got %d parses (same=%v)", calls, first == second)
	}
}

func TestCacheReloadReparses(t *testing.T) {
	t.Setenv(ReloadEnv, "true")
	c := NewCache()
	calls := 0
	parse := func() (*template.Template, error) {
		calls++
		return template.New("x").Parse("ok")
	}
	c.Get("page", parse)
	c.Get("page", parse)
	if calls != 2 {
		t.Fatalf("reload mode: got %d parses, want 2", calls)
	}
}

func TestExecuteNamedTemplate(t *testing.T) {
	tmpl := template.Must(template.New("page").Parse(`{{define "row"}}<td>{{.}}</td>{{end}}page`))
	rec := httptest.NewRecorder()
	if err := Execute(rec, tmpl, "row", "<b>"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := rec.Body.String(); got != "<td>&lt;b&gt;</td>" {
		t.Fatalf("got %q", got)
	}
}
