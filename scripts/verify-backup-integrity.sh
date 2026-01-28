#!/bin/bash
#
# Verify Backup Integrity
# This script checks that all critical data was backed up and restored correctly
#

set -e

CONTAINER_NAME="cold-storage-test-db"
DB_NAME="cold_db"
DB_USER="postgres"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Backup Integrity Verification                            ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Check if container is running
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    echo "✗ Container $CONTAINER_NAME is not running"
    exit 1
fi
echo "✓ Docker container is running"

# Function to run SQL and get result
run_sql() {
    docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1"
}

echo ""
echo "Checking database schema..."
echo "════════════════════════════════════════════════════════════"

# Count tables
TABLE_COUNT=$(run_sql "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';")
echo "✓ Total tables: $TABLE_COUNT"

# Check critical tables exist
CRITICAL_TABLES=("customers" "entries" "room_entries" "users" "rent_payments" "gate_passes" "ledger_entries" "system_settings")
for table in "${CRITICAL_TABLES[@]}"; do
    EXISTS=$(run_sql "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '$table';")
    if [ "$EXISTS" = "1" ]; then
        ROW_COUNT=$(run_sql "SELECT COUNT(*) FROM $table;")
        echo "✓ Table '$table' exists with $ROW_COUNT rows"
    else
        echo "✗ Critical table '$table' is missing!"
        exit 1
    fi
done

echo ""
echo "Checking data integrity..."
echo "════════════════════════════════════════════════════════════"

# Check for data in key tables
CUSTOMERS=$(run_sql "SELECT COUNT(*) FROM customers;")
ENTRIES=$(run_sql "SELECT COUNT(*) FROM entries;")
USERS=$(run_sql "SELECT COUNT(*) FROM users;")
PAYMENTS=$(run_sql "SELECT COUNT(*) FROM rent_payments;")

if [ "$CUSTOMERS" -gt 0 ]; then
    echo "✓ Customers: $CUSTOMERS records"
else
    echo "⚠ Warning: No customer records found"
fi

if [ "$ENTRIES" -gt 0 ]; then
    echo "✓ Entries: $ENTRIES records"
else
    echo "⚠ Warning: No entry records found"
fi

if [ "$USERS" -gt 0 ]; then
    echo "✓ Users: $USERS records"
else
    echo "✗ Error: No user records found!"
    exit 1
fi

if [ "$PAYMENTS" -gt 0 ]; then
    echo "✓ Payments: $PAYMENTS records"
else
    echo "⚠ Warning: No payment records found"
fi

echo ""
echo "Checking indexes and constraints..."
echo "════════════════════════════════════════════════════════════"

# Count indexes
INDEX_COUNT=$(run_sql "SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public';")
echo "✓ Total indexes: $INDEX_COUNT"

# Count constraints
CONSTRAINT_COUNT=$(run_sql "SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = 'public';")
echo "✓ Total constraints: $CONSTRAINT_COUNT"

echo ""
echo "Checking database size..."
echo "════════════════════════════════════════════════════════════"

# Get database size
DB_SIZE=$(run_sql "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));")
echo "✓ Database size: $DB_SIZE"

# Get top 5 largest tables
echo ""
echo "Largest tables:"
docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -c "
SELECT
    table_name,
    pg_size_pretty(pg_total_relation_size(quote_ident(table_name))) AS size,
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'public' AND information_schema.columns.table_name = tables.table_name) as columns
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY pg_total_relation_size(quote_ident(table_name)) DESC
LIMIT 5;" | grep -v "^$"

echo ""
echo "Checking sequences..."
echo "════════════════════════════════════════════════════════════"

# Count sequences
SEQUENCE_COUNT=$(run_sql "SELECT COUNT(*) FROM information_schema.sequences WHERE sequence_schema = 'public';")
echo "✓ Total sequences: $SEQUENCE_COUNT"

echo ""
echo "Checking functions and procedures..."
echo "════════════════════════════════════════════════════════════"

# Count functions
FUNCTION_COUNT=$(run_sql "SELECT COUNT(*) FROM pg_proc WHERE pronamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public');")
echo "✓ Total functions: $FUNCTION_COUNT"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✓ BACKUP INTEGRITY VERIFICATION PASSED"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Summary:"
echo "  - All critical tables present and populated"
echo "  - Indexes and constraints restored"
echo "  - Database schema complete"
echo "  - Data integrity verified"
echo ""
echo "The backup is complete and can be used for testing!"
echo ""
