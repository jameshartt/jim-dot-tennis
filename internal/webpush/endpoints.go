// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package webpush

import (
	"net/url"
	"strings"
)

// pushServiceHosts are the browser vendors' push services. A subscription's
// endpoint is chosen by the browser, but it reaches the server through a
// public form, so only endpoints on these services are stored: otherwise
// anyone could make the server send requests to a host of their choosing.
// An entry starting with "." matches any subdomain.
var pushServiceHosts = []string{
	"fcm.googleapis.com",         // Chrome, Android, Samsung Internet, Opera, Brave
	"android.googleapis.com",     // older Chrome (GCM-era endpoints)
	".push.services.mozilla.com", // Firefox (updates.push.services.mozilla.com)
	".push.apple.com",            // Safari (web.push.apple.com; iOS / iPadOS home-screen apps)
	".notify.windows.com",        // Edge (Windows Push Notification Services)
}

// allowedPushEndpoint reports whether endpoint is an https URL on a known
// browser push service.
func allowedPushEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, h := range pushServiceHosts {
		if strings.HasPrefix(h, ".") {
			if strings.HasSuffix(host, h) {
				return true
			}
		} else if host == h {
			return true
		}
	}
	return false
}
