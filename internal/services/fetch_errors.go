// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
)

// FetchProblem says, in broad terms, why a request to the BHPLTA website
// failed, so the message shown to the person can tell the truth about it.
type FetchProblem int

const (
	// FetchOther is anything not covered below.
	FetchOther FetchProblem = iota
	// FetchTimeout: no answer within the time allowed.
	FetchTimeout
	// FetchCancelled: the request was cancelled here.
	FetchCancelled
	// FetchNoConnection: this server never reached the website at all
	// (the name could not be looked up, or the connection could not be
	// made). This is the one case where the server's own internet access
	// is the likely cause.
	FetchNoConnection
	// FetchRefused: the website was reached but refused the request
	// (HTTP 401, 403 or 429) or reset or closed it part-way (a TCP reset,
	// an HTTP/2 stream reset or GOAWAY, an unexpected end of the answer).
	// The server's internet connection is working.
	FetchRefused
	// FetchHTTPStatus: the website answered with another status than 200
	// (for example 500 or 503).
	FetchHTTPStatus
)

// ClassifyFetchError says what kind of failure err is. It looks through
// wrapping (*url.Error, *net.OpError, fmt.Errorf %w).
func ClassifyFetchError(err error) FetchProblem {
	if err == nil {
		return FetchOther
	}
	var status *HTTPStatusError
	if errors.As(err, &status) {
		switch status.Status {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
			return FetchRefused
		}
		return FetchHTTPStatus
	}
	if errors.Is(err, context.Canceled) {
		return FetchCancelled
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return FetchTimeout
	}
	if isReset(err) {
		return FetchRefused
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return FetchNoConnection
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return FetchNoConnection
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return FetchNoConnection
	}
	return FetchOther
}

// isReset reports a request the website reset or closed after this server
// had reached it. Go's HTTP/2 stream and GOAWAY errors are not exported
// types, so those are recognised by their text.
func isReset(err error) bool {
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	for _, s := range []string{
		"stream error:",            // HTTP/2 RST_STREAM, e.g. "stream ID 1; PROTOCOL_ERROR; received from peer"
		"server sent GOAWAY",       // HTTP/2 connection closed by the site
		"connection reset by peer", // TCP reset, when not wrapped as syscall.ECONNRESET
		"REFUSED_STREAM",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// FetchRefusedDetail is a short description of a FetchRefused error for
// the person: "HTTP status 403", "HTTP status 429, too many requests", or
// "it reset or closed the connection (<Go's words>)".
func FetchRefusedDetail(err error) string {
	var status *HTTPStatusError
	if errors.As(err, &status) {
		if status.Status == http.StatusTooManyRequests {
			return "HTTP status 429, too many requests"
		}
		return "HTTP status " + strconv.Itoa(status.Status)
	}
	return "it reset or closed the connection (" + innermostText(err) + ")"
}

func innermostText(err error) string {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err.Error()
		}
		err = next
	}
}
