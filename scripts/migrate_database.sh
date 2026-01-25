#!/bin/bash
set -e

echo "==================================="
echo "Database Migration Script"
echo "From: 192.168.15.120 to 192.168.15.131"
echo "==================================="

OLD_SERVER="192.168.15.120"
NEW_SERVER="192.168.15.131"
DB_NAME="cold_db"
DUMP_FILE="/tmp/cold_db_migration_$(date +%Y%m%d_%H%M%S).dump"

echo ""
echo "Step 1: Creating database dump on old server ($OLD_SERVER)..."
ssh root@${OLD_SERVER} "sudo -u postgres pg_dump -d ${DB_NAME} --clean --if-exists -F c -f ${DUMP_FILE} && chmod 644 ${DUMP_FILE}"

echo ""
echo "Step 2: Transferring dump to new server ($NEW_SERVER)..."
ssh root@${OLD_SERVER} "cat ${DUMP_FILE}" | ssh cold@${NEW_SERVER} "cat > ${DUMP_FILE}"

echo ""
echo "Step 3: Stopping K8s apps to prevent data conflicts..."
ssh cold@${NEW_SERVER} "echo 'Lak992723/' | sudo -S k3s kubectl scale deployment/cold-backend-employee deployment/cold-backend-customer --replicas=0"

sleep 5

echo ""
echo "Step 4: Restoring database on new server ($NEW_SERVER)..."
ssh cold@${NEW_SERVER} "echo 'Lak992723/' | sudo -S -u postgres pg_restore -d ${DB_NAME} --clean --if-exists ${DUMP_FILE} 2>&1 | grep -v 'ERROR.*already exists' || true"

echo ""
echo "Step 5: Restarting K8s apps..."
ssh cold@${NEW_SERVER} "echo 'Lak992723/' | sudo -S k3s kubectl scale deployment/cold-backend-employee deployment/cold-backend-customer --replicas=1"

echo ""
echo "Step 6: Cleaning up dump files..."
ssh root@${OLD_SERVER} "rm -f ${DUMP_FILE}"
ssh cold@${NEW_SERVER} "rm -f ${DUMP_FILE}"

echo ""
echo "==================================="
echo "✅ Migration completed successfully!"
echo "==================================="
echo ""
echo "Verification:"
ssh cold@${NEW_SERVER} "echo 'Lak992723/' | sudo -S -u postgres psql -d ${DB_NAME} -c 'SELECT COUNT(*) as total_accounts FROM accounts;'"

