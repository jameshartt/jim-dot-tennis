// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func settingsWith(t *testing.T, appEnv, devMode, publicURL string) *ClubSettings {
	t.Helper()
	t.Setenv(EnvAppEnv, appEnv)
	t.Setenv(EnvDevMode, devMode)
	t.Setenv(EnvPublicURL, publicURL)
	return NewClubSettings(context.Background(), nil)
}

func TestValidateStartup(t *testing.T) {
	cases := []struct {
		name   string
		appEnv string
		dev    string
		url    string
		want   string // substring of the error, "" for none
	}{
		{"production without URL", "production", "", "", "PUBLIC_URL=https://"},
		{"production with http", "production", "", "http://tennis.example.org", "must start with https://"},
		{"production with garbage", "production", "", "not a url", "invalid public URL"},
		{"production with https", "production", "", "https://tennis.example.org", ""},
		{"production with dev mode", "production", "true", "https://tennis.example.org", "DEV_MODE"},
		{"development without URL", "", "", "", ""},
		{"development with http", "development", "true", "http://localhost:8080", ""},
		{"development with invalid URL", "", "", "ftp://x", "invalid public URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStartup(settingsWith(t, tc.appEnv, tc.dev, tc.url))
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestResolvePublicURL(t *testing.T) {
	cs := settingsWith(t, "", "", "https://Jim.Tennis:8443/club/")
	p, ok, err := cs.ResolvePublicURL()
	if err != nil || !ok {
		t.Fatalf("ResolvePublicURL = %v, %v, %v", p, ok, err)
	}
	if p.Origin != "https://jim.tennis:8443" || p.Base != "https://jim.tennis:8443/club" || p.Hostname != "jim.tennis" || !p.HTTPS() {
		t.Errorf("got %+v", p)
	}
	if cs.BaseURL(nil) != p.Base || cs.Host(nil) != "jim.tennis" || cs.PushSubscriber() != p.Origin {
		t.Errorf("BaseURL/Host/PushSubscriber = %q %q %q", cs.BaseURL(nil), cs.Host(nil), cs.PushSubscriber())
	}
}

func TestRequestFallbackWhenUnset(t *testing.T) {
	cs := settingsWith(t, "", "", "")
	if _, ok, err := cs.ResolvePublicURL(); ok || err != nil {
		t.Fatalf("ResolvePublicURL ok=%v err=%v, want unset", ok, err)
	}
	r := httptest.NewRequest(http.MethodGet, "http://example.test:8080/x", nil)
	if got := cs.BaseURL(r); got != "http://example.test:8080" {
		t.Errorf("BaseURL = %q", got)
	}
	if got := cs.Host(r); got != "example.test" {
		t.Errorf("Host = %q", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := cs.BaseURL(r); got != "https://example.test:8080" {
		t.Errorf("BaseURL behind proxy = %q", got)
	}
	if got := cs.Host(nil); got != FallbackHost {
		t.Errorf("Host(nil) = %q", got)
	}
	if got := cs.PushSubscriber(); got != "https://"+FallbackHost {
		t.Errorf("PushSubscriber = %q", got)
	}
	// A nil *ClubSettings behaves the same (tests build services without one).
	var nilCS *ClubSettings
	if got := nilCS.BaseURL(r); got != "https://example.test:8080" {
		t.Errorf("nil BaseURL = %q", got)
	}
}

func TestPublicURLWinsOverRequestHost(t *testing.T) {
	cs := settingsWith(t, "", "", "https://tennis.example.org")
	r := httptest.NewRequest(http.MethodGet, "http://attacker.test/x", nil)
	if got := cs.BaseURL(r); got != "https://tennis.example.org" {
		t.Errorf("BaseURL = %q, want the public URL", got)
	}
}

func TestSecureCookiesDefaultAndOptOut(t *testing.T) {
	for _, tc := range []struct {
		dev    string
		secure bool
	}{{"", true}, {"false", true}, {"0", true}, {"yes", true}, {"true", false}, {"TRUE", false}, {"1", false}} {
		t.Setenv(EnvDevMode, tc.dev)
		if got := SecureCookies(); got != tc.secure {
			t.Errorf("DEV_MODE=%q: SecureCookies = %v, want %v", tc.dev, got, tc.secure)
		}
	}
}
