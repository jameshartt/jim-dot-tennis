// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package auth

import (
	"html/template"
	"log"
	"net/http"

	"jim-dot-tennis/internal/models"
)

// ForbiddenMessage is what a signed-in user is told when their role does not
// allow a page or action.
const ForbiddenMessage = "Only an administrator can do this."

// forbiddenPage is the friendly 403 page. It is self-contained (no template
// directory needed) so every role check can use it.
var forbiddenPage = template.Must(template.New("forbidden").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Not available for your account</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <style>
        .forbidden-box { max-width: 36rem; margin: 3rem auto; padding: 2rem; background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); }
        .forbidden-box h1 { margin-top: 0; font-size: 1.4rem; }
        .forbidden-actions a { display: inline-block; margin: 0.5rem 1rem 0 0; }
    </style>
</head>
<body>
    <main class="container">
        <div class="forbidden-box" data-testid="forbidden-page">
            <h1>This page is for administrators</h1>
            <p data-testid="forbidden-message">{{if .Captain}}Your account is a captain account: you can run every team, but this part of the site is kept for the club's administrators.{{else}}Your account does not have a role that can use this part of the site.{{end}} If you need it, ask one of the club's administrators.</p>
            <p class="forbidden-actions">
                {{if .Captain}}<a href="/admin/league/dashboard" data-testid="forbidden-dashboard-link">Back to the dashboard</a>{{end}}
                <a href="/account/signin-methods">Your sign-in methods</a>
            </p>
        </div>
    </main>
</body>
</html>
`))

// WriteForbidden answers a signed-in user whose role does not allow the
// request with 403. A normal page request gets the friendly page; an HTMX
// request gets a short message and HX-Reswap: none, so no fragment is
// replaced with a whole page.
func WriteForbidden(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(RoleContextKey).(models.Role)
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Reswap", "none")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(ForbiddenMessage))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	if err := forbiddenPage.Execute(w, struct{ Captain bool }{models.RoleAtLeast(role, models.RoleCaptain)}); err != nil {
		log.Printf("Failed to render the forbidden page: %v", err)
	}
}
