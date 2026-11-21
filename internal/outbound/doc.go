// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

// Package outbound holds no code: its test scans the source tree for every
// host the app, its pages and its deployment files name in a URL, and fails
// on any host missing from the allowlist in outbound_test.go. That keeps
// docs/outbound_connections.md ("What this instance connects to") true: a new
// outbound connection cannot be added without updating both.
package outbound
