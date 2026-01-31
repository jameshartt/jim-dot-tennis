#!/bin/bash

# Sprint execution script for AI agents
# This script iterates over work items and can spawn AI agents for parallel execution

set -e

SPRINT_DIR="${1:-sprint-001}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRINTS_ROOT="$(dirname "$SCRIPT_DIR")"
SPRINT_PATH="$SPRINTS_ROOT/$SPRINT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Sprint Runner for AI Agents${NC}"
echo "=============================="
echo ""

# Check if sprint directory exists
if [ ! -d "$SPRINT_PATH" ]; then
    echo -e "${RED}Error: Sprint directory not found: $SPRINT_PATH${NC}"
    exit 1
fi

# Check if jq is installed for JSON parsing
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is required but not installed${NC}"
    echo "Install with: sudo apt install jq (Ubuntu/Debian) or brew install jq (Mac)"
    exit 1
fi

# Read sprint metadata
SPRINT_JSON="$SPRINT_PATH/sprint.json"
if [ ! -f "$SPRINT_JSON" ]; then
    echo -e "${RED}Error: sprint.json not found in $SPRINT_PATH${NC}"
    exit 1
fi

SPRINT_NAME=$(jq -r '.sprint.name' "$SPRINT_JSON")
SPRINT_GOAL=$(jq -r '.sprint.goal' "$SPRINT_JSON")
WORK_ITEMS=($(jq -r '.sprint.work_items[]' "$SPRINT_JSON"))

echo -e "${GREEN}Sprint:${NC} $SPRINT_NAME"
echo -e "${GREEN}Goal:${NC} $SPRINT_GOAL"
echo -e "${GREEN}Work Items:${NC} ${#WORK_ITEMS[@]}"
echo ""

# Function to check if work item has unmet dependencies
check_dependencies() {
    local work_item_id=$1
    local work_item_file="$SPRINT_PATH/${work_item_id}.json"

    if [ ! -f "$work_item_file" ]; then
        echo "true" # Missing file means can't check
        return
    fi

    local deps=$(jq -r '.dependencies[]' "$work_item_file" 2>/dev/null)

    if [ -z "$deps" ]; then
        echo "false" # No dependencies
        return
    fi

    # Check if any dependencies are incomplete
    # In a real implementation, you'd check completion status
    # For now, assume all dependencies are met
    echo "false"
}

# Function to check if work item is parallelisable
is_parallelisable() {
    local work_item_id=$1
    local work_item_file="$SPRINT_PATH/${work_item_id}.json"

    if [ ! -f "$work_item_file" ]; then
        echo "false"
        return
    fi

    jq -r '.parallelisable' "$work_item_file"
}

# Function to display work item summary
display_work_item() {
    local work_item_id=$1
    local work_item_file="$SPRINT_PATH/${work_item_id}.json"

    if [ ! -f "$work_item_file" ]; then
        echo -e "${RED}  ✗ $work_item_id - File not found${NC}"
        return
    fi

    local title=$(jq -r '.title' "$work_item_file")
    local priority=$(jq -r '.priority' "$work_item_file")
    local complexity=$(jq -r '.complexity' "$work_item_file")
    local parallelisable=$(jq -r '.parallelisable' "$work_item_file")
    local dependencies=$(jq -r '.dependencies | join(", ")' "$work_item_file")

    local priority_color=$NC
    case $priority in
        high) priority_color=$RED ;;
        medium) priority_color=$YELLOW ;;
        low) priority_color=$GREEN ;;
    esac

    echo -e "${BLUE}$work_item_id${NC}: $title"
    echo -e "  Priority: ${priority_color}${priority}${NC} | Complexity: $complexity | Parallel: $parallelisable"
    if [ "$dependencies" != "" ] && [ "$dependencies" != "null" ]; then
        echo -e "  Dependencies: $dependencies"
    fi
}

# Display all work items
echo -e "${YELLOW}Work Items Overview:${NC}"
echo "--------------------"
for work_item in "${WORK_ITEMS[@]}"; do
    display_work_item "$work_item"
    echo ""
done

# Categorize work items
echo -e "${YELLOW}Execution Plan:${NC}"
echo "---------------"

PARALLEL_ITEMS=()
SEQUENTIAL_ITEMS=()
BLOCKED_ITEMS=()

for work_item in "${WORK_ITEMS[@]}"; do
    work_item_file="$SPRINT_PATH/${work_item}.json"

    if [ ! -f "$work_item_file" ]; then
        continue
    fi

    has_deps=$(check_dependencies "$work_item")
    parallel=$(is_parallelisable "$work_item")

    if [ "$has_deps" = "true" ]; then
        BLOCKED_ITEMS+=("$work_item")
    elif [ "$parallel" = "true" ]; then
        PARALLEL_ITEMS+=("$work_item")
    else
        SEQUENTIAL_ITEMS+=("$work_item")
    fi
done

echo -e "${GREEN}Parallelisable Items (${#PARALLEL_ITEMS[@]}):${NC}"
for item in "${PARALLEL_ITEMS[@]}"; do
    echo "  - $item"
done
echo ""

echo -e "${YELLOW}Sequential Items (${#SEQUENTIAL_ITEMS[@]}):${NC}"
for item in "${SEQUENTIAL_ITEMS[@]}"; do
    echo "  - $item"
done
echo ""

if [ ${#BLOCKED_ITEMS[@]} -gt 0 ]; then
    echo -e "${RED}Blocked Items (${#BLOCKED_ITEMS[@]}):${NC}"
    for item in "${BLOCKED_ITEMS[@]}"; do
        echo "  - $item"
    done
    echo ""
fi

# Function to spawn agent for a work item (placeholder)
spawn_agent() {
    local work_item_id=$1
    local work_item_file="$SPRINT_PATH/${work_item_id}.json"

    echo -e "${BLUE}Spawning agent for $work_item_id...${NC}"

    # In a real implementation, this would:
    # 1. Read the work item JSON
    # 2. Construct a prompt for Claude Code
    # 3. Spawn a new agent with the Task tool
    # 4. Track the agent's progress

    # Example command (not executed in this script):
    # claude-code task execute --work-item "$work_item_file"

    echo -e "${GREEN}  → Agent would be spawned for: $work_item_id${NC}"

    # Read and display the work item for demonstration
    local title=$(jq -r '.title' "$work_item_file")
    local description=$(jq -r '.description' "$work_item_file")

    echo -e "${YELLOW}  Title:${NC} $title"
    echo -e "${YELLOW}  Description:${NC} $description"
}

# Example: Process parallel items
echo -e "${YELLOW}Example: Processing Parallelisable Items${NC}"
echo "------------------------------------------"
for item in "${PARALLEL_ITEMS[@]}"; do
    spawn_agent "$item"
    echo ""
done

echo -e "${GREEN}Sprint analysis complete!${NC}"
echo ""
echo "To execute work items with AI agents:"
echo "  1. Read each work item JSON file"
echo "  2. Pass the work item to an AI agent with:"
echo "     - The work item JSON as context"
echo "     - Instructions to follow acceptance criteria"
echo "     - Access to the codebase"
echo ""
echo "For parallel execution:"
echo "  - Spawn multiple agents simultaneously for parallelisable items"
echo "  - Use agent task queuing for sequential items"
echo ""
echo "Example AI agent prompt:"
echo "  'Implement work item WI-001. Read the work item JSON at"
echo "   $SPRINT_PATH/WI-001.json and implement according to"
echo "   the acceptance criteria and technical notes.'"
