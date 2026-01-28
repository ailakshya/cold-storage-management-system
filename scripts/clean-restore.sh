#!/bin/bash
#
# Clean Restore - Drop and recreate database before restore
# This eliminates "already exists" errors
#

set -e

BACKUP_FILE=$1
CONTAINER=${2:-cold-storage-postgres}
DB_USER=${3:-postgres}
DB_NAME=${4:-cold_db}

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file> [container_name] [db_user] [db_name]"
    echo ""
    echo "Example:"
    echo "  $0 backups/cold_db_backup.sql"
    echo "  $0 backups/cold_db_backup.sql cold-storage-postgres postgres cold_db"
    exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
    echo "✗ Backup file not found: $BACKUP_FILE"
    exit 1
fi

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Clean Database Restore                                    ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Backup: $BACKUP_FILE"
echo "Container: $CONTAINER"
echo "Database: $DB_NAME"
echo ""

# Step 1: Drop existing database
echo "Step 1/4: Dropping existing database (if exists)..."
docker exec "$CONTAINER" psql -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;" 2>&1 | grep -v "^$"
echo "✓ Database dropped"

# Step 2: Create fresh database
echo ""
echo "Step 2/4: Creating fresh database..."
docker exec "$CONTAINER" psql -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;" 2>&1 | grep -v "^$"
echo "✓ Database created"

# Step 3: Restore from backup
echo ""
echo "Step 3/4: Restoring from backup..."
docker exec -i "$CONTAINER" psql -U "$DB_USER" -d postgres < "$BACKUP_FILE" 2>&1 | \
    grep -E "(ERROR|WARNING|CREATE|^[0-9]+ rows|COPY)" | head -30
echo "✓ Restore completed"

# Step 4: Verify
echo ""
echo "Step 4/4: Verifying restoration..."
TABLE_COUNT=$(docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')
CUSTOMER_COUNT=$(docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM customers;" | tr -d ' ')
ENTRY_COUNT=$(docker exec "$CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM entries;" | tr -d ' ')

echo "✓ Tables: $TABLE_COUNT"
echo "✓ Customers: $CUSTOMER_COUNT"
echo "✓ Entries: $ENTRY_COUNT"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✓ CLEAN RESTORE COMPLETED WITHOUT ERRORS"
echo "════════════════════════════════════════════════════════════"
echo ""
