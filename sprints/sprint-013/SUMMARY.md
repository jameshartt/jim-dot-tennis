# Sprint 013: Attribution, Licensing & Open Source Readiness

## Goal
Establish proper open-source licensing, clear attribution for all contributors, GitHub repository configuration, and documentation that positions jim.tennis as an adaptable parks league management system — with a practical on-ramp for other clubs.

## Problem Statement
jim.tennis has no license file, no contributor guidelines, no GitHub repository configuration, and no in-app attribution. The README has a placeholder where the license should be. There is no public acknowledgement that James Hartt built the system, no credit to CourtHive and Charles Allen for tournament/Cup management, and no path for someone at another parks league club to evaluate whether this system could work for them. This sprint fixes all of that.

## Design Principles

- **Honest, not promotional.** The parks league is a small community with complicated relationships. Everything written in this sprint should read as factual and understated. If someone from another club reads the README, the reaction should be curiosity, not suspicion.
- **Credit where it's due.** James Hartt built jim.tennis. CourtHive (Charles Allen) built the tournament/Cup management. The BHPLTA provides the league infrastructure. All three should be clearly and prominently credited.
- **Realistic about limitations.** The system was built for St Ann's. The club adaptation guide should be honest about what's involved in deploying for another club, including the known pain points (apostrophe handling, hardcoded ClubID logic).

## Work Items

| ID | Title | Priority | Complexity | Dependencies | Parallelisable |
|----|-------|----------|------------|--------------|----------------|
| WI-071 | Add MIT License to repository | Critical | S | None | Yes |
| WI-072 | GitHub repository configuration and .github directory | High | S | None | Yes |
| WI-073 | README overhaul: attribution, GitHub links, and project positioning | Critical | M | WI-071 | No |
| WI-074 | CONTRIBUTING.md and club adaptation guide | High | L | WI-071 | No |
| WI-075 | In-app credits and about page | Critical | M | WI-071 | No |
| WI-076 | Source file headers and dependency acknowledgements | Medium | S | WI-071 | Yes |

## Execution Plan

### Phase 1 — Foundation (parallel)
- **WI-071**: MIT License file — must land first as everything else references it
- **WI-072**: .github/ directory with issue templates, PR template, SECURITY.md — no dependencies, can be done in parallel with WI-071

### Phase 2 — Documentation (after WI-071, can be parallel with each other)
- **WI-073**: README overhaul — needs license in place before referencing it
- **WI-074**: CONTRIBUTING.md with developer guide and club adaptation section

### Phase 3 — Application (after WI-071, parallel with Phase 2)
- **WI-075**: /about page with developer profile, CourtHive attribution, GitHub link

### Phase 4 — Housekeeping (after WI-071, parallel with Phases 2-3)
- **WI-076**: Copyright headers on source files, ACKNOWLEDGEMENTS.md for dependencies

## Key Decisions

- **MIT License** chosen for maximum permissiveness — no barriers to other clubs using or modifying the code
- **Club enquiry issue template** provides a frictionless way for interested clubs to make contact without it feeling like a sales funnel
- **The apostrophe problem is documented, not solved** — it's the biggest barrier to multi-club support but is out of scope for this sprint. The CONTRIBUTING.md will be honest about it.
- **CourtHive attribution is prominent, not footnoted** — Charles's work on Cup management deserves clear, specific credit both in the README and on the in-app about page
- **No separate "marketing" page or landing site** — the README and about page do the job. Anything more would feel like overreach in the context of parks league politics.

## Tone Guidance

This sprint produces a lot of prose. Every piece of text should be reviewed for tone:

- ✓ "jim.tennis is a league management system built for St Ann's Tennis Club"
- ✗ "jim.tennis is a powerful, feature-rich platform that revolutionises league management"
- ✓ "If you manage a parks league club and this looks useful, the adaptation guide covers what's involved"
- ✗ "Any club can easily deploy their own instance in minutes!"
- ✓ "Tournament and Cup management is developed by CourtHive, with the primary development effort by Charles Allen"
- ✗ "We leverage CourtHive's industry-leading tournament platform"
