#!/bin/bash

# Tennis Import Management Script
# Usage: ./tennis-import.sh [command] [options]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CREDENTIALS_FILE="$SCRIPT_DIR/.tennis-credentials"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

show_help() {
    echo "Tennis Import Management Script"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  setup             Set up or update authentication credentials"
    echo "  run               Run the import with stored credentials"
    echo "  run-dry           Run a dry-run import"
    echo "  run-week N        Import specific week N"
    echo "  run-range N-M     Import weeks N through M"
    echo "  status            Show current credential status"
    echo "  clear             Clear stored credentials"
    echo ""
    echo "Examples:"
    echo "  $0 setup                    # Set up credentials interactively"
    echo "  $0 run                      # Run full import (weeks 1-18)"
    echo "  $0 run-dry                  # Dry run for testing"
    echo "  $0 run-week 5               # Import only week 5"
    echo "  $0 run-range 1-5            # Import weeks 1 through 5"
}

load_credentials() {
    if [ -f "$CREDENTIALS_FILE" ]; then
        source "$CREDENTIALS_FILE"
        return 0
    else
        return 1
    fi
}

save_credentials() {
    cat > "$CREDENTIALS_FILE" << EOF
# Tennis Import Credentials
# Generated on $(date)
export TENNIS_NONCE='$TENNIS_NONCE'
EOF
    chmod 600 "$CREDENTIALS_FILE"
    echo -e "${GREEN}✅ Credentials saved to $CREDENTIALS_FILE${NC}"
}

setup_credentials() {
    echo -e "${BLUE}🔐 Tennis Import Credential Setup${NC}"
    echo ""
    echo "You'll need to get the nonce value from your browser after logging into the tennis club website:"
    echo ""
    
    read -p "Enter TENNIS_NONCE: " TENNIS_NONCE
    echo ""
    
    if [ -n "$TENNIS_NONCE" ]; then
        save_credentials
        echo -e "${GREEN}✅ Setup complete!${NC}"
    else
        echo -e "${RED}❌ Error: TENNIS_NONCE is required${NC}"
        exit 1
    fi
}

show_status() {
    if load_credentials; then
        echo -e "${GREEN}✅ Credentials loaded${NC}"
        echo "   Nonce: ${TENNIS_NONCE:0:10}..."
        echo "   From: $CREDENTIALS_FILE"
    else
        echo -e "${RED}❌ No credentials found${NC}"
        echo "   Run '$0 setup' to configure credentials"
    fi
}

run_import() {
    local dry_run=""
    local start_week=""
    local end_week=""
    local extra_args=""
    
    # Parse arguments
    while [ $# -gt 0 ]; do
        case $1 in
            --dry-run)
                dry_run="--dry-run"
                ;;
            --start-week=*)
                start_week="$1"
                ;;
            --end-week=*)
                end_week="$1"
                ;;
            *)
                extra_args="$extra_args $1"
                ;;
        esac
        shift
    done
    
    if ! load_credentials; then
        echo -e "${RED}❌ No credentials found. Run '$0 setup' first.${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}🚀 Starting tennis import...${NC}"
    
    # Build the Docker command
    docker run --rm \
        --volume /opt/jim-dot-tennis:/app \
        --volume jim-dot-tennis-data:/app/data \
        --workdir /app \
        --env TENNIS_NONCE="$TENNIS_NONCE" \
        jim-dot-tennis-import:latest \
        bash ./scripts/import_all_weeks_optimized.sh $dry_run $start_week $end_week $extra_args
}

case "${1:-}" in
    setup)
        setup_credentials
        ;;
    run)
        shift
        run_import "$@"
        ;;
    run-dry)
        shift
        run_import --dry-run "$@"
        ;;
    run-week)
        if [ -z "$2" ]; then
            echo -e "${RED}❌ Error: Week number required${NC}"
            echo "Usage: $0 run-week N"
            exit 1
        fi
        shift
        week="$1"
        shift
        run_import --start-week="$week" --end-week="$week" "$@"
        ;;
    run-range)
        if [ -z "$2" ]; then
            echo -e "${RED}❌ Error: Week range required${NC}"
            echo "Usage: $0 run-range N-M"
            exit 1
        fi
        range="$2"
        if [[ ! "$range" =~ ^[0-9]+-[0-9]+$ ]]; then
            echo -e "${RED}❌ Error: Invalid range format. Use N-M (e.g., 1-5)${NC}"
            exit 1
        fi
        start_week="${range%-*}"
        end_week="${range#*-}"
        shift 2
        run_import --start-week="$start_week" --end-week="$end_week" "$@"
        ;;
    status)
        show_status
        ;;
    clear)
        if [ -f "$CREDENTIALS_FILE" ]; then
            rm "$CREDENTIALS_FILE"
            echo -e "${GREEN}✅ Credentials cleared${NC}"
        else
            echo -e "${YELLOW}⚠️  No credentials to clear${NC}"
        fi
        ;;
    help|--help|-h)
        show_help
        ;;
    "")
        show_help
        ;;
    *)
        echo -e "${RED}❌ Unknown command: $1${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac 