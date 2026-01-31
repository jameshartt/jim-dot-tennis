# Sprint Management for AI Agents

This directory contains sprint-based work items structured for AI agent consumption and execution.

## Directory Structure

```
sprints/
├── README.md (this file)
├── sprint-001/
│   ├── sprint.json          # Sprint metadata and work item list
│   ├── WI-001.json          # Individual work item
│   ├── WI-002.json
│   └── ...
├── sprint-002/
│   └── ...
└── tools/
    └── run-sprint.sh        # Script to iterate and spawn agents
```

## Work Item Format

Each work item is a JSON file with the following structure:

```json
{
  "id": "WI-001",
  "title": "Brief description",
  "description": "Detailed description of the work",
  "type": "feature|bug|refactor|documentation",
  "priority": "high|medium|low",
  "complexity": "S|M|L",
  "parallelisable": true|false,
  "dependencies": ["WI-002", "WI-003"],
  "phase": "Reference to technical implementation plan phase",
  "acceptance_criteria": [
    "Criterion 1",
    "Criterion 2"
  ],
  "technical_notes": {
    "files_to_create": [],
    "files_to_modify": [],
    "routes_to_add": [],
    "schema_changes": {}
  },
  "testing_requirements": [],
  "context": {
    "user_story": "As a [role], I want [goal] so that [benefit]",
    "related_docs": []
  }
}
```

## Key Fields

### Metadata
- **id**: Unique identifier (format: WI-XXX)
- **title**: Short, actionable title
- **description**: Detailed explanation of what needs to be done
- **type**: Category of work
- **priority**: Business priority
- **complexity**: Size estimate (Small, Medium, Large)

### Execution
- **parallelisable**: Can this work be done independently of other items?
- **dependencies**: Array of work item IDs that must be completed first
- **phase**: Maps to technical implementation plan phases

### Requirements
- **acceptance_criteria**: Clear, testable criteria for completion
- **technical_notes**: Specific implementation guidance for AI agents
- **testing_requirements**: What needs to be tested

### Context
- **user_story**: User-facing value proposition
- **related_docs**: Links to relevant documentation

## How to Use

### For AI Agents

When executing a work item:

1. **Read the work item JSON** to understand requirements
2. **Check dependencies** - ensure all dependency work items are completed
3. **Review technical notes** for implementation guidance
4. **Review related docs** in the `context.related_docs` array
5. **Implement** according to acceptance criteria
6. **Test** according to testing requirements
7. **Mark complete** when all acceptance criteria are met

### For Humans

Review work items to:
- Understand sprint scope and goals
- Track progress on individual items
- Identify blockers and dependencies
- Prioritize work for agents or developers

## Execution Order

Work items can be executed in these patterns:

### 1. Sequential (Dependency Chain)
```
WI-002 → WI-003
```
WI-003 depends on WI-002, so must wait.

### 2. Parallel (Independent)
```
WI-001
WI-004  ← All can run simultaneously
WI-009
```

### 3. Batch (After Dependencies)
```
WI-005 → WI-006
      → WI-007  ← Both can run in parallel after WI-005
```

## Sprint Metadata

Each sprint directory contains a `sprint.json` file:

```json
{
  "sprint": {
    "id": "sprint-001",
    "name": "Sprint Name",
    "goal": "High-level sprint objective",
    "start_date": "YYYY-MM-DD",
    "end_date": "YYYY-MM-DD",
    "duration_weeks": 2,
    "focus_areas": ["Area 1", "Area 2"],
    "phases_addressed": ["Phase X"],
    "work_items": ["WI-001", "WI-002", ...]
  }
}
```

## Running Work Items with AI Agents

See `tools/run-sprint.sh` for automated execution scripts that:
- Parse work items
- Resolve dependencies
- Spawn parallel agents for independent work
- Track completion status

## Adding New Work Items

1. Create a new `WI-XXX.json` file in the sprint directory
2. Follow the JSON schema above
3. Add the ID to the `sprint.json` work_items array
4. Ensure dependencies reference existing work items

## Best Practices

### For Work Item Authors
- Keep complexity reasonable (break Large items into multiple Medium/Small items)
- Be specific in technical_notes (list exact files, methods, routes)
- Write testable acceptance criteria
- Mark parallelisable=true when possible to enable parallel execution

### For AI Agents
- Always read the full work item before starting
- Check related documentation for context
- Follow the technical notes precisely
- Ensure all acceptance criteria are met before marking complete
- Run all testing requirements

## Sprint Workflow

1. **Planning**: Create work items based on product requirements and technical gaps
2. **Dependency Resolution**: Organize items by dependencies
3. **Execution**: Run parallelisable items concurrently, sequential items in order
4. **Testing**: Validate acceptance criteria for each item
5. **Review**: Ensure sprint goal is achieved
6. **Retrospective**: Update process based on learnings
