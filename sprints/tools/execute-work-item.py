#!/usr/bin/env python3
"""
Execute work items by spawning actual Claude Code agents.

This script reads work item JSON and spawns real agents using Claude Code's Task tool.
It's designed to be called FROM WITHIN a Claude Code session.

Usage:
    From Claude Code, say:
    "Run sprints/tools/execute-work-item.py sprint-001 WI-001"

    Or run the batch executor:
    "Run sprints/tools/execute-work-item.py sprint-001 --parallel"
"""

import json
import sys
from pathlib import Path
from typing import Dict, List, Set
from collections import defaultdict


def load_work_item(work_item_path: Path) -> dict:
    """Load a work item JSON file."""
    with open(work_item_path) as f:
        return json.load(f)


def build_agent_prompt(work_item: dict, sprint_dir: Path) -> str:
    """
    Build a comprehensive prompt for the agent to execute the work item.
    This prompt will be passed to Claude Code's Task tool.
    """

    # Format acceptance criteria
    criteria = "\n".join(f"{i+1}. {c}" for i, c in enumerate(work_item.get('acceptance_criteria', [])))

    # Format technical notes
    tech_notes = []
    for key, value in work_item.get('technical_notes', {}).items():
        tech_notes.append(f"\n**{key.replace('_', ' ').title()}:**")
        if isinstance(value, list):
            tech_notes.extend(f"  - {item}" for item in value)
        elif isinstance(value, dict):
            for k, v in value.items():
                tech_notes.append(f"  - {k}: {v}")
        else:
            tech_notes.append(f"  {value}")

    tech_notes_str = "\n".join(tech_notes) if tech_notes else "No specific technical notes provided."

    # Format testing requirements
    tests = "\n".join(f"- {t}" for t in work_item.get('testing_requirements', [])) or "No specific tests specified."

    # Format related docs
    related_docs = work_item.get('context', {}).get('related_docs', [])
    docs_str = "\n".join(f"- {doc}" for doc in related_docs) if related_docs else "None"

    # Build the prompt
    prompt = f"""# Work Item: {work_item['id']} - {work_item['title']}

## Description
{work_item.get('description', 'No description provided.')}

## User Story
{work_item.get('context', {}).get('user_story', 'No user story provided.')}

## Acceptance Criteria
These are the requirements you MUST complete:

{criteria}

## Technical Implementation Guidance
{tech_notes_str}

## Testing Requirements
After implementation, you must verify:

{tests}

## Related Documentation
Please review these files for context before starting:

{docs_str}

## Instructions for Implementation

1. **Read the related documentation** listed above to understand the existing codebase and architecture
2. **Review the technical notes** carefully - they specify exact files, routes, and implementation details
3. **Implement the feature** according to the acceptance criteria
4. **Follow the existing patterns** in the codebase (check CLAUDE.md for architecture guidance)
5. **Test your implementation** according to the testing requirements
6. **Verify all acceptance criteria** are met before completing

## Important Notes
- This is {work_item.get('priority', 'medium')} priority with {work_item.get('complexity', 'M')} complexity
- Phase: {work_item.get('phase', 'Not specified')}
- Use the Read tool to examine existing files before modifying them
- Follow the repository pattern and architectural conventions
- DO NOT create files that aren't specified in the technical notes
- Ensure database migrations are created if schema changes are needed

## Definition of Done
The work item is complete ONLY when:
1. All acceptance criteria are met
2. All testing requirements pass
3. Code follows existing patterns and conventions
4. No errors or warnings in the implementation

Begin implementation now.
"""

    return prompt


def format_work_item_for_display(work_item: dict) -> str:
    """Format work item for console display."""
    deps = work_item.get('dependencies', [])
    deps_str = f" (depends on: {', '.join(deps)})" if deps else ""

    return f"""
{'='*70}
{work_item['id']}: {work_item['title']}
{'='*70}
Priority: {work_item.get('priority', 'medium').upper()}
Complexity: {work_item.get('complexity', 'M')}
Parallelisable: {'Yes' if work_item.get('parallelisable', False) else 'No'}{deps_str}
Phase: {work_item.get('phase', 'Not specified')}
{'='*70}
"""


def resolve_execution_order(work_items: Dict[str, dict]) -> List[List[str]]:
    """
    Resolve dependencies and return work items grouped by execution batch.
    Each batch can be executed in parallel.
    """
    dependents = defaultdict(list)
    dependencies_remaining = {}

    for item_id, item in work_items.items():
        deps = item.get('dependencies', [])
        dependencies_remaining[item_id] = len(deps)
        for dep in deps:
            dependents[dep].append(item_id)

    batches = []
    completed = set()

    while len(completed) < len(work_items):
        ready = [
            item_id
            for item_id, count in dependencies_remaining.items()
            if count == 0 and item_id not in completed
        ]

        if not ready:
            remaining = set(work_items.keys()) - completed
            raise RuntimeError(f"Circular or missing dependencies: {remaining}")

        batches.append(ready)

        for item_id in ready:
            completed.add(item_id)
            for dependent in dependents[item_id]:
                dependencies_remaining[dependent] -= 1

    return batches


def main():
    if len(sys.argv) < 2:
        print("Usage: execute-work-item.py <sprint-dir> [work-item-id] [--parallel]")
        print("\nExamples:")
        print("  execute-work-item.py sprint-001 WI-001")
        print("  execute-work-item.py sprint-001 --parallel")
        sys.exit(1)

    script_dir = Path(__file__).parent
    sprints_dir = script_dir.parent
    sprint_dir = sprints_dir / sys.argv[1]

    if not sprint_dir.exists():
        print(f"Error: Sprint directory not found: {sprint_dir}")
        sys.exit(1)

    # Load sprint metadata
    sprint_file = sprint_dir / "sprint.json"
    with open(sprint_file) as f:
        sprint_data = json.load(f)

    sprint = sprint_data['sprint']
    print(f"\n{'='*70}")
    print(f"Sprint: {sprint['name']}")
    print(f"Goal: {sprint['goal']}")
    print(f"{'='*70}\n")

    # Load all work items
    work_items = {}
    for item_id in sprint['work_items']:
        item_file = sprint_dir / f"{item_id}.json"
        if item_file.exists():
            work_items[item_id] = load_work_item(item_file)

    # Check if specific work item requested
    if len(sys.argv) >= 3 and not sys.argv[2].startswith('--'):
        work_item_id = sys.argv[2]

        if work_item_id not in work_items:
            print(f"Error: Work item {work_item_id} not found")
            sys.exit(1)

        work_item = work_items[work_item_id]
        print(format_work_item_for_display(work_item))

        # Check dependencies
        deps = work_item.get('dependencies', [])
        if deps:
            print(f"\n⚠️  WARNING: This work item depends on: {', '.join(deps)}")
            print("Make sure these are completed first!\n")

        # Build and print the prompt
        prompt = build_agent_prompt(work_item, sprint_dir)

        print("\n" + "="*70)
        print("AGENT PROMPT")
        print("="*70)
        print("\nCopy the prompt below and use Claude Code's Task tool to spawn an agent:\n")
        print("-"*70)
        print(prompt)
        print("-"*70)

        print("\n" + "="*70)
        print("HOW TO SPAWN THE AGENT")
        print("="*70)
        print("""
From within Claude Code, tell it:

"Use the Task tool with subagent_type='general-purpose' and this prompt:

[paste the prompt above]
"

Or simply say:

"Spawn a general-purpose agent to implement work item WI-XXX according to
the prompt shown above."
        """)

    else:
        # Show execution plan
        batches = resolve_execution_order(work_items)

        print(f"Total work items: {len(work_items)}\n")
        print("Execution Plan (by batch):")
        print("="*70 + "\n")

        for i, batch in enumerate(batches, 1):
            print(f"Batch {i} - Can execute in parallel ({len(batch)} items):")
            for item_id in batch:
                item = work_items[item_id]
                deps = item.get('dependencies', [])
                deps_str = f" [depends on: {', '.join(deps)}]" if deps else ""
                print(f"  ✓ {item_id}: {item['title']}{deps_str}")
            print()

        if '--parallel' in sys.argv or '--batch' in sys.argv:
            print("\n" + "="*70)
            print("READY TO SPAWN AGENTS")
            print("="*70)
            print("\nTo execute this sprint, spawn agents for each batch sequentially.")
            print("Within each batch, you can spawn multiple agents in parallel.\n")

            for i, batch in enumerate(batches, 1):
                print(f"\nBatch {i} - Execute these in parallel:")
                for item_id in batch:
                    print(f"\n  python3 sprints/tools/execute-work-item.py {sys.argv[1]} {item_id}")

            print("\n\nOr, to spawn agents automatically, tell Claude Code:")
            print("\n  'For each work item in batch 1 of sprint-001, spawn a general-purpose")
            print("   agent to implement it according to the work item JSON.'")


if __name__ == "__main__":
    main()
