// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package render caches parsed templates and executes them into a buffer so a
// failed render never leaves a half-written page behind a 200 status.
package render

import (
	"bytes"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// ReloadEnv names the environment variable that disables caching, so edits to
// bind-mounted templates show up on the next request during development.
const ReloadEnv = "TEMPLATE_RELOAD"

// Cache holds parsed templates by key. Parse errors are never cached, so a
// broken template is retried on the next request.
type Cache struct {
	mu     sync.Mutex
	byKey  map[string]*template.Template
	reload bool
}

// NewCache returns a cache that re-parses on every call when TEMPLATE_RELOAD
// is set to a true value.
func NewCache() *Cache {
	reload, _ := strconv.ParseBool(os.Getenv(ReloadEnv))
	return &Cache{byKey: map[string]*template.Template{}, reload: reload}
}

// Get returns the cached template for key, calling parse on a miss.
func (c *Cache) Get(key string, parse func() (*template.Template, error)) (*template.Template, error) {
	if c.reload {
		return parse()
	}
	c.mu.Lock()
	tmpl, ok := c.byKey[key]
	c.mu.Unlock()
	if ok {
		return tmpl, nil
	}

	tmpl, err := parse()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.byKey[key]; ok {
		return existing, nil
	}
	c.byKey[key] = tmpl
	return tmpl, nil
}

// Execute renders tmpl (or its named template when name is non-empty) into a
// buffer and only writes to w once rendering succeeded. On error nothing has
// been written, so the caller can still send an error status.
func Execute(w http.ResponseWriter, tmpl *template.Template, name string, data interface{}) error {
	var buf bytes.Buffer
	var err error
	if name == "" {
		err = tmpl.Execute(&buf, data)
	} else {
		err = tmpl.ExecuteTemplate(&buf, name, data)
	}
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w)
	return err
}
