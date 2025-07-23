#!/bin/bash
set -e

# Download Database Script for Jim.Tennis
# Downloads the production database from DigitalOcean server and replaces local database

# Configuration - matches deploy-digitalocean.sh
DROPLET_IP="144.126.228.64"              # Your droplet's IP address
SSH_USER="root"                           # SSH user
SSH_KEY_PATH=""                           # Path to your SSH private key (leave empty for default)
DEPLOY_DIR="/opt/jim-dot-tennis"          # Deployment directory on the server
DB_PATH_SERVER="/var/lib/docker/volumes/jim-dot-tennis-data/_data/tennis.db"  # Database path on server (Docker volume)
DB_PATH_LOCAL="./tennis.db"               # Local database path

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Determine SSH key parameter
if [ -n "$SSH_KEY_PATH" ]; then
  SSH_KEY_PARAM="-i $SSH_KEY_PATH"
else
  SSH_KEY_PARAM=""
fi

# SSH options
SSH_OPTS="-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -o BatchMode=yes"
SCP_OPTS="-o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new"

echo "========================================================"
echo "Jim.Tennis Database Download"
echo "========================================================"
echo "Server IP:      $DROPLET_IP"
echo "SSH User:       $SSH_USER"
echo "Remote DB:      $DB_PATH_SERVER"
echo "Local DB:       $DB_PATH_LOCAL"
echo "========================================================"

# Function to handle SSH commands with proper error handling
function remote_command() {
  if ! ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "$1"; then
    echo -e "${RED}Error: Command failed: $1${NC}"
    exit 1
  fi
}

# Step 1: Test SSH connection
echo "Testing SSH connection..."
if ! ssh $SSH_KEY_PARAM $SSH_OPTS -q $SSH_USER@$DROPLET_IP exit; then
  echo -e "${RED}Error: Cannot establish SSH connection to $DROPLET_IP${NC}"
  echo "Please check your SSH key and that the server is reachable."
  exit 1
fi
echo -e "${GREEN}✓ SSH connection successful${NC}"

# Step 2: Check if remote database exists
echo "Checking if remote database exists..."
if ! ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "[ -f $DB_PATH_SERVER ]"; then
  echo -e "${RED}Error: Database file not found at $DB_PATH_SERVER${NC}"
  echo "Please verify the database path on the server."
  exit 1
fi
echo -e "${GREEN}✓ Remote database found${NC}"

# Step 3: Get database file size for confirmation
echo "Getting remote database information..."
DB_SIZE=$(ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "ls -lh $DB_PATH_SERVER | awk '{print \$5}'")
DB_DATE=$(ssh $SSH_KEY_PARAM $SSH_OPTS $SSH_USER@$DROPLET_IP "ls -l $DB_PATH_SERVER | awk '{print \$6, \$7, \$8}'")
echo "Remote database size: $DB_SIZE"
echo "Remote database date: $DB_DATE"

# Step 4: Backup existing local database if it exists
if [ -f "$DB_PATH_LOCAL" ]; then
  BACKUP_NAME="tennis.db.backup.$(date +%Y%m%d_%H%M%S)"
  echo -e "${YELLOW}Backing up existing local database to $BACKUP_NAME${NC}"
  cp "$DB_PATH_LOCAL" "$BACKUP_NAME"
  echo -e "${GREEN}✓ Local database backed up${NC}"
fi

# Step 5: Download the database
echo "Downloading database from server..."
if scp $SSH_KEY_PARAM $SCP_OPTS $SSH_USER@$DROPLET_IP:$DB_PATH_SERVER $DB_PATH_LOCAL; then
  echo -e "${GREEN}✓ Database downloaded successfully${NC}"
else
  echo -e "${RED}Error: Failed to download database${NC}"
  # Restore backup if download failed and backup exists
  if [ -n "$BACKUP_NAME" ] && [ -f "$BACKUP_NAME" ]; then
    echo "Restoring backup..."
    mv "$BACKUP_NAME" "$DB_PATH_LOCAL"
    echo -e "${YELLOW}Original database restored from backup${NC}"
  fi
  exit 1
fi

# Step 6: Verify the downloaded database
if [ -f "$DB_PATH_LOCAL" ]; then
  LOCAL_SIZE=$(ls -lh "$DB_PATH_LOCAL" | awk '{print $5}')
  echo "Downloaded database size: $LOCAL_SIZE"
  echo -e "${GREEN}✓ Database download completed successfully${NC}"
else
  echo -e "${RED}Error: Downloaded database file not found${NC}"
  exit 1
fi

# Step 7: Optional - Test database integrity
echo "Testing database integrity..."
if command -v sqlite3 >/dev/null 2>&1; then
  if sqlite3 "$DB_PATH_LOCAL" "PRAGMA integrity_check;" | grep -q "ok"; then
    echo -e "${GREEN}✓ Database integrity check passed${NC}"
  else
    echo -e "${YELLOW}⚠ Database integrity check failed or returned warnings${NC}"
    echo "The database was still downloaded, but you may want to check it manually."
  fi
else
  echo -e "${YELLOW}⚠ sqlite3 not found, skipping integrity check${NC}"
fi

echo "========================================================"
echo -e "${GREEN}Database download completed successfully!${NC}"
echo "========================================================"
echo "Local database: $DB_PATH_LOCAL"
echo "Size: $(ls -lh "$DB_PATH_LOCAL" | awk '{print $5}')"

if [ -n "$BACKUP_NAME" ]; then
  echo -e "${YELLOW}Previous database backed up as: $BACKUP_NAME${NC}"
  echo "You can remove the backup file when you're confident the new database is working correctly."
fi

echo ""
echo "You can now run your local application with the production database." 