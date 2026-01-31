.PHONY: build run stop clean backup logs restart build-local run-local local

# Docker compose command
DOCKER_COMPOSE = docker compose

# Project name
PROJECT = jim-dot-tennis

# Local binary paths
BINARY_PATH = ./bin/jim-dot-tennis
EXTRACT_NONCE_PATH = ./bin/extract-nonce
IMPORT_MATCHCARDS_PATH = ./bin/import-matchcards

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

# Build all utilities
build-utils: build-extract-nonce build-import-matchcards

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
