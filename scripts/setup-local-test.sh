#!/bin/bash
#
# Setup Local Testing Environment with Database Backup
# This script creates a local Docker container with PostgreSQL and restores the backup
#

set -e

BACKUP_FILE=$1
CONTAINER_NAME="cold-storage-test-db"
DB_NAME="cold_db"
DB_USER="postgres"
DB_PASSWORD="testpassword123"
DB_PORT="5433"  # Use different port to avoid conflicts with production

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file>"
    echo ""
    echo "Example:"
    echo "  $0 backups/remote/cold_db_remote_20260128_120000.sql"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    exit 1
fi

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Cold Storage - Local Testing Environment Setup           ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Backup File: $BACKUP_FILE"
echo "Container Name: $CONTAINER_NAME"
echo "Database Port: $DB_PORT"
echo ""

echo "Step 1/5: Stopping and removing existing test container (if any)..."
docker stop "$CONTAINER_NAME" 2>/dev/null || true
docker rm "$CONTAINER_NAME" 2>/dev/null || true
echo "✓ Cleaned up existing container"

echo ""
echo "Step 2/5: Starting PostgreSQL container..."
docker run -d \
    --name "$CONTAINER_NAME" \
    -e POSTGRES_PASSWORD="$DB_PASSWORD" \
    -e POSTGRES_USER="$DB_USER" \
    -e POSTGRES_DB="postgres" \
    -p "$DB_PORT:5432" \
    postgres:17-alpine

if [ $? -eq 0 ]; then
    echo "✓ PostgreSQL container started"
else
    echo "✗ Failed to start PostgreSQL container"
    exit 1
fi

echo ""
echo "Step 3/5: Waiting for PostgreSQL to be ready..."
sleep 5

# Wait for PostgreSQL to be ready
for i in {1..30}; do
    if docker exec "$CONTAINER_NAME" pg_isready -U "$DB_USER" > /dev/null 2>&1; then
        echo "✓ PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "✗ PostgreSQL failed to start within 30 seconds"
        docker logs "$CONTAINER_NAME"
        exit 1
    fi
    echo "  Waiting... ($i/30)"
    sleep 1
done

echo ""
echo "Step 4/5: Restoring database from backup..."
echo "  This may take a few minutes depending on backup size..."

# Copy backup file into container
docker cp "$BACKUP_FILE" "$CONTAINER_NAME:/tmp/backup.sql"

# Restore the backup
docker exec -i "$CONTAINER_NAME" psql -U "$DB_USER" -d postgres < "$BACKUP_FILE" 2>&1 | grep -v "^$" | head -20

if [ ${PIPESTATUS[0]} -eq 0 ]; then
    echo "✓ Database restored successfully"
else
    echo "✗ Database restoration had errors (check logs above)"
    echo "  Note: Some errors may be expected (e.g., role already exists)"
fi

echo ""
echo "Step 5/5: Verifying restoration..."

# Check if database exists
DB_EXISTS=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -lqt | cut -d \| -f 1 | grep -w "$DB_NAME" | wc -l)
if [ "$DB_EXISTS" -gt 0 ]; then
    echo "✓ Database '$DB_NAME' exists"

    # Count tables
    TABLE_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')
    echo "✓ Tables restored: $TABLE_COUNT"

    # Check for key tables
    docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name;" | head -10
else
    echo "✗ Database '$DB_NAME' was not created"
    exit 1
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✓ LOCAL TESTING ENVIRONMENT READY"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Connection Details:"
echo "  Host: localhost"
echo "  Port: $DB_PORT"
echo "  Database: $DB_NAME"
echo "  User: $DB_USER"
echo "  Password: $DB_PASSWORD"
echo ""
echo "Connection String:"
echo "  postgresql://$DB_USER:$DB_PASSWORD@localhost:$DB_PORT/$DB_NAME"
echo ""
echo "Useful Commands:"
echo "  Connect to DB:      docker exec -it $CONTAINER_NAME psql -U $DB_USER -d $DB_NAME"
echo "  View logs:          docker logs $CONTAINER_NAME"
echo "  Stop container:     docker stop $CONTAINER_NAME"
echo "  Remove container:   docker rm $CONTAINER_NAME"
echo ""
echo "Next steps:"
echo "  1. Update your config.yaml to point to localhost:$DB_PORT"
echo "  2. Start the application: go run cmd/server/main.go"
echo ""
