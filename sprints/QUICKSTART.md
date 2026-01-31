# Sprint System Quickstart

Get started with the sprint-based AI agent system in 5 minutes.

## What is this?

A structured system for managing development work items that AI agents can:
- Parse and understand
- Execute independently or in parallel
- Track dependencies automatically
- Follow detailed acceptance criteria

## Directory Structure

```
sprints/
├── sprint-001/              # Your first sprint
│   ├── sprint.json          # Sprint metadata
│   ├── WI-001.json         # Work item 1
│   ├── WI-002.json         # Work item 2
│   └── SUMMARY.md          # Human-readable summary
├── tools/
│   ├── run-sprint.sh       # Bash analysis tool
│   └── spawn-agents.py     # Python execution tool
└── README.md               # Full documentation
```

## Quick Commands

### 1. Analyze a Sprint
```bash
cd sprints
./tools/run-sprint.sh sprint-001
```

This shows:
- Sprint overview
- Work items with dependencies
- Execution plan (parallel vs sequential)

### 2. View Execution Plan
```bash
python3 tools/spawn-agents.py sprint-001 --dry-run
```

This displays:
- Dependency resolution
- Batch execution order
- AI agent prompts that would be generated

### 3. Execute a Single Work Item (Dry Run)
```bash
python3 tools/spawn-agents.py sprint-001 --item WI-001 --dry-run
```

This generates the prompt for WI-001 that you can:
- Copy to an AI agent manually
- Use as input for automated agent spawning

## Work Item Format at a Glance

Each `WI-XXX.json` contains:

```json
{
  "id": "WI-001",
  "title": "Short description",
  "description": "Detailed explanation",
  "parallelisable": true,           // Can run independently?
  "dependencies": ["WI-002"],       // Must wait for these
  "acceptance_criteria": [...],     // Clear completion criteria
  "technical_notes": {...},         // Implementation guidance
  "testing_requirements": [...]     // What to test
}
```

## Integration with AI Agents

### Manual Execution
1. Run: `python3 tools/spawn-agents.py sprint-001 --item WI-001 --dry-run`
2. Copy the generated prompt
3. Give it to an AI agent (Claude, GPT, etc.)
4. Agent implements according to acceptance criteria

### Automated Execution (Future)
The tools are designed to integrate with:
- Claude Code Task API
- GitHub Actions
- Custom CI/CD pipelines
- Agent orchestration systems

## Current Sprint: sprint-001

**Goal**: Complete core player availability features and implement push notifications

**12 Work Items** covering:
- Player profiles and availability preferences
- Push notification pipeline
- PWA installation flow
- Captain selection dashboard
- Fixture listing and WhatsApp sharing

**Parallelisable Items**: 8 can run simultaneously
**Sequential Items**: 4 must run after dependencies

## Next Steps

1. **Review the sprint**: Check `sprint-001/SUMMARY.md`
2. **Test the tools**: Run the commands above
3. **Start implementing**:
   - Pick a parallelisable item with no dependencies
   - Read the work item JSON
   - Implement according to acceptance criteria
   - Test according to testing requirements

## Common Workflows

### For AI Agents
```bash
# Get work item details
cat sprint-001/WI-001.json

# Generate implementation prompt
python3 tools/spawn-agents.py sprint-001 --item WI-001 --dry-run

# Follow acceptance criteria to implement
# Mark complete when all criteria met
```

### For Humans
```bash
# Review sprint
cat sprint-001/SUMMARY.md

# See execution order
./tools/run-sprint.sh sprint-001

# Track progress
# (Update work item JSON with status field or use external tracker)
```

## Tips

1. **Start with parallelisable items**: No waiting for dependencies
2. **Read the full work item**: Don't just read the title
3. **Check technical notes**: They have file paths and implementation hints
4. **Follow acceptance criteria**: They define "done"
5. **Run tests**: Testing requirements ensure quality

## Questions?

- Full documentation: [README.md](README.md)
- Sprint details: [sprint-001/SUMMARY.md](sprint-001/SUMMARY.md)
- Work item format: See any `WI-XXX.json` file

## Example: Running WI-001

```bash
# View the work item
cat sprint-001/WI-001.json | jq .

# Generate AI prompt
python3 tools/spawn-agents.py sprint-001 --item WI-001 --dry-run

# The output is a complete prompt you can give to an AI agent
# The agent will implement the player profile view page
# according to the acceptance criteria
```

Happy sprinting! 🚀
