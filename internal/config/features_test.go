// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package config

import (
	"context"
	"testing"
	"time"

	"jim-dot-tennis/internal/repository"
)

func TestFeatureDefaults(t *testing.T) {
	clearClubSettingsEnv(t)
	t.Setenv(EnvMyTennisEnabled, "")
	e := newTestEnv(t)
	ctx := context.Background()
	cs := NewClubSettings(ctx, e.settings)

	// An instance that predates the setup flow (no setup_completed_at) keeps today's behaviour.
	if f := cs.Features(); !f.Tournaments || !f.ClubWrapped || f.MyTennis {
		t.Errorf("legacy defaults = %+v, want tournaments on, wrapped on, my tennis off", f)
	}

	// An instance set up through the setup flow starts with tournaments off.
	if err := e.settings.SetTime(ctx, repository.SettingSetupCompletedAt, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := cs.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if f := cs.Features(); f.Tournaments || !f.ClubWrapped || f.MyTennis {
		t.Errorf("new-install defaults = %+v, want tournaments off, wrapped on, my tennis off", f)
	}

	// A stored value wins over either default, and applies on Save without a restart.
	if err := cs.Save(ctx, map[string]string{
		repository.SettingFeatureTournaments: "true",
		repository.SettingFeatureClubWrapped: "false",
		repository.SettingFeatureMyTennis:    "true",
	}); err != nil {
		t.Fatal(err)
	}
	if f := cs.Features(); !f.Tournaments || f.ClubWrapped || !f.MyTennis {
		t.Errorf("stored = %+v", f)
	}

	// A nil ClubSettings reports the legacy defaults.
	var none *ClubSettings
	if f := none.Features(); !f.Tournaments || !f.ClubWrapped || f.MyTennis {
		t.Errorf("nil = %+v", f)
	}
}

func TestMyTennisEnvOverridesStored(t *testing.T) {
	clearClubSettingsEnv(t)
	e := newTestEnv(t)
	ctx := context.Background()
	if err := e.settings.SetString(ctx, repository.SettingFeatureMyTennis, "false"); err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvMyTennisEnabled, "1")
	cs := NewClubSettings(ctx, e.settings)
	if !cs.MyTennisEnabled() {
		t.Error("MY_TENNIS_ENABLED=1 should override the stored false")
	}
	if v, ok := cs.EnvOverride(repository.SettingFeatureMyTennis); !ok || v != EnvMyTennisEnabled {
		t.Errorf("EnvOverride = %q %v", v, ok)
	}

	t.Setenv(EnvMyTennisEnabled, "false")
	if err := e.settings.SetString(ctx, repository.SettingFeatureMyTennis, "true"); err != nil {
		t.Fatal(err)
	}
	cs = NewClubSettings(ctx, e.settings)
	if cs.MyTennisEnabled() {
		t.Error("MY_TENNIS_ENABLED=false should override the stored true")
	}

	t.Setenv(EnvMyTennisEnabled, "")
	cs = NewClubSettings(ctx, e.settings)
	if !cs.MyTennisEnabled() {
		t.Error("an empty env value counts as unset")
	}
	if _, ok := cs.EnvOverride(repository.SettingFeatureMyTennis); ok {
		t.Error("empty env reported as an override")
	}
}
