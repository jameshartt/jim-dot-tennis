# Execution Guide: How to Actually Spawn Agents

This guide shows you how to **actually execute** work items by spawning real AI agents.

## Method 1: Direct Command (Recommended)

From within this Claude Code session, you can spawn agents directly by asking me to do it:

### Execute a Single Work Item

```
Spawn a general-purpose agent to implement work item WI-001 from sprint-001.
Read the work item JSON at sprints/sprint-001/WI-001.json and create a detailed
prompt with all acceptance criteria, technical notes, and testing requirements.
```

### Execute Multiple Work Items in Parallel

```
Spawn general-purpose agents in parallel for these work items from sprint-001:
- WI-001
- WI-002
- WI-004
- WI-008

For each one, read the JSON file and spawn with full acceptance criteria.
```

## Method 2: Using the Helper Script

The `execute-work-item.py` script generates the prompts for you:

### Step 1: Generate the Prompt

```bash
python3 sprints/tools/execute-work-item.py sprint-001 WI-001
```

This outputs a complete prompt that you can copy.

### Step 2: Ask Me to Spawn with That Prompt

Then tell me:

```
Use the Task tool to spawn a general-purpose agent with the prompt that was
just printed above.
```

## Method 3: Automated Batch Execution

Tell me to execute a whole batch:

```
I want to execute sprint-001 batch 1 in parallel. Please:

1. Read sprints/sprint-001/sprint.json to get all work items
2. Resolve dependencies to find batch 1 items (no dependencies)
3. For each item in batch 1, spawn a general-purpose agent with a prompt
   that includes:
   - The work item description and title
   - All acceptance criteria
   - All technical notes
   - All testing requirements
   - Related documentation
4. Spawn all batch 1 agents in parallel (single message with multiple Task calls)
```

## Example: Spawning WI-001 Right Now

Here's exactly what to say:

```
Read sprints/sprint-001/WI-001.json and use the Task tool to spawn a
general-purpose agent with this task:

"Implement the player profile view page according to work item WI-001.

Read sprints/sprint-001/WI-001.json for complete requirements.

Your implementation must:
1. Create templates/players/profile.html
2. Create internal/players/profile.go
3. Modify internal/players/handler.go to add routes
4. Display player name, club, teams, upcoming fixtures, and availability
5. Be mobile-responsive
6. Use existing auth middleware

Test by:
- Verifying profile loads for authenticated players
- Testing with players in multiple teams
- Checking mobile responsiveness

Follow the architectural patterns in CLAUDE.md and use the repository pattern."

Use subagent_type='general-purpose' and model='haiku' for this task.
```

## Parallel Execution Example

To run batch 1 (7 parallelisable items) simultaneously:

```
Spawn 7 general-purpose agents in parallel for sprint-001 batch 1.
Send a single message with 7 Task tool calls, one for each of these work items:

- WI-001: Player profile view page
- WI-002: General availability preferences
- WI-004: Push notification subscription flow
- WI-008: Captain selection overview dashboard
- WI-009: PWA installation prompt flow
- WI-010: Offline availability update
- WI-011: Fixture details and listing page

For each agent, read the corresponding JSON file (e.g., sprints/sprint-001/WI-001.json)
and create a complete prompt with acceptance criteria, technical notes, and testing
requirements.

Use model='haiku' for faster execution on these Medium complexity items.
```

## Monitoring Agent Progress

After spawning agents:

1. **Check task list**: Use `/tasks` command to see running agents
2. **View agent output**: Use TaskOutput tool to check progress
3. **Review when complete**: Each agent will report completion with a summary

## Best Practices

1. **Start with one**: Test with a single work item first (WI-001 is a good start)
2. **Check dependencies**: Don't run WI-003 before WI-002, WI-005 before WI-004, etc.
3. **Use parallel execution**: Batch 1 has 7 independent items - run them together!
4. **Choose right model**:
   - Use `haiku` for S/M complexity items (faster, cheaper)
   - Use `sonnet` for L complexity items (more capable)
5. **Monitor progress**: Check `/tasks` periodically to see status

## Quick Reference: Work Item Batches

### Batch 1 (No Dependencies - Run in Parallel)
- WI-001, WI-002, WI-004, WI-008, WI-009, WI-010, WI-011

### Batch 2 (After Batch 1 Completes)
- WI-003 (after WI-002)
- WI-005 (after WI-004)
- WI-012 (after WI-011)

### Batch 3 (After Batch 2 Completes)
- WI-006 (after WI-005)
- WI-007 (after WI-005)

## Troubleshooting

**"Agent failed" or "Couldn't complete"**
- Check the agent's output for errors
- Verify acceptance criteria weren't too strict
- May need to break work item into smaller pieces

**"Missing dependencies"**
- Check the work item's `dependencies` array
- Ensure prerequisite work items are completed first

**"Agent is stuck"**
- Use TaskOutput to check what it's doing
- May need to provide additional context or guidance
- Consider stopping and restarting with refined prompt

## Ready to Start?

Try this right now:

```
Spawn a general-purpose agent with model haiku to implement WI-001 from sprint-001.
Read sprints/sprint-001/WI-001.json and include all acceptance criteria and
technical notes in the prompt.
```

Then check progress with `/tasks` and see the agent implement the player profile page!
