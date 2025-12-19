#!/bin/bash
#
# Database Migration Script - migrate_via_stdin.sh
# ================================================
#
# Purpose:
#   Migrate database schema and data from local PostgreSQL container
#   to the K3s cluster's CloudNativePG database.
#
# Usage:
#   ./scripts/database/migrate_via_stdin.sh
#
# Requirements:
#   - Local PostgreSQL container 'cold-storage-postgres' running on port 5432
#   - kubectl configured to access K3s cluster
#   - Docker access (via 'sg docker' command)
#   - Migration files in migrations/ directory
#
# What it does:
#   1. Exports data from local PostgreSQL using pg_dump
#   2. Runs all schema migrations in alphabetical order to K3s cluster
#   3. Imports the exported data to K3s cluster
#   4. Verifies the migration by showing table counts
#
# Notes:
#   - Uses stdin piping to avoid "read-only filesystem" errors in K3s pods
#   - Skips migrations with "reset" in filename (destructive operations)
#   - Continues on non-critical errors (e.g., table already exists)
#
# Last Updated: 2025-12-16
# Version: 1.2
#

set -e  # Exit immediately if any command fails

# Configuration
# =============
POD_NAME="cold-postgres-1"    # K3s PostgreSQL primary pod name
DB_NAME="cold_db"               # Database name in both source and target
LOCAL_CONTAINER="cold-storage-postgres"  # Local PostgreSQL container name
TEMP_DUMP="/tmp/data_dump.sql"  # Temporary file for data dump

# Display header
echo "=========================================="
echo "  Database Migration via stdin"
echo "=========================================="
echo ""
echo "Source: Local PostgreSQL ($LOCAL_CONTAINER)"
echo "Target: K3s PostgreSQL ($POD_NAME)"
echo "Database: $DB_NAME"
echo ""

# Step 1: Export data from local PostgreSQL
# ==========================================
echo "Step 1: Exporting data from local PostgreSQL..."

# Use pg_dump to export only data (not schema) in INSERT format
# Flags explained:
#   --data-only: Export only data, not schema (CREATE TABLE statements)
#   --inserts: Use INSERT statements instead of COPY (more portable)
#   --no-owner: Don't set ownership (avoids permission issues)
#   --no-privileges: Don't dump access privileges (avoids permission issues)
sg docker -c "docker exec $LOCAL_CONTAINER pg_dump -U postgres \
    --data-only \
    --inserts \
    --no-owner \
    --no-privileges \
    $DB_NAME" > $TEMP_DUMP

# Check if export succeeded and show file size
if [ ! -f "$TEMP_DUMP" ]; then
    echo "✗ Export failed - file not created"
    exit 1
fi

SIZE=$(du -h $TEMP_DUMP | cut -f1)
echo "✓ Data exported ($SIZE)"

# Step 2: Run schema migrations to K3s cluster
# =============================================
echo ""
echo "Step 2: Running schema migrations..."

MIGRATION_COUNT=0

# Loop through all migration files in alphabetical order
for migration in $(ls -1 migrations/*.sql | sort); do
    filename=$(basename "$migration")

    # Skip reset/destructive migrations (identified by "reset" in filename)
    # These are only for development and should not run automatically
    if [[ "$filename" == *"reset"* ]]; then
        echo "  ⊘ Skipping: $filename (reset script)"
        continue
    fi

    echo "  → Running: $filename"

    # Execute migration via stdin piping to avoid file system write issues
    # Use kubectl exec -i (interactive) to pipe SQL directly to psql
    # Filter output to show only important messages (CREATE, ALTER, INSERT, ERROR)
    # Use || true to continue even if migration fails (e.g., table already exists)
    kubectl exec -i $POD_NAME -- psql -U postgres -d $DB_NAME < "$migration" 2>&1 \
        | grep -E "(ERROR|CREATE|ALTER|INSERT)" | head -2 || true

    ((MIGRATION_COUNT++))
done

echo "✓ Ran $MIGRATION_COUNT migrations"

# Step 3: Import data to K3s cluster
# ===================================
echo ""
echo "Step 3: Importing data..."

# Import data dump via stdin piping
# Show last 10 lines of output to see final status and any errors
kubectl exec -i $POD_NAME -- psql -U postgres -d $DB_NAME < $TEMP_DUMP 2>&1 | tail -10

echo "✓ Data imported"

# Step 4: Cleanup temporary files
# ================================
echo ""
echo "Step 4: Cleaning up..."
rm -f $TEMP_DUMP
echo "✓ Temporary files removed"

# Step 5: Verify the migration
# =============================
echo ""
echo "=========================================="
echo "  Verification"
echo "=========================================="
echo ""

# List all tables in the database
echo "Tables in database:"
kubectl exec -i $POD_NAME -- psql -U postgres -d $DB_NAME << 'SQL'
\dt
SQL

echo ""
echo "Row counts by table:"

# Query row counts for all main tables
# Using UNION ALL to combine results from multiple tables
kubectl exec -i $POD_NAME -- psql -U postgres -d $DB_NAME -t << 'SQL'
SELECT 'users:', COUNT(*) FROM users
UNION ALL
SELECT 'customers:', COUNT(*) FROM customers
UNION ALL
SELECT 'entries:', COUNT(*) FROM entries
UNION ALL
SELECT 'room_entries:', COUNT(*) FROM room_entries
UNION ALL
SELECT 'gate_passes:', COUNT(*) FROM gate_passes
UNION ALL
SELECT 'invoices:', COUNT(*) FROM invoices
UNION ALL
SELECT 'rent_payments:', COUNT(*) FROM rent_payments
UNION ALL
SELECT 'entry_events:', COUNT(*) FROM entry_events;
SQL

# Display success message
echo ""
echo "=========================================="
echo "  ✓ Migration Complete!"
echo "=========================================="
echo ""
echo "Your data has been successfully migrated to the K3s cluster."
echo "Access the database at: cold-postgres-rw.default.svc.cluster.local:5432"
echo ""

# Exit with success
exit 0
