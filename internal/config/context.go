// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"

	"jim-dot-tennis/internal/models"
)

type contextKey int

const (
	homeClubIDKey contextKey = iota
	homeClubKey
	homeClubLogoPathKey
	brandingKey
)

// WithHomeClub returns a new context with the home club ID, model and logo path injected.
func WithHomeClub(ctx context.Context, id uint, club *models.Club, logoPath string) context.Context {
	ctx = context.WithValue(ctx, homeClubIDKey, id)
	ctx = context.WithValue(ctx, homeClubKey, club)
	ctx = context.WithValue(ctx, homeClubLogoPathKey, logoPath)
	return ctx
}

// GetHomeClubID retrieves the home club ID from context. Returns 0 if not set.
func GetHomeClubID(ctx context.Context) uint {
	id, _ := ctx.Value(homeClubIDKey).(uint)
	return id
}

// GetHomeClub retrieves the full Club model from context. Returns nil if not set.
func GetHomeClub(ctx context.Context) *models.Club {
	club, _ := ctx.Value(homeClubKey).(*models.Club)
	return club
}

// GetHomeClubLogoPath retrieves the configured home club logo path from context.
// Returns an empty string if not set.
func GetHomeClubLogoPath(ctx context.Context) string {
	path, _ := ctx.Value(homeClubLogoPathKey).(string)
	return path
}

// WithBranding returns a new context carrying the club branding.
func WithBranding(ctx context.Context, b Branding) context.Context {
	return context.WithValue(ctx, brandingKey, b)
}

// GetBranding retrieves the club branding from context. Without one (tests,
// tools) it returns the defaults, using the home club's name when present.
func GetBranding(ctx context.Context) Branding {
	if b, ok := ctx.Value(brandingKey).(Branding); ok {
		return b
	}
	return (*ClubSettings)(nil).Branding(GetHomeClub(ctx))
}
