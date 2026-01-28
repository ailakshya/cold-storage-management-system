#!/bin/bash
#
# Start Application with Local Test Database
# This script starts the application connected to the Docker PostgreSQL container
#

set -e

CONTAINER_NAME="cold-storage-test-db"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Starting Cold Storage Application (Local Test)           ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Check if container is running
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    echo "✗ Container $CONTAINER_NAME is not running"
    echo ""
    echo "Please run the setup script first:"
    echo "  ./scripts/setup-local-test.sh <backup_file>"
    echo ""
    echo "Or if you already have a backup:"
    echo "  LATEST=\$(ls -t backups/remote/*.sql | head -1)"
    echo "  ./scripts/setup-local-test.sh \$LATEST"
    exit 1
fi

echo "✓ Database container is running"
echo "✓ Database: localhost:5433/cold_db"
echo ""

# Export environment variables for the app
export DB_HOST=localhost
export DB_PORT=5433
export DB_USER=postgres
export DB_PASSWORD=testpassword123
export DB_NAME=cold_db
export DB_SSLMODE=disable
export JWT_SECRET=test-jwt-secret-for-local-testing-only
export SERVER_PORT=8080
export BACKUP_DIR=./backups

echo "Starting application in employee mode..."
echo "Press Ctrl+C to stop"
echo ""
echo "════════════════════════════════════════════════════════════"
echo ""

# Start the application
./bin/server -mode employee
