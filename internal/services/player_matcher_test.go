// Copyright (c) 2025-2026 James Hartt. Licensed under the MIT License.

package services

import (
	"context"
	"testing"

	"jim-dot-tennis/internal/models"
	"jim-dot-tennis/internal/repository"
)

func newMatcher() *PlayerMatcher { return &PlayerMatcher{} }

func TestNormalizeName(t *testing.T) {
	m := newMatcher()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "John SMITH", "john smith"},
		{"collapses whitespace", "  John    Smith  ", "john smith"},
		{"strips dots and commas", "Smith, John.", "smith john"},
		{"hyphen becomes space", "Anne-Marie", "anne marie"},
		{"keeps apostrophe", "O'Brien", "o'brien"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := m.normalizeName(c.in); got != c.want {
				t.Errorf("normalizeName(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}

	// Apostrophe variants must normalise to the same string so O'Brien matches
	// regardless of which Unicode apostrophe the source used.
	straight := m.normalizeName("Sean O'Brien") // U+0027
	curly := m.normalizeName("Sean O’Brien")    // U+2019
	if straight != curly {
		t.Errorf("apostrophe variants differ: %q vs %q", straight, curly)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	m := newMatcher()
	cases := []struct {
		s1, s2 string
		want   int
	}{
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "", 3},
		{"same", "same", 0},
		{"flaw", "lawn", 2},
	}
	for _, c := range cases {
		if got := m.levenshteinDistance(c.s1, c.s2); got != c.want {
			t.Errorf("levenshteinDistance(%q,%q) = %d, want %d", c.s1, c.s2, got, c.want)
		}
	}
}

func TestCalculateSimilarity(t *testing.T) {
	m := newMatcher()
	if got := m.calculateSimilarity("john smith", "john smith"); got != 1.0 {
		t.Errorf("identical strings: got %v, want 1.0", got)
	}
	if got := m.calculateSimilarity("", "john"); got != 0.0 {
		t.Errorf("empty string: got %v, want 0.0", got)
	}
	// One insertion in a 10-char string -> 1 - 1/10 = 0.9
	if got := m.calculateSimilarity("jon smith", "john smith"); got < 0.89 || got > 0.91 {
		t.Errorf("one-edit similarity: got %v, want ~0.9", got)
	}
}

// fakePlayerRepo embeds the interface so only FindAll needs implementing;
// MatchPlayer is the only caller and only uses FindAll.
type fakePlayerRepo struct {
	repository.PlayerRepository
	players []models.Player
}

func (f *fakePlayerRepo) FindAll(ctx context.Context) ([]models.Player, error) {
	return f.players, nil
}

func TestMatchPlayer(t *testing.T) {
	repo := &fakePlayerRepo{players: []models.Player{
		{ID: "p1", FirstName: "John", LastName: "Smith"},
		{ID: "p2", FirstName: "Sean", LastName: "O’Brien"}, // curly apostrophe in DB
		{ID: "p3", FirstName: "Alice", LastName: "Wong"},
	}}
	m := NewPlayerMatcher(repo)
	ctx := context.Background()

	t.Run("exact match", func(t *testing.T) {
		id, err := m.MatchPlayer(ctx, "John Smith")
		if err != nil || id != "p1" {
			t.Fatalf("got (%q, %v), want (p1, nil)", id, err)
		}
	})

	t.Run("apostrophe variant matches", func(t *testing.T) {
		id, err := m.MatchPlayer(ctx, "Sean O'Brien") // straight apostrophe in input
		if err != nil || id != "p2" {
			t.Fatalf("got (%q, %v), want (p2, nil)", id, err)
		}
	})

	t.Run("fuzzy match on a typo", func(t *testing.T) {
		id, err := m.MatchPlayer(ctx, "Jon Smith")
		if err != nil || id != "p1" {
			t.Fatalf("got (%q, %v), want (p1, nil)", id, err)
		}
	})

	t.Run("no match below threshold errors", func(t *testing.T) {
		if id, err := m.MatchPlayer(ctx, "Zachary Xylophone"); err == nil {
			t.Fatalf("expected error for unmatched name, got id %q", id)
		}
	})
}
