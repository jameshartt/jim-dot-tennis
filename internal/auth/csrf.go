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
			r.Method, RedactPath(r.URL.Path), r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site"))
		http.Error(w, "Cross-origin request blocked", http.StatusForbidden)
	}))
	return protection.Handler(next)
}

// NoFraming tells browsers never to show any of the app's pages inside a
// frame on any site, its own included, so a page cannot be overlaid on
// another to trick someone into clicking it (clickjacking). Both headers are
// sent: Content-Security-Policy frame-ancestors is the current one, and
// X-Frame-Options covers older browsers. The app sets them itself, on every
// response, so the protection does not depend on the proxy in front of it.
// Nothing in the app frames its own pages (the only frames are OpenStreetMap
// maps, which are another site's pages inside ours and are unaffected).
func NoFraming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
