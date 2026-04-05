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
)

// WithHomeClub returns a new context with the home club ID and model injected.
func WithHomeClub(ctx context.Context, id uint, club *models.Club) context.Context {
	ctx = context.WithValue(ctx, homeClubIDKey, id)
	ctx = context.WithValue(ctx, homeClubKey, club)
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
