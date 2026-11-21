// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"net/http"
)

// HomeClubMiddleware injects the home club into every request context. It reads
// the live holder on each request, so a home club chosen at runtime (setup)
// takes effect without a restart. While unresolved, the injected ID is 0 and
// the club is nil.
func HomeClubMiddleware(holder *HomeClubHolder, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, club := holder.Current(r.Context())
		ctx := WithHomeClub(r.Context(), id, club, holder.LogoPath())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
