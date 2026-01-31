# Sprint Index

This file provides a quick reference to all sprints and their status.

## Active Sprints

### Sprint 001: Player Experience & Notifications MVP
- **Status**: Not Started
- **Duration**: 2 weeks (Feb 3 - Feb 17, 2026)
- **Work Items**: 12
- **Goal**: Complete core player availability features and implement push notification system
- **Directory**: `sprint-001/`
- **Summary**: [sprint-001/SUMMARY.md](sprint-001/SUMMARY.md)

## Completed Sprints

None yet.

## Planned Sprints

### Sprint 002: Fixture Management & Venue Integration (Planned)
- **Focus**: Complete Phase 5 items
- **Estimated Items**: 8-10
- **Key Features**: Venue maps, fixture reminders, match result UI

### Sprint 003: Team Selection Optimization (Planned)
- **Focus**: Advanced captain tools
- **Estimated Items**: 6-8
- **Key Features**: Auto-suggestions, player statistics, historical performance

## Sprint Metrics

| Sprint | Work Items | Completed | In Progress | Blocked | Success Rate |
|--------|-----------|-----------|-------------|---------|--------------|
| 001 | 12 | 0 | 0 | 0 | - |

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
