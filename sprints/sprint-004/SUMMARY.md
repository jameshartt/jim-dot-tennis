# Sprint 004: Spring Clean: Go Tooling, Dead Code, Linting & Environment Update

## Overview

**Goal**: Use Go's static analysis, dead code detection, linting, and formatting tools to clean up the codebase, update the Go version and Docker images to current stable, and establish ongoing code quality tooling

**Duration**: Short sprint (Feb 1-3, 2026)

**Status**: Completed (closed 2026-02-01)

## Focus Areas

1. Go and Docker version updates
2. Go tooling infrastructure via Docker (all Go operations run in containers)
3. Dead code detection and removal
4. Static analysis and linting fixes
5. Code formatting standardisation
6. Module dependency cleanup

## Work Items Summary

| ID | Title | Priority | Complexity | Dependencies | Status |
|----|-------|----------|------------|--------------|--------|
| WI-022 | Add Go tooling Makefile targets via Docker | High | S | - | Completed |
| WI-023 | Dead code detection and removal | High | M | WI-022 | Completed |
| WI-024 | Static analysis and linting fixes | High | L | WI-022, WI-023 | Completed |
| WI-025 | Code formatting standardisation (gofmt/goimports) | Medium | S | WI-022 | Completed |
| WI-026 | Module dependency cleanup (go mod tidy) | Medium | S | WI-022, WI-023 | Completed |
| WI-027 | Update Go version and Docker base images | High | S | - | Completed |

## Work Item Details

### WI-022: Add Go tooling Makefile targets via Docker

Go is not installed on the host machine. All Go operations run inside Docker. This adds Makefile targets for running analysis tools inside containers:

- `make vet` - run go vet
- `make fmt` / `make fmt-fix` - check/fix formatting
- `make imports` / `make imports-fix` - check/fix import organisation
- `make lint` - run golangci-lint or staticcheck
- `make deadcode` - run dead code analysis
- `make tidy` - run go mod tidy
- `make check` - run all read-only checks in sequence

All targets use the same golang:1.25-alpine base as the builder stage.

**Files affected**: `Makefile`

### WI-023: Dead code detection and removal

Run Go's `deadcode` tool and `go vet` to find and remove unused functions, methods, types, constants, and variables. Particularly valuable after the service.go refactor in sprint-003. The `getCurrentWeekDateRange` method in service.go has already been identified as dead code.

**Files affected**: Multiple Go source files across `internal/`

### WI-024: Static analysis and linting fixes

Run `golangci-lint` against the codebase and fix all reported issues: unchecked errors, shadowed variables, inefficient patterns, potential nil dereferences, etc. Create a `.golangci.yml` configuration to establish ongoing linting standards.

**Files affected**: Multiple Go source files, new `.golangci.yml`

### WI-025: Code formatting standardisation

Run `gofmt` and `goimports` across the entire codebase. Zero-risk operation that only changes whitespace and import ordering. Ensures all files follow canonical Go formatting and imports are properly grouped (stdlib / external / internal).

**Files affected**: Potentially all `.go` files (whitespace/import ordering only)

### WI-026: Module dependency cleanup

Run `go mod tidy` to remove unused dependencies and ensure all required dependencies are explicitly listed. Should run after dead code removal (WI-023) since removing code may eliminate imports.

**Files affected**: `go.mod`, `go.sum`

### WI-027: Update Go version and Docker base images

The project uses Go 1.24.1 but Go 1.25 is the current stable release (1.25.6 available). Update the Dockerfile builder stage, go.mod directive, and Alpine runtime image to current stable versions. Go 1.26 is expected in February 2026 - if it releases and is stable during this sprint, consider targeting that instead.

**Files affected**: `Dockerfile`, `go.mod`

## Execution Order

1. **WI-027** (Go version update) and **WI-022** (tooling targets) can run in parallel as the first step
2. **WI-025** (formatting) can run as soon as WI-022 is done
3. **WI-023** (dead code) runs after WI-022
4. **WI-026** (mod tidy) runs after WI-023
5. **WI-024** (linting) runs last, after dead code is removed and dependencies are clean

## Technical Impact

### New Files
- `.golangci.yml` - linter configuration
- Possible `Dockerfile.tools` or tooling docker-compose service

### Modified Files
- `Makefile` - new tooling targets
- `Dockerfile` - Go version bump
- `go.mod` / `go.sum` - Go version and dependency updates
- Multiple `.go` files - formatting, dead code removal, lint fixes

### No Functional Changes
This is a pure code quality and infrastructure sprint. No features are added, no behaviour changes. All routes, pages, and interactions remain identical.

## Outcomes

### WI-027: Go version updated from 1.24.1 to 1.25
- Dockerfile builder stage: `golang:1.24.1-alpine` -> `golang:1.25-alpine`
- go.mod directive updated, `toolchain` directive removed (handled by Go 1.25)

### WI-022: 10 Makefile targets added
- `make vet`, `make fmt`, `make fmt-fix`, `make imports`, `make imports-fix`, `make lint`, `make deadcode`, `make tidy`, `make check`
- All run inside Docker containers — no local Go installation required
- `make check` runs vet + fmt + lint + deadcode in sequence

### WI-025: 14 files reformatted
- gofmt and goimports applied across the entire codebase
- Import groups standardised: stdlib / external / internal

### WI-023: 18 dead functions and 2 dead types removed
- Removed from: service.go, auth/service.go, auth/middleware.go, models/availability.go, players/service.go, players/availability.go, services/matchcard_parser.go, services/nonce_extractor.go
- 10 admin service domain files cleaned (service_fantasy.go, service_fixture_players.go, service_fixtures.go, service_matchups.go, service_players.go, service_seasons.go, service_teams.go, service_users_sessions.go, selection_overview.go, team_eligibility.go)
- deadcode tool now reports zero findings

### WI-026: Dependencies confirmed clean
- `go mod tidy` produced no changes beyond the Go version update
- All dependencies are explicitly listed and used

### WI-024: Linting established with zero findings
- `.golangci.yml` created with 11 enabled linters (errcheck, govet, staticcheck, ineffassign, unused, gosimple, typecheck, misspell, goconst, gofmt, goimports)
- 3 staticcheck issues fixed (unused append result, regexp in loop, empty branch)
- errcheck exclusions configured for standard patterns (tx.Rollback, json.Encode, w.Write)
- gosimple fix (unnecessary fmt.Sprintf)
- `make lint` now runs clean
