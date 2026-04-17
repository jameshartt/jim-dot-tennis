// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"net/http"
)

// HomeClubMiddleware injects the home club into every request context.
func HomeClubMiddleware(cfg *AppConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithHomeClub(r.Context(), cfg.HomeClubID, cfg.HomeClub, cfg.HomeClubLogoPath)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
