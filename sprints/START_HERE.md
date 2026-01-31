# 🚀 START HERE: Sprint Execution Quick Start

Welcome! This sprint system is designed for AI agents to execute work items automatically.

## The Key Concept

The scripts **analyze and plan** the work, but **I (Claude) spawn the actual agents** when you ask me to.

## How It Works

```
You → "Execute WI-001" → Me (Claude) → Task Tool → New Agent → Implements WI-001
```

## 3-Step Quick Start

### Step 1: See What's Available
```bash
python3 sprints/tools/execute-work-item.py sprint-001
```

This shows all 12 work items organized by execution batch.

### Step 2: Pick a Work Item
Start with WI-001 (player profile page) - it has no dependencies.

### Step 3: Ask Me to Spawn an Agent
Just say:

> **"Spawn an agent to implement WI-001 from sprint-001"**

I'll:
1. Read `sprints/sprint-001/WI-001.json`
2. Extract acceptance criteria, technical notes, testing requirements
3. Use the Task tool to spawn a general-purpose agent
4. The agent will implement the feature

## That's It!

You don't need to manually copy prompts or run complex commands. Just tell me which work item(s) to execute, and I'll spawn the agents.

## Common Commands

### Execute One Work Item
> "Spawn an agent to implement WI-001 from sprint-001"

### Execute Multiple in Parallel
> "Spawn agents in parallel for WI-001, WI-002, WI-004, and WI-008 from sprint-001"

### Execute a Full Batch
> "Execute batch 1 of sprint-001 in parallel"

### Check Progress
> "/tasks"
> Shows all running agents

## Next Steps

1. **Review the sprint**: Check `sprints/sprint-001/SUMMARY.md`
2. **Execute batch 1**: 7 parallelisable items ready to go
3. **Monitor progress**: Use `/tasks` to track agents
4. **Proceed to batch 2**: After batch 1 completes

## Full Documentation

- **EXECUTION_GUIDE.md** - Detailed execution instructions
- **sprint-001/SUMMARY.md** - Sprint overview and strategy
- **README.md** - Complete system documentation

## Ready?

Try this now:

> "Spawn an agent for WI-001 from sprint-001 and show me the task ID"

Then check its progress with `/tasks`!
