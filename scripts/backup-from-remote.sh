#!/bin/bash
#
# Backup Database from Remote Server
# This script connects to the remote cold storage server and creates a complete backup
#

set -e

REMOTE_USER="cold"
REMOTE_HOST="192.168.1.134"
REMOTE_DB="cold_db"
REMOTE_DB_USER="postgres"
BACKUP_DIR="./backups/remote"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="cold_db_remote_${TIMESTAMP}.sql"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Cold Storage Database Backup - Remote Server             ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "Remote Server: ${REMOTE_USER}@${REMOTE_HOST}"
echo "Database: ${REMOTE_DB}"
echo "Backup File: ${BACKUP_FILE}"
echo ""

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

echo "Step 1/3: Creating backup on remote server..."
echo "You will be prompted for the SSH password for ${REMOTE_USER}@${REMOTE_HOST}"
echo ""

# Create backup on remote server
ssh "${REMOTE_USER}@${REMOTE_HOST}" "pg_dump -U ${REMOTE_DB_USER} -d ${REMOTE_DB} --clean --if-exists --create" > "${BACKUP_DIR}/${BACKUP_FILE}"

if [ $? -eq 0 ]; then
    echo "✓ Backup created successfully on remote server"
else
    echo "✗ Failed to create backup on remote server"
    exit 1
fi

echo ""
echo "Step 2/3: Verifying backup integrity..."

# Check if backup file is not empty
if [ -s "${BACKUP_DIR}/${BACKUP_FILE}" ]; then
    BACKUP_SIZE=$(du -h "${BACKUP_DIR}/${BACKUP_FILE}" | cut -f1)
    echo "✓ Backup file size: ${BACKUP_SIZE}"

    # Check if backup contains essential tables
    if grep -q "CREATE TABLE" "${BACKUP_DIR}/${BACKUP_FILE}"; then
        echo "✓ Backup contains table definitions"
    else
        echo "✗ Warning: Backup may be incomplete (no CREATE TABLE found)"
    fi

    if grep -q "COPY" "${BACKUP_DIR}/${BACKUP_FILE}"; then
        echo "✓ Backup contains data"
    else
        echo "✗ Warning: Backup may be incomplete (no data found)"
    fi
else
    echo "✗ Backup file is empty or was not created"
    exit 1
fi

echo ""
echo "Step 3/3: Backup summary"
echo "════════════════════════════════════════════════════════════"
echo "Backup Location: ${BACKUP_DIR}/${BACKUP_FILE}"
echo "Backup Size: ${BACKUP_SIZE}"
echo "Timestamp: ${TIMESTAMP}"
echo ""
echo "✓ BACKUP COMPLETED SUCCESSFULLY"
echo ""
echo "Next steps:"
echo "  1. Review the backup file: ${BACKUP_DIR}/${BACKUP_FILE}"
echo "  2. Run './scripts/setup-local-test.sh ${BACKUP_FILE}' to deploy locally"
echo ""
