# Sprint 018: My Tennis — Tiered Wizard & Enthusiastic Consent

## Overview

**Goal**: Reshape the My Tennis form (delivered in Sprint 016) from a single long page of nine collapsible sections into a stepped wizard of six priority-ordered tiers. Every tier ends with an equal-weight "Save & finish here" off-ramp. Players are encouraged — never pressured — to stop once they've shared enough.

**Duration**: 2 weeks (dates TBD)

**Status**: Completed (2026-04-25)

**Framing**: After feedback that the Sprint 016 form felt overwhelming despite every field being optional, this sprint changes *presentation order and gating* — not the data model. The wizard exists primarily for the player's enjoyment; the captain's data is the secondary beneficiary.

## Background

Sprint 016 shipped a rich nine-section profile form with per-section saves, mobile-first CSS, and a write-only privacy contract. Feedback after delivery: the volume of optional fields, presented all at once, makes the form feel like a chore — even though every field is optional and partial completion was the design intent.

The redesign is a presentation refactor. The data model from migration 024 is preserved exactly. The write-only contract from WI-097 is preserved exactly. The admin read surface from WI-098/WI-099 is untouched. What changes: how the form is *presented* to the player, and a single new integer column tracking how far they've progressed.

## Focus Areas

1. Priority-ordered tiers — what captains actually use first, pure colour last
2. Enthusiastic consent at every tier boundary — equal-weight "finish here" and "keep going" CTAs
3. Privacy preserved — write-only contract intact; only a single integer (tier completion) crosses the wire to the player
4. No data loss for existing testers — migration backfills `wizard_progress_tier` from existing non-null answers

## Tier Map

| Tier | Theme | Why this position |
|------|-------|-------------------|
| 1 | **Team basics** — match types appetite, fill-in willingness, contact preference, last-minute window | What captains genuinely use to schedule. Finishable in ~30s. |
| 2 | **When & where** — days, times, transport, travel, home-court | Pure scheduling utility. |
| 3 | **How you play** — handedness, backhand, serve, net comfort, court side, signature shot | Pairing signal. |
| 4 | **Partners & pressure** — partner consistency, on-court vibe, partner picker, competitiveness, pressure response | Reflective; useful for matchups. |
| 5 | **Goals & anything to know** — season goal, improvement focus, what to know about my game, accessibility, weather, notes to captain | Self-knowledge surface. |
| 6 | **The fun stuff** — spirit animal, walkout song, celebration, post-match, one-liner, hero, pre-match ritual, how I got into tennis, years playing | Pure colour. Lots of people stop before here — by design. |

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies |
|----|-------|----------|------------|--------------|
| WI-108 | Schema: `wizard_progress_tier` column + backfill | High | S | None |
| WI-109 | Wizard handler: tier-aware GET, advance/finish POST, monotonic progress | High | L | WI-108 |
| WI-110 | Wizard template + localStorage drafts: stepped UI, progress strip, dual CTAs | High | L | WI-109 |
| WI-111 | E2E tests — wizard gating, progress monotonicity, write-only regression | High | M | WI-108, WI-109, WI-110 |

## Execution Strategy

### Phase 1: Foundations
- **WI-108** — Migration 027 + backfill + repo BumpWizardProgressTier method

### Phase 2: Core build (sequential, single-developer flow)
- **WI-109** — Tier-aware handler with shared Go-side tier definition
- **WI-110** — Template refactor + localStorage drafts (depends on the handler shape from WI-109)

### Phase 3: Lock it in
- **WI-111** — E2E regression suite

## Critical Path

```
WI-108 ─→ WI-109 ─→ WI-110 ─→ WI-111
```

## Key Design Decisions

1. **Server stores answers as before**. No data leaves the existing storage shape from Sprint 016. The user explicitly chose this over local-only storage because users are unlikely to switch devices and admins still need to read answers.

2. **Server returns only `wizard_progress_tier` to the player**. The write-only contract from Sprint 016 is preserved verbatim — no stored answer ever appears in a player-facing GET response. Completion state is not private; answers are.

3. **localStorage holds in-progress drafts**. Keyed per-token (`my-tennis-draft-{token}`), cleared on successful POST. Reuses the existing localStorage pattern in `availability.html` — no new infrastructure.

4. **Tier definitions live in Go, not the template**. A single `internal/players/my_tennis_tiers.go` source of truth feeds both the handler and (via reference) the backfill SQL. Adding/removing/reordering fields is a one-place change.

5. **Equal-weight CTAs at every tier boundary**. "Save & finish here" must be at least as visually prominent as "Save & keep going". This is the core consent gesture and cannot be undermined by a hierarchy that pushes users forward.

6. **Monotonic progress**. `wizard_progress_tier` only goes up. A user re-editing tier 2 after completing tier 4 stays at tier 4. Re-editing earlier tiers always opens them as blank forms — the privacy contract for fresh and returning users is identical.

7. **Backfill cascading, not exact**. If a Sprint 016 tester filled in tier-4 answers but skipped tier 2, they're considered "past tier 4" for gating purposes. Skipping tiers was always allowed; the backfill respects that.

8. **No "skip" button**. Saving with empty fields *is* skipping, given the merge semantics from Sprint 016 WI-097. Adding a third button would muddy the consent gesture.

## What Is NOT Changing

- Data model from Sprint 016 (`player_tennis_preferences`, `player_preferred_partners`, `captain_player_notes`) — untouched
- Write-only contract — preserved
- Initials-only rendering on the token URL — preserved
- Admin read surface (`/admin/league/players/{id}` partial) — untouched
- Captain-only notes scoping — untouched
- The fantasy token URL pattern and auth middleware — untouched

## Success Metrics

- [ ] Player on a fresh profile sees tier 1 only — five fields, finishable in under a minute
- [ ] At every tier boundary, the "Save & finish here" CTA is visually equal to or stronger than the advance CTA
- [ ] No stored answer value appears in any player-facing GET response (regression test passes)
- [ ] Existing Sprint 016 testers retain their progress after migration 027 runs
- [ ] Returning user with progress > 0 sees a warm welcome — never a guilt-trip about unfinished tiers
- [ ] Drafted-but-not-submitted fields survive a page reload via localStorage

## Work Items Tracking

### Completed
- WI-108 — Migration 027 + model + repo BumpWizardProgressTier (2026-04-25)
- WI-109 — Tier-aware handler + canonical tier definitions in Go (2026-04-25)
- WI-110 — Wizard template + localStorage drafts (2026-04-25)
- WI-111 — E2E wizard suite + tier-aware privacy sweep (2026-04-25)

### In Progress
- None

### Not Started
- None
