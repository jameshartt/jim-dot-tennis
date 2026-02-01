.PHONY: build run stop clean backup logs restart build-local run-local local \
	build-tmx courthive courthive-up courthive-down courthive-restart courthive-logs \
	vet fmt fmt-fix imports imports-fix lint deadcode tidy check

# Docker compose command
DOCKER_COMPOSE = docker compose

# Project name
PROJECT = jim-dot-tennis

# Local binary paths
BINARY_PATH = ./bin/jim-dot-tennis
EXTRACT_NONCE_PATH = ./bin/extract-nonce
IMPORT_MATCHCARDS_PATH = ./bin/import-matchcards
IMPORT_CLUB_INFO_PATH = ./bin/import-club-info

# Default target
all: build run

# Local development commands
# Build the Go binary locally and put it in bin/
build-local:
	@echo "Building $(PROJECT) binary..."
	@mkdir -p bin
	go build -o $(BINARY_PATH) ./cmd/jim-dot-tennis

# Build the extract-nonce utility
build-extract-nonce:
	@echo "Building extract-nonce utility..."
	@mkdir -p bin
	go build -o $(EXTRACT_NONCE_PATH) ./cmd/extract-nonce

# Build the import-matchcards utility
build-import-matchcards:
	@echo "Building import-matchcards utility..."
	@mkdir -p bin
	go build -o $(IMPORT_MATCHCARDS_PATH) ./cmd/import-matchcards

# Build the import-club-info utility
build-import-club-info:
	@echo "Building import-club-info utility..."
	@mkdir -p bin
	go build -o $(IMPORT_CLUB_INFO_PATH) ./cmd/import-club-info

# Build all utilities
build-utils: build-extract-nonce build-import-matchcards build-import-club-info

# Build everything
build-all: build-local build-utils

# Run the application locally with database at project root
run-local: build-local
	@echo "Starting $(PROJECT) locally..."
	@echo "Database will be created at: ./tennis.db"
	@echo "Server will be available at: http://localhost:8080"
	DB_PATH=./tennis.db $(BINARY_PATH)

# Combined local development command
local: run-local

# Clean local build artifacts
clean-local:
	@echo "Cleaning local build artifacts..."
	rm -f $(BINARY_PATH)
	rm -f $(EXTRACT_NONCE_PATH)
	rm -f $(IMPORT_MATCHCARDS_PATH)
	rm -f $(IMPORT_CLUB_INFO_PATH)
	rm -f ./tennis.db

# Build the Docker images
build:
	$(DOCKER_COMPOSE) build

# Start the application
run:
	$(DOCKER_COMPOSE) up -d

# Stop the application
stop:
	$(DOCKER_COMPOSE) down

# Stop the application and remove volumes
clean:
	$(DOCKER_COMPOSE) down -v

# Restart the application
restart: stop run

# View logs
logs:
	$(DOCKER_COMPOSE) logs -f

# View app logs only
app-logs:
	$(DOCKER_COMPOSE) logs -f app

# View backup logs only
backup-logs:
	$(DOCKER_COMPOSE) logs -f backup

# Create a manual backup
backup:
	docker exec jim-dot-tennis-backup sh -c 'DATE=$$(date +%Y-%m-%d-%H%M%S) && \
		sqlite3 /data/tennis.db ".backup /backups/tennis-$${DATE}-manual.db" && \
		echo "Manual backup created: tennis-$${DATE}-manual.db"'

# Export a backup to the host system
export-backup:
	@mkdir -p ./exported-backups
	@LATEST=$$(docker run --rm -v jim-dot-tennis-backups:/backups alpine:latest \
		find /backups -name "*.db" -type f -printf "%T@ %p\n" | sort -nr | head -n 1 | cut -d' ' -f2); \
	FILENAME=$$(basename $$LATEST); \
	docker run --rm -v jim-dot-tennis-backups:/backups -v $$(pwd)/exported-backups:/exported alpine:latest \
		cp $$LATEST /exported/$$FILENAME && \
	echo "Exported backup to ./exported-backups/$$FILENAME"

# Enter shell in the app container
shell:
	docker exec -it jim-dot-tennis /bin/sh

# Show running containers
ps:
	$(DOCKER_COMPOSE) ps

# Follow the TDD development workflow
dev: build run logs

# ============================================================
# Go Tooling (runs inside Docker - no local Go required)
# ============================================================

# Go image (matches Dockerfile builder stage)
GO_IMAGE = golang:1.25-alpine

# Docker run for tools that need CGO (vet, deadcode, mod tidy - anything that type-checks sqlite)
DOCKER_GO_CGO = docker run --rm -v $$(pwd):/app -w /app $(GO_IMAGE) sh -c \
	"apk add --no-cache gcc musl-dev sqlite-dev build-base > /dev/null 2>&1 && CGO_ENABLED=1

# Docker run for text-only tools (gofmt, goimports - no compilation needed)
DOCKER_GO = docker run --rm -v $$(pwd):/app -w /app $(GO_IMAGE)

# Run go vet (static analysis)
vet:
	@echo "Running go vet..."
	@$(DOCKER_GO_CGO) go vet ./..."

# Check formatting (list unformatted files)
fmt:
	@echo "Checking formatting..."
	@$(DOCKER_GO) gofmt -l .

# Fix formatting in-place
fmt-fix:
	@echo "Fixing formatting..."
	@$(DOCKER_GO) gofmt -w .

# Check import ordering (list files with import issues)
imports:
	@echo "Checking imports..."
	@$(DOCKER_GO) sh -c "go install golang.org/x/tools/cmd/goimports@latest && goimports -l -local jim-dot-tennis ."

# Fix import ordering in-place
imports-fix:
	@echo "Fixing imports..."
	@$(DOCKER_GO) sh -c "go install golang.org/x/tools/cmd/goimports@latest && goimports -w -local jim-dot-tennis ."

# Run golangci-lint (comprehensive linting)
lint:
	@echo "Running golangci-lint..."
	@docker run --rm -v $$(pwd):/app -w /app -e GOFLAGS=-buildvcs=false $(GO_IMAGE) sh -c \
		"apk add --no-cache gcc musl-dev sqlite-dev build-base git > /dev/null 2>&1 && CGO_ENABLED=1 go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && golangci-lint run ./..."

# Run dead code detection
deadcode:
	@echo "Running deadcode analysis..."
	@$(DOCKER_GO_CGO) go install golang.org/x/tools/cmd/deadcode@latest && deadcode ./..."

# Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	@$(DOCKER_GO_CGO) go mod tidy"

# Run all read-only checks
check: vet fmt lint deadcode
	@echo "All checks complete."

# ============================================================
# CourtHive
# ============================================================

# CourtHive compose file
COURTHIVE_COMPOSE = $(DOCKER_COMPOSE) -f docker-compose.courthive.yml

# TMX source directory (relative to this project)
TMX_DIR = ../TMX

# Build TMX frontend for local development
build-tmx:
	@echo "Building TMX frontend for local development..."
	cd $(TMX_DIR) && pnpm build

# Build and start the full CourtHive stack locally
courthive: build-tmx
	@echo "Starting CourtHive stack..."
	$(COURTHIVE_COMPOSE) up -d --build

# Start the CourtHive stack without rebuilding TMX
courthive-up:
	$(COURTHIVE_COMPOSE) up -d

# Stop the CourtHive stack
courthive-down:
	$(COURTHIVE_COMPOSE) down

# Restart the CourtHive stack
courthive-restart: courthive-down courthive-up

# View CourtHive stack logs
courthive-logs:
	$(COURTHIVE_COMPOSE) logs -f
