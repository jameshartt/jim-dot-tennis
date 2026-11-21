// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Environment variables that select the run mode.
const (
	// EnvAppEnv is "production" on a live instance. Production requires a
	// valid https public URL and never runs with insecure cookies.
	EnvAppEnv = "APP_ENV"
	// EnvDevMode opts in to development behaviour: session cookies are not
	// marked Secure, so the app works over plain http (local runs, the e2e
	// containers). It is off unless set to true/1, and the app refuses to
	// start with it set in production.
	EnvDevMode = "DEV_MODE"
)

// FallbackHost is the host used where an address is needed but neither a
// public URL nor a request is available (development only).
const FallbackHost = "localhost"

// Production reports whether APP_ENV=production.
func Production() bool { return os.Getenv(EnvAppEnv) == "production" }

// DevMode reports whether development mode was explicitly enabled (DEV_MODE=true or 1).
func DevMode() bool {
	v := strings.TrimSpace(os.Getenv(EnvDevMode))
	return v == "1" || strings.EqualFold(v, "true")
}

// SecureCookies reports whether session cookies must be marked Secure: always,
// unless development mode is explicitly enabled.
func SecureCookies() bool { return !DevMode() }

// PublicURL is the validated public address of the instance.
type PublicURL struct {
	Origin   string // scheme://host[:port], no path: what a browser reports as the origin
	Base     string // Origin plus any sub-path, no trailing slash: prefix for absolute links
	Scheme   string // "https" or "http"
	Hostname string // host without port: the passkey relying-party ID and the iCal UID domain
}

// HTTPS reports whether the URL is https.
func (p PublicURL) HTTPS() bool { return p.Scheme == "https" }

// parsePublicURL validates s (see NormalizePublicURL) and splits it.
func parsePublicURL(s string) (PublicURL, error) {
	norm, err := NormalizePublicURL(s)
	if err != nil {
		return PublicURL{}, err
	}
	if norm == "" {
		return PublicURL{}, fmt.Errorf("public URL is not set")
	}
	u, err := url.Parse(norm)
	if err != nil {
		return PublicURL{}, err
	}
	return PublicURL{
		Origin:   u.Scheme + "://" + u.Host,
		Base:     norm,
		Scheme:   u.Scheme,
		Hostname: strings.ToLower(u.Hostname()),
	}, nil
}

// ResolvePublicURL returns the configured public URL (PUBLIC_URL, else the
// stored setting), validated. ok is false when none is set or the value is
// invalid; err then says why (nil when simply unset). Code that needs the
// instance's real address (passkeys, recovery links) calls this and must not
// guess from a request.
func (c *ClubSettings) ResolvePublicURL() (p PublicURL, ok bool, err error) {
	raw := c.PublicURL()
	if raw == "" {
		return PublicURL{}, false, nil
	}
	p, err = parsePublicURL(raw)
	if err != nil {
		return PublicURL{}, false, err
	}
	return p, true, nil
}

// BaseURL is the prefix for absolute links: the public URL when configured.
// This is the one place that falls back to the request, which is acceptable
// only outside production (where startup guarantees a public URL). r may be nil.
func (c *ClubSettings) BaseURL(r *http.Request) string {
	if p, ok, _ := c.ResolvePublicURL(); ok {
		return p.Base
	}
	return RequestBaseURL(r)
}

// Host is the host name (no port) for identifiers such as iCal UIDs: the
// public URL's host, else the request's host, else FallbackHost.
func (c *ClubSettings) Host(r *http.Request) string {
	if p, ok, _ := c.ResolvePublicURL(); ok {
		return p.Hostname
	}
	if u, err := url.Parse(RequestBaseURL(r)); err == nil && u.Hostname() != "" {
		return strings.ToLower(u.Hostname())
	}
	return FallbackHost
}

// RequestBaseURL derives scheme://host from a request. Development-only
// fallback: behind a proxy the scheme comes from X-Forwarded-Proto, and the
// Host header is client-supplied, so never use it where the public URL is available.
func RequestBaseURL(r *http.Request) string {
	if r == nil || r.Host == "" {
		return "http://" + FallbackHost
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// ValidateStartup checks the run mode and public URL. In production it
// requires a valid https public URL and refuses DEV_MODE. Outside production
// an unset public URL is allowed (links fall back to the request host) but an
// invalid one is still an error.
func ValidateStartup(c *ClubSettings) error {
	p, ok, err := c.ResolvePublicURL()
	if err != nil {
		return fmt.Errorf("invalid public URL %q: %v; set %s (or the Public URL on the admin Club Settings page) to an address like https://tennis.example.org",
			c.PublicURL(), err, EnvPublicURL)
	}
	if !Production() {
		return nil
	}
	if DevMode() {
		return fmt.Errorf("%s is set but %s=production: development mode makes session cookies insecure; remove %s", EnvDevMode, EnvAppEnv, EnvDevMode)
	}
	if !ok {
		return fmt.Errorf("%s=production needs the instance's public address: set %s=https://tennis.example.org in the app's environment (or enter it as the Public URL on the admin Club Settings page)", EnvAppEnv, EnvPublicURL)
	}
	if !p.HTTPS() {
		return fmt.Errorf("the public URL %q must start with https:// in production; set %s=https://tennis.example.org", c.PublicURL(), EnvPublicURL)
	}
	return nil
}

// PushSubscriber is the VAPID subscriber claim: the https public origin, or
// (development with no https public URL) an https URL on FallbackHost, since
// push services require an https or mailto contact.
func (c *ClubSettings) PushSubscriber() string {
	if p, ok, _ := c.ResolvePublicURL(); ok && p.HTTPS() {
		return p.Origin
	}
	return "https://" + FallbackHost
}
