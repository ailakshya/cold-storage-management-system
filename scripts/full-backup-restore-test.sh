#!/bin/bash
#
# Full Backup and Restore Test
# This script performs a complete backup from remote and sets up local testing environment
#

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Cold Storage - Full Backup & Restore Test                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "This script will:"
echo "  1. Create a complete backup from remote server (192.168.1.134)"
echo "  2. Set up a local Docker PostgreSQL container"
echo "  3. Restore the backup to the local container"
echo "  4. Verify the restoration"
echo ""
read -p "Do you want to continue? (y/n): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Operation cancelled."
    exit 0
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "Phase 1: Creating Backup from Remote Server"
echo "════════════════════════════════════════════════════════════"
echo ""

# Run backup script
"${SCRIPT_DIR}/backup-from-remote.sh"

if [ $? -ne 0 ]; then
    echo "✗ Backup failed. Exiting."
    exit 1
fi

# Get the latest backup file
LATEST_BACKUP=$(ls -t backups/remote/cold_db_remote_*.sql 2>/dev/null | head -1)

if [ -z "$LATEST_BACKUP" ]; then
    echo "✗ No backup file found. Exiting."
    exit 1
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "Phase 2: Setting Up Local Testing Environment"
echo "════════════════════════════════════════════════════════════"
echo ""

# Run setup script with the latest backup
"${SCRIPT_DIR}/setup-local-test.sh" "$LATEST_BACKUP"

if [ $? -ne 0 ]; then
    echo "✗ Local setup failed. Exiting."
    exit 1
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✓ COMPLETE BACKUP & RESTORE TEST SUCCESSFUL"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Your local testing environment is ready!"
echo ""
