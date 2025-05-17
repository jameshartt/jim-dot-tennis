#!/bin/bash
# backup-manager.sh - External backup script for jim-dot-tennis application
# This script exports backups from the Docker volume to external storage

# Configuration
BACKUP_DIR="/path/to/external/backups"  # Change this to your desired location
RETENTION_DAYS=90                       # Keep backups for 90 days
CONTAINER_NAME="jim-dot-tennis-backup"  # Docker container name for backups
DOCKER_VOLUME="jim-dot-tennis-backups"  # Docker volume name for backups
TODAY=$(date +%Y-%m-%d)

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Get the most recent backup from the Docker volume
echo "Copying latest backup from Docker volume to external storage..."
LATEST_BACKUP=$(docker run --rm -v "$DOCKER_VOLUME:/backups" alpine:latest find /backups -name "*.db" -type f -printf "%T@ %p\n" | sort -nr | head -n 1 | cut -d' ' -f2)

if [ -z "$LATEST_BACKUP" ]; then
    echo "No backups found in Docker volume"
    exit 1
fi

# Extract the filename from the path
BACKUP_FILENAME=$(basename "$LATEST_BACKUP")

# Copy the backup to the external storage
echo "Copying $BACKUP_FILENAME to $BACKUP_DIR..."
docker run --rm -v "$DOCKER_VOLUME:/backups" -v "$BACKUP_DIR:/external" alpine:latest cp "$LATEST_BACKUP" "/external/$BACKUP_FILENAME"

# Compress the backup
echo "Compressing backup..."
gzip -f "$BACKUP_DIR/$BACKUP_FILENAME"

# Clean up old backups
echo "Cleaning up backups older than $RETENTION_DAYS days..."
find "$BACKUP_DIR" -name "*.db.gz" -type f -mtime +"$RETENTION_DAYS" -delete

# Optional: Upload to cloud storage
# Uncomment and modify as needed for your cloud provider

# # AWS S3
# if command -v aws &> /dev/null; then
#     echo "Uploading to S3..."
#     aws s3 cp "$BACKUP_DIR/$BACKUP_FILENAME.gz" "s3://your-bucket/jim-dot-tennis/backups/$BACKUP_FILENAME.gz"
# fi

# # Backblaze B2
# if command -v b2 &> /dev/null; then
#     echo "Uploading to B2..."
#     b2 upload-file your-bucket-name "$BACKUP_DIR/$BACKUP_FILENAME.gz" "jim-dot-tennis/backups/$BACKUP_FILENAME.gz"
# fi

echo "Backup process completed successfully!"
echo "Backup stored at: $BACKUP_DIR/$BACKUP_FILENAME.gz" 