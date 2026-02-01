# Sprint Index

This file provides a quick reference to all sprints and their status.

## Active Sprints

None currently active.

## Completed Sprints

### Sprint 003: Admin Polish: Dashboard, Divisions, Users & Service Refactor
- **Status**: Completed (closed 2026-02-01)
- **Duration**: Short sprint (Feb 1, 2026)
- **Work Items**: 5 (WI-017 to WI-021) - all completed
- **Goal**: Reorganise dashboard, add division editing, implement user/session management, refactor service.go
- **Directory**: `sprint-003/`
- **Summary**: [sprint-003/SUMMARY.md](sprint-003/SUMMARY.md)

### Sprint 002: Venue Infrastructure & Season Filtering
- **Status**: Completed (closed 2026-02-01)
- **Duration**: 2 days (Jan 31 - Feb 1, 2026)
- **Work Items**: 4 (WI-013 to WI-016)
- **Goal**: Add venue/club infrastructure, fix season filtering, improve player fixture UX
- **Directory**: `sprint-002/`
- **Summary**: [sprint-002/SUMMARY.md](sprint-002/SUMMARY.md)

### Sprint 001: Player Experience & Notifications MVP
- **Status**: Completed (closed 2026-02-01)
- **Duration**: 2 weeks (Feb 3 - Feb 17, 2026)
- **Work Items**: 12 (5 completed, 1 deferred, 6 carried to sprint-pwa)
- **Goal**: Complete core player availability features and implement push notification system
- **Directory**: `sprint-001/`
- **Summary**: [sprint-001/SUMMARY.md](sprint-001/SUMMARY.md)

## Planned Sprints

### Sprint PWA: Push Notifications & PWA Enhancement
- **Focus**: Push notification pipeline and PWA capabilities
- **Items**: 6 (carried from sprint-001)
- **Key Features**: Push subscriptions, notification sending, PWA install prompt, offline sync
- **Directory**: `sprint-pwa/`
- **Summary**: [sprint-pwa/SUMMARY.md](sprint-pwa/SUMMARY.md)

### Sprint 004: Team Selection Optimization (Planned)
- **Focus**: Advanced captain tools
- **Estimated Items**: 6-8
- **Key Features**: Auto-suggestions, player statistics, historical performance

## Sprint Metrics

| Sprint | Work Items | Completed | In Progress | Blocked | Success Rate |
|--------|-----------|-----------|-------------|---------|--------------|
| 003 | 5 | 5 | 0 | 0 | 100% |
| 002 | 4 | 4 | 0 | 0 | 100% |
| 001 | 12 | 5 | 0 | 0 | 42% (6 carried to sprint-pwa) |

## How to Use This Index

1. **Review active sprints** to understand current focus
2. **Check sprint summaries** for detailed breakdown
3. **Use sprint tools** in `tools/` directory to execute work items
4. **Update this index** as sprints progress and complete

## Sprint Execution Commands

### View Sprint Details
```bash
# Using bash script
./tools/run-sprint.sh sprint-001

# Using Python script
python3 ./tools/spawn-agents.py sprint-001 --dry-run
```

### Execute Work Items
```bash
# Execute specific work item
python3 ./tools/spawn-agents.py sprint-001 --item WI-001 --dry-run

# Execute full sprint (dry run)
python3 ./tools/spawn-agents.py sprint-001 --dry-run

# Execute full sprint with parallel agents
python3 ./tools/spawn-agents.py sprint-001 --parallel
```

## Work Item Status Tracking

Work items can have the following statuses:
- **Not Started**: Default state
- **In Progress**: Agent/developer actively working
- **Blocked**: Waiting on dependencies
- **In Review**: Implementation complete, awaiting review
- **Completed**: All acceptance criteria met

To track status, you can:
1. Add a `status` field to work item JSON files
2. Use a separate tracking file (e.g., `sprint-001/status.json`)
3. Use project management tools integrated with the work items

## Adding New Sprints

1. Create a new directory: `sprint-XXX/`
2. Create `sprint.json` with metadata
3. Create work item files: `WI-XXX.json`
4. Create `SUMMARY.md` for overview
5. Update this INDEX.md file

See [README.md](README.md) for detailed format and structure.
