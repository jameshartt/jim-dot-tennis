// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"log"
	"net/http"
)

// CrossOriginGuard rejects state-changing requests (POST, PUT, DELETE...)
// that a browser sends on behalf of another site, using the Sec-Fetch-Site
// header with an Origin-vs-Host fallback for older browsers. This is CSRF
// protection without per-form tokens: every page, HTMX swap and fetch() is
// covered without template changes. GET/HEAD and non-browser clients (no
// Origin or Sec-Fetch-Site) pass through.
func CrossOriginGuard(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Blocked cross-origin %s %s (Origin %q, Sec-Fetch-Site %q)",
			r.Method, r.URL.Path, r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site"))
		http.Error(w, "Cross-origin request blocked", http.StatusForbidden)
	}))
	return protection.Handler(next)
}
