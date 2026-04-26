#!/bin/bash

# AI API Gateway Database Backup Script
# Usage: ./backup.sh [backup_directory]

set -e

# Default backup directory
BACKUP_DIR="${1:-./backups}"

# Docker service names
POSTGRES_SERVICE="postgres"
POSTGRES_DB="ai_gateway"
POSTGRES_USER="postgres"

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Generate backup filename with timestamp
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/ai_gateway_${TIMESTAMP}.sql"
BACKUP_ZIP="$BACKUP_DIR/ai_gateway_${TIMESTAMP}.sql.gz"

echo "Starting database backup at $(date)"

# Check if docker compose is running
if ! docker compose ps --services | grep -q "$POSTGRES_SERVICE"; then
    echo "Error: PostgreSQL service '$POSTGRES_SERVICE' is not running."
    exit 1
fi

# Perform database backup
echo "Backing up database '$POSTGRES_DB'..."
docker compose exec -T "$POSTGRES_SERVICE" pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > "$BACKUP_FILE"

# Check if backup was successful
if [ $? -eq 0 ] && [ -s "$BACKUP_FILE" ]; then
    echo "Database backup completed: $BACKUP_FILE"

    # Compress backup
    echo "Compressing backup file..."
    gzip -c "$BACKUP_FILE" > "$BACKUP_ZIP"

    # Remove uncompressed file
    rm "$BACKUP_FILE"

    # Keep only last 30 days of backups
    echo "Cleaning up old backups (keeping last 30 days)..."
    find "$BACKUP_DIR" -name "ai_gateway_*.sql.gz" -mtime +30 -delete

    # Report backup size
    BACKUP_SIZE=$(du -h "$BACKUP_ZIP" | cut -f1)
    echo "Backup completed successfully: $BACKUP_ZIP ($BACKUP_SIZE)"

    # Count remaining backups
    BACKUP_COUNT=$(find "$BACKUP_DIR" -name "ai_gateway_*.sql.gz" | wc -l)
    echo "Total backups in directory: $BACKUP_COUNT"
else
    echo "Error: Database backup failed!"
    if [ -f "$BACKUP_FILE" ]; then
        rm "$BACKUP_FILE"
    fi
    exit 1
fi

echo "Backup finished at $(date)"