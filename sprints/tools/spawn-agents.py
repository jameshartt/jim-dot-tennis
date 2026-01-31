#!/usr/bin/env python3
"""
AI Agent Spawner for Sprint Work Items

This script reads work items from a sprint and spawns AI agents to execute them.
It handles dependency resolution and parallel execution of independent work items.

Usage:
    python spawn-agents.py sprint-001 [--dry-run] [--parallel]
"""

import json
import os
import sys
import argparse
from pathlib import Path
from typing import List, Dict, Set
from collections import defaultdict


class WorkItem:
    """Represents a work item with metadata and dependencies."""

    def __init__(self, item_id: str, data: dict):
        self.id = item_id
        self.title = data.get("title", "")
        self.description = data.get("description", "")
        self.priority = data.get("priority", "medium")
        self.complexity = data.get("complexity", "M")
        self.parallelisable = data.get("parallelisable", False)
        self.dependencies = data.get("dependencies", [])
        self.phase = data.get("phase", "")
        self.acceptance_criteria = data.get("acceptance_criteria", [])
        self.technical_notes = data.get("technical_notes", {})
        self.testing_requirements = data.get("testing_requirements", [])
        self.context = data.get("context", {})

    def __repr__(self):
        return f"WorkItem({self.id}: {self.title})"

    def to_prompt(self) -> str:
        """Generate a prompt for an AI agent to execute this work item."""
        prompt = f"""Implement work item {self.id}: {self.title}

## Description
{self.description}

## Acceptance Criteria
{self._format_list(self.acceptance_criteria)}

## Technical Notes
{self._format_technical_notes()}

## Testing Requirements
{self._format_list(self.testing_requirements)}

## Context
{self._format_context()}

Please implement this work item according to the acceptance criteria and technical notes.
Ensure all testing requirements are met before marking this work item as complete.
"""
        return prompt

    def _format_list(self, items: List[str]) -> str:
        if not items:
            return "None specified"
        return "\n".join(f"- {item}" for item in items)

    def _format_technical_notes(self) -> str:
        if not self.technical_notes:
            return "No specific technical notes"

        lines = []
        for key, value in self.technical_notes.items():
            if isinstance(value, list):
                lines.append(f"\n**{key}:**")
                lines.extend(f"  - {item}" for item in value)
            elif isinstance(value, dict):
                lines.append(f"\n**{key}:**")
                for k, v in value.items():
                    lines.append(f"  - {k}: {v}")
            else:
                lines.append(f"**{key}:** {value}")

        return "\n".join(lines)

    def _format_context(self) -> str:
        lines = []
        if self.context.get("user_story"):
            lines.append(f"**User Story:** {self.context['user_story']}")

        if self.context.get("related_docs"):
            lines.append("\n**Related Documentation:**")
            lines.extend(f"  - {doc}" for doc in self.context["related_docs"])

        return "\n".join(lines) if lines else "No additional context"


class SprintExecutor:
    """Manages execution of work items in a sprint."""

    def __init__(self, sprint_dir: Path):
        self.sprint_dir = sprint_dir
        self.work_items: Dict[str, WorkItem] = {}
        self.sprint_metadata = {}

    def load_sprint(self):
        """Load sprint metadata and work items."""
        # Load sprint.json
        sprint_file = self.sprint_dir / "sprint.json"
        if not sprint_file.exists():
            raise FileNotFoundError(f"Sprint file not found: {sprint_file}")

        with open(sprint_file) as f:
            data = json.load(f)
            self.sprint_metadata = data.get("sprint", {})

        # Load work items
        for work_item_id in self.sprint_metadata.get("work_items", []):
            work_item_file = self.sprint_dir / f"{work_item_id}.json"
            if work_item_file.exists():
                with open(work_item_file) as f:
                    work_item_data = json.load(f)
                    self.work_items[work_item_id] = WorkItem(work_item_id, work_item_data)

    def resolve_dependencies(self) -> List[List[str]]:
        """
        Resolve dependencies and return work items grouped by execution order.
        Returns a list of lists, where each inner list can be executed in parallel.
        """
        # Build dependency graph
        dependents = defaultdict(list)  # who depends on me
        dependencies_remaining = {}  # how many dependencies do I have left

        for item_id, item in self.work_items.items():
            dependencies_remaining[item_id] = len(item.dependencies)
            for dep in item.dependencies:
                dependents[dep].append(item_id)

        # Topological sort with parallel batching
        execution_batches = []
        completed = set()

        while len(completed) < len(self.work_items):
            # Find all items with no remaining dependencies
            ready = [
                item_id
                for item_id, count in dependencies_remaining.items()
                if count == 0 and item_id not in completed
            ]

            if not ready:
                # Circular dependency or missing dependency
                remaining = set(self.work_items.keys()) - completed
                raise RuntimeError(f"Circular or missing dependencies detected: {remaining}")

            execution_batches.append(ready)

            # Mark as completed and update dependents
            for item_id in ready:
                completed.add(item_id)
                for dependent in dependents[item_id]:
                    dependencies_remaining[dependent] -= 1

        return execution_batches

    def categorize_items(self) -> Dict[str, List[WorkItem]]:
        """Categorize work items by various attributes."""
        categories = {
            "high_priority": [],
            "parallelisable": [],
            "sequential": [],
            "no_dependencies": [],
        }

        for item in self.work_items.values():
            if item.priority == "high":
                categories["high_priority"].append(item)
            if item.parallelisable:
                categories["parallelisable"].append(item)
            else:
                categories["sequential"].append(item)
            if not item.dependencies:
                categories["no_dependencies"].append(item)

        return categories

    def print_summary(self):
        """Print a summary of the sprint."""
        print(f"\n{'='*60}")
        print(f"Sprint: {self.sprint_metadata.get('name', 'Unknown')}")
        print(f"Goal: {self.sprint_metadata.get('goal', 'N/A')}")
        print(f"{'='*60}\n")

        print(f"Total Work Items: {len(self.work_items)}")

        categories = self.categorize_items()
        print(f"  - High Priority: {len(categories['high_priority'])}")
        print(f"  - Parallelisable: {len(categories['parallelisable'])}")
        print(f"  - No Dependencies: {len(categories['no_dependencies'])}")

        print("\n" + "="*60)
        print("Execution Plan (by batch)")
        print("="*60 + "\n")

        batches = self.resolve_dependencies()
        for i, batch in enumerate(batches, 1):
            print(f"Batch {i} (parallel execution possible):")
            for item_id in batch:
                item = self.work_items[item_id]
                print(f"  - {item_id}: {item.title} [{item.priority}] [{item.complexity}]")
            print()

    def spawn_agent(self, work_item_id: str, dry_run: bool = False):
        """
        Spawn an AI agent to execute a work item.

        In a real implementation, this would integrate with Claude Code's Task API.
        For now, it prints the prompt that would be sent.
        """
        item = self.work_items[work_item_id]

        print(f"\n{'─'*60}")
        print(f"Spawning agent for: {work_item_id}")
        print(f"{'─'*60}")

        if dry_run:
            print("\n[DRY RUN] Prompt that would be sent:\n")
            print(item.to_prompt())
        else:
            # In a real implementation, you would call something like:
            # subprocess.run(['claude-code', 'task', 'execute', '--prompt', item.to_prompt()])
            # Or use an API to spawn the task
            print(f"[PLACEHOLDER] Would spawn agent with prompt for {work_item_id}")
            print(f"Priority: {item.priority}, Complexity: {item.complexity}")

    def execute_sprint(self, parallel: bool = False, dry_run: bool = False):
        """Execute the sprint by spawning agents for work items."""
        batches = self.resolve_dependencies()

        for i, batch in enumerate(batches, 1):
            print(f"\n{'='*60}")
            print(f"Executing Batch {i}")
            print(f"{'='*60}")

            if parallel and len(batch) > 1:
                print(f"Spawning {len(batch)} agents in parallel...")
                for item_id in batch:
                    self.spawn_agent(item_id, dry_run)
            else:
                for item_id in batch:
                    self.spawn_agent(item_id, dry_run)

            if not dry_run:
                print(f"\nBatch {i} execution initiated.")
                print("Waiting for completion before proceeding to next batch...")


def main():
    parser = argparse.ArgumentParser(
        description="Spawn AI agents for sprint work items"
    )
    parser.add_argument(
        "sprint",
        help="Sprint directory name (e.g., sprint-001)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be done without executing",
    )
    parser.add_argument(
        "--parallel",
        action="store_true",
        help="Execute parallelisable items concurrently",
    )
    parser.add_argument(
        "--item",
        help="Execute only a specific work item (e.g., WI-001)",
    )

    args = parser.parse_args()

    # Find sprint directory
    script_dir = Path(__file__).parent
    sprints_dir = script_dir.parent
    sprint_dir = sprints_dir / args.sprint

    if not sprint_dir.exists():
        print(f"Error: Sprint directory not found: {sprint_dir}")
        sys.exit(1)

    # Load and execute sprint
    executor = SprintExecutor(sprint_dir)
    executor.load_sprint()
    executor.print_summary()

    if args.item:
        # Execute single work item
        if args.item in executor.work_items:
            executor.spawn_agent(args.item, args.dry_run)
        else:
            print(f"Error: Work item not found: {args.item}")
            sys.exit(1)
    else:
        # Execute full sprint
        executor.execute_sprint(args.parallel, args.dry_run)


if __name__ == "__main__":
    main()
