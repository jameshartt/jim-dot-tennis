# Sprint 016: My Tennis — Profile Revamp & Player Self-Expression

## Overview

**Goal**: Revamp `/my-profile` into a playful 'My Tennis' experience that helps the club get the most out of everyone's tennis — while hardening privacy on the open-internet shareable token URL via initials-only rendering and a write-only submission flow.

**Duration**: 2 weeks (dates TBD)

**Status**: Completed (2026-04-18)

**Framing**: This is not a 'preferences' page. It's a celebration of how each player plays — their strengths, their quirks, their style — in service of making the whole club's tennis better. The language, structure, and affordances all lean into that.

## Background

The current `/my-profile/{token}` route renders a player's full name (profile.html:349-358) and match history (match_history.html:156). The fantasy token is effectively a shareable link — forwarded in group chats, screenshots — so any page rendered under that URL must assume an untrusted viewer.

The existing profile is read-only and thin. This sprint turns it into the canonical way players describe their tennis, with rich optional fields that feed the Sprint 017 captain planning dashboard.

## Focus Areas

1. Privacy hardening on token-accessed URLs (initials-only, write-only)
2. Rich, playful tennis self-expression captured as structured data
3. Groundwork for captain-facing planning (Sprint 017) without leaking back to players

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies |
|----|-------|----------|------------|--------------|
| WI-093 | Initials-only rendering on profile + history | High | S | None |
| WI-094 | Schema: tennis prefs, partner prefs, captain notes, user-player link | High | M | None |
| WI-095 | Revamp /my-profile into 'My Tennis' form | High | L | WI-094 |
| WI-096 | Prominent 'My Tennis' CTA on availability page | Medium | S | WI-095 |
| WI-097 | Write-only submission flow | High | M | WI-094, WI-095 |
| WI-098 | Reusable preference summary partial (admin-only) | Medium | S | WI-094 |
| WI-099 | Admin read view on player detail page | Medium | S | WI-098 |
| WI-100 | E2E tests — privacy, write-only, merge | High | M | WI-093, WI-095, WI-096, WI-097, WI-099 |

## Execution Strategy

### Phase 1: Foundations (parallel)
- **WI-093** — Initials-only rendering (no dependencies; trivial, land first)
- **WI-094** — Schema + models + down migrations

### Phase 2: Core build (WI-094 must complete)
- **WI-095** — New 'My Tennis' form template + handler
- **WI-098** — Preference summary partial (parallel to WI-095)

### Phase 3: Privacy and integration
- **WI-097** — Write-only submission flow (depends on WI-094 + WI-095)
- **WI-099** — Admin read view (depends on WI-098)
- **WI-096** — Availability CTA (depends on WI-095)

### Phase 4: Lock it in
- **WI-100** — E2E regression suite

## Critical Path

```
WI-093 ─────────────────────────────────────→ WI-100
WI-094 ─→ WI-095 ─→ WI-097 ─────────────────→ WI-100
      └─→ WI-098 ─→ WI-099 ─────────────────→ WI-100
              └──→ WI-096 ─────────────────→ WI-100
```

## Key Design Decisions

1. **Positive framing throughout**: The player-facing form celebrates difference. No 'avoid' fields, no negative prompts. Discreet/tactical information ('someone I'd rather not partner with') is captain-managed in Sprint 017, not player-authored.

2. **Write-only on the token URL**: GET renders a blank form, POST merges and shows a session-only confirmation, re-visit shows a blank form again. Stored values are ONLY readable via admin-session auth. This accepts UX friction (a player can't review what they last said from the shareable link) in exchange for a clean privacy guarantee.

3. **Initials-only everywhere under /my-profile/***: Full names leak identity to anyone with the forwarded URL. Hero uses 'My Tennis — J.H.' pattern. Page `<title>` is generic.

4. **Schema shape**: One-to-one preferences table (scalar columns, nullable) + join table for partner-preferred (positive only) + separate captain-only notes table (never queried from player handlers). JSON columns only for intrinsically list-shaped values.

5. **user.player_id FK delivered here**: Used by Sprint 017 but migration 025 lands now so Sprint 017 can build straight on top.

## Taxonomy (abbreviated — full list in WI-094)

- **Identity & Vibe** — years playing, how I got into it, tennis hero, pre-match ritual
- **Match Types** — mixed appetite, same-gender appetite, open-to-fill-in
- **Playing Style** — handedness, backhand, serve, net comfort, court side, signature shot, shot I'm working on, favourite tactic
- **Partnership** — consistency, on-court vibe, clicks_with, would_love_to_try
- **Intensity & Goals** — competitiveness (1–5), pressure response, season goal, improvement focus
- **Logistics** — preferred days, times, travel, transport, home court
- **Health & Access** — 'what to know about my game', accessibility, weather tolerance
- **Fun** — tennis spirit animal, walkout song, celebration style, post-match, my tennis in one line
- **Comms** — preferred contact, last-minute window, notes to captain

## Success Metrics

- [ ] No full name renders on `/my-profile/*` or `/my-profile/{token}/history`
- [ ] Reloading `/my-profile/{token}` after submission shows a blank form
- [ ] Preferences round-trip cleanly through the admin-side read view
- [ ] Merge semantics: partial submissions never clobber other fields
- [ ] Copy across the form is warm, playful, optional-friendly

## Work Items Tracking

### Completed
- None

### In Progress
- None

### Not Started
- WI-093 through WI-100
