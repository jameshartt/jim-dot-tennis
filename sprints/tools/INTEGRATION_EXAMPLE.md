# Integration Examples

This document shows how to integrate the sprint system with various AI agent platforms.

## Claude Code Task API Integration

### Using the Task Tool Programmatically

If you're using Claude Code and want to programmatically spawn agents for work items:

```python
#!/usr/bin/env python3
"""
Example: Spawn Claude Code agents for work items
This would be called from within a Claude Code session
"""

import json
from pathlib import Path


def spawn_claude_agent_for_work_item(work_item_path: Path):
    """
    Generate a prompt to use with Claude Code's Task tool.

    In practice, you would invoke the Task tool with this prompt.
    """
    with open(work_item_path) as f:
        work_item = json.load(f)

    # Build comprehensive prompt
    prompt = f"""
I need you to implement work item {work_item['id']}: {work_item['title']}

## Background
{work_item['description']}

## Acceptance Criteria
You must complete ALL of the following:
{chr(10).join(f"- {criteria}" for criteria in work_item['acceptance_criteria'])}

## Technical Implementation Notes
{_format_technical_notes(work_item['technical_notes'])}

## Testing Requirements
After implementation, verify:
{chr(10).join(f"- {req}" for req in work_item['testing_requirements'])}

## User Story
{work_item['context'].get('user_story', 'N/A')}

## Related Documentation
{chr(10).join(f"- {doc}" for doc in work_item['context'].get('related_docs', []))}

## Instructions
1. Read the related documentation to understand context
2. Implement the feature according to the technical notes
3. Ensure ALL acceptance criteria are met
4. Complete ALL testing requirements
5. Report completion with summary of changes

Begin implementation now.
"""

    return prompt


def _format_technical_notes(notes: dict) -> str:
    """Format technical notes for the prompt."""
    lines = []

    if notes.get('files_to_create'):
        lines.append("**Files to create:**")
        lines.extend(f"- {f}" for f in notes['files_to_create'])
        lines.append("")

    if notes.get('files_to_modify'):
        lines.append("**Files to modify:**")
        lines.extend(f"- {f}" for f in notes['files_to_modify'])
        lines.append("")

    if notes.get('routes_to_add'):
        lines.append("**Routes to add:**")
        lines.extend(f"- {r}" for r in notes['routes_to_add'])
        lines.append("")

    # Add other sections as needed
    for key, value in notes.items():
        if key not in ['files_to_create', 'files_to_modify', 'routes_to_add']:
            lines.append(f"**{key}:**")
            if isinstance(value, list):
                lines.extend(f"- {item}" for item in value)
            elif isinstance(value, dict):
                for k, v in value.items():
                    lines.append(f"- {k}: {v}")
            else:
                lines.append(f"{value}")
            lines.append("")

    return "\n".join(lines)


# Example usage:
if __name__ == "__main__":
    work_item_path = Path("../sprint-001/WI-001.json")
    prompt = spawn_claude_agent_for_work_item(work_item_path)
    print(prompt)

    # In a real integration, you would:
    # 1. Use Claude Code's Task tool API
    # 2. Pass this prompt to spawn a new agent
    # 3. Track the agent's progress
    # 4. Update work item status when complete
```

### Manual Task Spawning in Claude Code

From within a Claude Code session, you can manually spawn tasks:

```
Read sprint-001/WI-001.json and implement the work item according to the
acceptance criteria and technical notes provided.
```

Or use the Task tool:

```
I need you to use the Task tool to spawn a general-purpose agent with the
following prompt:

"Read /path/to/sprint-001/WI-001.json and implement the player profile view
page according to the acceptance criteria. Ensure all files listed in
technical_notes.files_to_create are created and files_to_modify are updated."
```

## Parallel Execution Pattern

### Using Shell Scripts with Background Jobs

```bash
#!/bin/bash
# Execute parallelisable work items concurrently

SPRINT_DIR="sprint-001"

# Get parallelisable items
PARALLEL_ITEMS=(WI-001 WI-002 WI-004 WI-008 WI-009 WI-010 WI-011)

# Spawn agent for each (background jobs)
for item in "${PARALLEL_ITEMS[@]}"; do
    echo "Spawning agent for $item..."

    # In practice, this would call your agent spawning command
    # python spawn_agent.py "$SPRINT_DIR/$item.json" &

    # For demo, just print
    echo "Agent spawned for $item"
done

# Wait for all background jobs
wait

echo "All parallel work items complete!"
```

### Using Python with Multiprocessing

```python
from concurrent.futures import ThreadPoolExecutor, as_completed
import json
from pathlib import Path


def execute_work_item(work_item_path: Path):
    """Execute a single work item (placeholder for actual agent call)."""
    with open(work_item_path) as f:
        work_item = json.load(f)

    print(f"Executing {work_item['id']}: {work_item['title']}")

    # In practice, spawn agent here
    # result = spawn_agent_api(work_item)

    return work_item['id'], "success"


def execute_parallel_batch(work_items: list, max_workers: int = 4):
    """Execute multiple work items in parallel."""
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # Submit all work items
        future_to_item = {
            executor.submit(execute_work_item, item): item
            for item in work_items
        }

        # Process as they complete
        for future in as_completed(future_to_item):
            item = future_to_item[future]
            try:
                item_id, status = future.result()
                print(f"✓ {item_id} completed: {status}")
            except Exception as e:
                print(f"✗ {item} failed: {e}")


# Example usage
if __name__ == "__main__":
    sprint_dir = Path("../sprint-001")

    # Parallelisable items from batch 1
    parallel_items = [
        sprint_dir / "WI-001.json",
        sprint_dir / "WI-002.json",
        sprint_dir / "WI-004.json",
        sprint_dir / "WI-008.json",
    ]

    execute_parallel_batch(parallel_items, max_workers=4)
```

## GitHub Actions Integration

```yaml
# .github/workflows/execute-sprint.yml
name: Execute Sprint Work Items

on:
  workflow_dispatch:
    inputs:
      sprint:
        description: 'Sprint directory (e.g., sprint-001)'
        required: true
      work_item:
        description: 'Specific work item (leave empty for all)'
        required: false

jobs:
  execute:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.10'

      - name: Execute work item
        run: |
          cd sprints
          if [ -n "${{ github.event.inputs.work_item }}" ]; then
            python3 tools/spawn-agents.py ${{ github.event.inputs.sprint }} \
              --item ${{ github.event.inputs.work_item }}
          else
            python3 tools/spawn-agents.py ${{ github.event.inputs.sprint }} \
              --parallel
          fi

      - name: Create PR with changes
        uses: peter-evans/create-pull-request@v5
        with:
          commit-message: "Implement ${{ github.event.inputs.work_item }}"
          title: "Sprint ${{ github.event.inputs.sprint }}: ${{ github.event.inputs.work_item }}"
          body: |
            Auto-generated implementation of work item.

            Please review the changes against acceptance criteria.
```

## API-Based Integration

If you're building an API to manage agents:

```python
from fastapi import FastAPI, BackgroundTasks
import json
from pathlib import Path
from typing import Optional

app = FastAPI()


@app.post("/api/sprint/{sprint_id}/execute")
async def execute_sprint(
    sprint_id: str,
    background_tasks: BackgroundTasks,
    work_item_id: Optional[str] = None,
    parallel: bool = False
):
    """Execute a sprint or specific work item."""
    sprint_dir = Path(f"sprints/{sprint_id}")

    if work_item_id:
        # Execute specific work item
        work_item_path = sprint_dir / f"{work_item_id}.json"
        background_tasks.add_task(execute_work_item, work_item_path)
        return {"status": "started", "work_item": work_item_id}
    else:
        # Execute full sprint
        with open(sprint_dir / "sprint.json") as f:
            sprint = json.load(f)

        work_items = sprint['sprint']['work_items']

        if parallel:
            # Spawn all parallelisable items
            for item_id in work_items:
                item_path = sprint_dir / f"{item_id}.json"
                with open(item_path) as f:
                    item = json.load(f)

                if item.get('parallelisable', False):
                    background_tasks.add_task(execute_work_item, item_path)

        return {"status": "started", "sprint": sprint_id, "items": len(work_items)}


@app.get("/api/sprint/{sprint_id}/status")
async def get_sprint_status(sprint_id: str):
    """Get status of all work items in a sprint."""
    # In practice, read from status tracking system
    return {"sprint": sprint_id, "status": "in_progress"}
```

## Status Tracking

### Simple JSON Status File

```json
{
  "sprint_id": "sprint-001",
  "status": "in_progress",
  "started_at": "2026-02-03T10:00:00Z",
  "work_items": {
    "WI-001": {
      "status": "completed",
      "started_at": "2026-02-03T10:00:00Z",
      "completed_at": "2026-02-03T14:30:00Z",
      "agent_id": "agent-abc123"
    },
    "WI-002": {
      "status": "in_progress",
      "started_at": "2026-02-03T10:00:00Z",
      "agent_id": "agent-def456"
    },
    "WI-003": {
      "status": "blocked",
      "blocked_by": ["WI-002"]
    }
  }
}
```

### Update Status Script

```python
import json
from datetime import datetime
from pathlib import Path


def update_work_item_status(sprint_id: str, work_item_id: str, status: str):
    """Update the status of a work item."""
    status_file = Path(f"sprints/{sprint_id}/status.json")

    # Load existing status
    if status_file.exists():
        with open(status_file) as f:
            data = json.load(f)
    else:
        data = {"sprint_id": sprint_id, "work_items": {}}

    # Update work item status
    if work_item_id not in data["work_items"]:
        data["work_items"][work_item_id] = {}

    data["work_items"][work_item_id]["status"] = status

    if status == "in_progress":
        data["work_items"][work_item_id]["started_at"] = datetime.now().isoformat()
    elif status == "completed":
        data["work_items"][work_item_id]["completed_at"] = datetime.now().isoformat()

    # Save status
    with open(status_file, 'w') as f:
        json.dump(data, f, indent=2)

    print(f"Updated {work_item_id} status to: {status}")


# Example usage
if __name__ == "__main__":
    update_work_item_status("sprint-001", "WI-001", "in_progress")
    # ... agent does work ...
    update_work_item_status("sprint-001", "WI-001", "completed")
```

## Best Practices

1. **Always validate work item JSON** before spawning agents
2. **Track agent IDs** to associate work with agents
3. **Implement timeout/retry logic** for failed agents
4. **Log all agent activities** for debugging
5. **Update status files** as work progresses
6. **Verify dependencies** before starting work items
7. **Run tests** after implementation before marking complete
