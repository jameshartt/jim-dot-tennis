// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"jim-dot-tennis/internal/version"
)

// Every request to the BHPLTA website carries the one honest User-Agent.
func TestBHPLTARequestsSendTheHonestUserAgent(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path+" "+r.UserAgent())
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer srv.Close()
	client := srv.Client()
	ctx := context.Background()

	_, _ = FetchFixtures(ctx, client, BHPLTAFixturesURL(srv.URL))
	_, _ = FetchClubPage(ctx, client, srv.URL, "a-club")
	_, _ = (&NonceExtractor{httpClient: client, baseURL: srv.URL + "/cards/"}).ExtractNonce()
	_, _ = (&NonceExtractor{httpClient: client, baseURL: srv.URL + "/cards/"}).ExtractNonceWithClubCode("code")
	_, _ = (&MatchCardService{httpClient: client}).fetchMatchCards(ImportConfig{BaseURL: srv.URL + "/ajax"}, 1)
	_, _ = (&BHPLTAClubLookup{client: client, pageURL: srv.URL + "/cards/", ajaxURL: srv.URL + "/ajax"}).Lookup(ctx, "code")

	if len(got) < 5 {
		t.Fatalf("only %d requests reached the test server: %v", len(got), got)
	}
	for _, line := range got {
		if !strings.HasSuffix(line, " "+version.UserAgent()) {
			t.Errorf("request %q does not carry the User-Agent %q", line, version.UserAgent())
		}
	}
}

func TestClassifyFetchError(t *testing.T) {
	stream := &url.Error{Op: "Get", URL: "https://example.test/", Err: errors.New("stream error: stream ID 13; PROTOCOL_ERROR; received from peer")}
	dns := &url.Error{Op: "Get", URL: "https://example.test/", Err: &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "no such host", Name: "example.test", IsNotFound: true}}}
	cases := []struct {
		name string
		err  error
		want FetchProblem
	}{
		{"403", fmt.Errorf("fetch: %w", &HTTPStatusError{Status: 403}), FetchRefused},
		{"429", &HTTPStatusError{Status: 429}, FetchRefused},
		{"503", &HTTPStatusError{Status: 503}, FetchHTTPStatus},
		{"http2 stream reset", fmt.Errorf("failed to fetch fixtures page: %w", stream), FetchRefused},
		{"goaway", errors.New("http2: server sent GOAWAY and closed the connection"), FetchRefused},
		{"dns", dns, FetchNoConnection},
		{"deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), FetchTimeout},
		{"cancelled", fmt.Errorf("x: %w", context.Canceled), FetchCancelled},
		{"other", errors.New("something else"), FetchOther},
	}
	for _, tc := range cases {
		if got := ClassifyFetchError(tc.err); got != tc.want {
			t.Errorf("%s: ClassifyFetchError(%v) = %d, want %d", tc.name, tc.err, got, tc.want)
		}
	}
	if d := FetchRefusedDetail(fmt.Errorf("x: %w", stream)); !strings.Contains(d, "reset or closed") || !strings.Contains(d, "PROTOCOL_ERROR") {
		t.Errorf("FetchRefusedDetail(stream) = %q", d)
	}
	if d := FetchRefusedDetail(&HTTPStatusError{Status: 429}); d != "HTTP status 429, too many requests" {
		t.Errorf("FetchRefusedDetail(429) = %q", d)
	}
}

// Real connections: a server that is not there, one that resets the
// connection, and one that refuses with 403.
func TestClassifyFetchErrorOnRealConnections(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	_, err := FetchFixtures(ctx, client, BHPLTAFixturesURL(closed.URL))
	if got := ClassifyFetchError(err); got != FetchNoConnection {
		t.Errorf("closed server: %v classified %d, want FetchNoConnection", err, got)
	}

	reset := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, herr := w.(http.Hijacker).Hijack()
		if herr != nil {
			return
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0) // close with a TCP reset
		}
		_ = conn.Close()
	}))
	defer reset.Close()
	_, err = FetchFixtures(ctx, client, BHPLTAFixturesURL(reset.URL))
	if got := ClassifyFetchError(err); got != FetchRefused {
		t.Errorf("reset: %v classified %d, want FetchRefused", err, got)
	}

	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer forbidden.Close()
	_, err = FetchFixtures(ctx, client, BHPLTAFixturesURL(forbidden.URL))
	if got := ClassifyFetchError(err); got != FetchRefused {
		t.Errorf("403: %v classified %d, want FetchRefused", err, got)
	}
}
