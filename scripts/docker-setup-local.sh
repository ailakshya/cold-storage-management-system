#!/bin/bash
#
# Docker Setup for Local Testing
# This script sets up the complete Docker environment with database restore
#

set -e

BACKUP_FILE=$1
COMPOSE_FILE="docker-compose.test.yml"

echo "╔════════════════════════════════════════════════════════════╗"
echo "║  Cold Storage - Docker Local Testing Setup                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Check if docker is installed
if ! command -v docker &> /dev/null; then
    echo "✗ Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if docker-compose is installed
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "✗ Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Determine docker compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

echo "✓ Docker and Docker Compose are installed"
echo ""

# Check if backup file is provided
if [ -z "$BACKUP_FILE" ]; then
    # Find latest backup
    BACKUP_FILE=$(ls -t backups/remote/*.sql 2>/dev/null | head -1)
    if [ -z "$BACKUP_FILE" ]; then
        echo "⚠ No backup file found."
        echo ""
        echo "Please either:"
        echo "  1. Provide a backup file: $0 <backup.sql>"
        echo "  2. Create a backup first: ./scripts/backup-with-password.exp"
        exit 1
    fi
    echo "ℹ Using latest backup: $BACKUP_FILE"
else
    if [ ! -f "$BACKUP_FILE" ]; then
        echo "✗ Backup file not found: $BACKUP_FILE"
        exit 1
    fi
fi

echo ""
echo "Step 1/6: Stopping any existing containers..."
$DOCKER_COMPOSE -f "$COMPOSE_FILE" down -v 2>/dev/null || true
echo "✓ Cleaned up existing containers"

echo ""
echo "Step 2/6: Building application Docker image..."
$DOCKER_COMPOSE -f "$COMPOSE_FILE" build --no-cache
echo "✓ Application image built"

echo ""
echo "Step 3/6: Starting PostgreSQL and Redis..."
$DOCKER_COMPOSE -f "$COMPOSE_FILE" up -d postgres redis timescaledb

# Wait for PostgreSQL to be ready
echo "  Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker exec cold-storage-postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo "✓ PostgreSQL is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "✗ PostgreSQL failed to start within 30 seconds"
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" logs postgres
        exit 1
    fi
    sleep 1
done

# Wait for Redis to be ready
echo "  Waiting for Redis to be ready..."
for i in {1..30}; do
    if docker exec cold-storage-redis redis-cli ping > /dev/null 2>&1; then
        echo "✓ Redis is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "✗ Redis failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done

echo ""
echo "Step 4/6: Restoring database from backup..."
echo "  Backup: $BACKUP_FILE"

# Copy backup into container and restore
docker cp "$BACKUP_FILE" cold-storage-postgres:/tmp/backup.sql
docker exec -i cold-storage-postgres psql -U postgres -d postgres < "$BACKUP_FILE" 2>&1 | grep -v "^$" | head -20

echo "✓ Database restored"

echo ""
echo "Step 5/6: Verifying database restoration..."

# Check if database exists
DB_EXISTS=$(docker exec cold-storage-postgres psql -U postgres -lqt | cut -d \| -f 1 | grep -w cold_db | wc -l)
if [ "$DB_EXISTS" -gt 0 ]; then
    TABLE_COUNT=$(docker exec cold-storage-postgres psql -U postgres -d cold_db -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d ' ')
    echo "✓ Database 'cold_db' verified with $TABLE_COUNT tables"
else
    echo "✗ Database 'cold_db' was not created"
    exit 1
fi

echo ""
echo "Step 6/6: Starting application services..."
$DOCKER_COMPOSE -f "$COMPOSE_FILE" up -d app-employee app-customer

# Wait for apps to be ready
sleep 5

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✓ DOCKER ENVIRONMENT READY"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Services Running:"
echo "  ✓ PostgreSQL:        localhost:5433"
echo "  ✓ Redis:             localhost:6379"
echo "  ✓ TimescaleDB:       localhost:5434"
echo "  ✓ Employee Portal:   http://localhost:8080"
echo "  ✓ Customer Portal:   http://localhost:8081"
echo "  ✓ Monitoring:        http://localhost:9090"
echo ""
echo "Quick Commands:"
echo "  View all logs:       $DOCKER_COMPOSE -f $COMPOSE_FILE logs -f"
echo "  View app logs:       $DOCKER_COMPOSE -f $COMPOSE_FILE logs -f app-employee"
echo "  Stop all:            $DOCKER_COMPOSE -f $COMPOSE_FILE stop"
echo "  Remove all:          $DOCKER_COMPOSE -f $COMPOSE_FILE down -v"
echo "  Restart app:         $DOCKER_COMPOSE -f $COMPOSE_FILE restart app-employee"
echo ""
echo "Database Access:"
echo "  Connect:             docker exec -it cold-storage-postgres psql -U postgres -d cold_db"
echo "  Redis CLI:           docker exec -it cold-storage-redis redis-cli"
echo ""
echo "Next Steps:"
echo "  1. Open http://localhost:8080 in your browser"
echo "  2. Login with existing user credentials"
echo "  3. Check logs: $DOCKER_COMPOSE -f $COMPOSE_FILE logs -f app-employee"
echo ""
