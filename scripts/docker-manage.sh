#!/bin/bash
#
# Docker Management Script
# Easy commands to manage the Docker testing environment
#

COMPOSE_FILE="docker-compose.test.yml"

# Determine docker compose command
if docker compose version &> /dev/null 2>&1; then
    DOCKER_COMPOSE="docker compose"
else
    DOCKER_COMPOSE="docker-compose"
fi

case "$1" in
    start)
        echo "Starting all services..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" start
        echo "✓ All services started"
        ;;

    stop)
        echo "Stopping all services..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" stop
        echo "✓ All services stopped"
        ;;

    restart)
        echo "Restarting all services..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" restart
        echo "✓ All services restarted"
        ;;

    restart-app)
        echo "Restarting application..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" restart app-employee app-customer
        echo "✓ Application restarted"
        ;;

    logs)
        if [ -z "$2" ]; then
            $DOCKER_COMPOSE -f "$COMPOSE_FILE" logs -f
        else
            $DOCKER_COMPOSE -f "$COMPOSE_FILE" logs -f "$2"
        fi
        ;;

    status)
        echo "Service Status:"
        echo "════════════════════════════════════════════════════════════"
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" ps
        ;;

    shell)
        SERVICE=${2:-app-employee}
        echo "Opening shell in $SERVICE..."
        docker exec -it "cold-storage-$SERVICE" sh
        ;;

    db)
        echo "Connecting to database..."
        docker exec -it cold-storage-postgres psql -U postgres -d cold_db
        ;;

    redis-cli)
        echo "Connecting to Redis..."
        docker exec -it cold-storage-redis redis-cli
        ;;

    rebuild)
        echo "Rebuilding application..."
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" build --no-cache
        $DOCKER_COMPOSE -f "$COMPOSE_FILE" up -d app-employee app-customer
        echo "✓ Application rebuilt and restarted"
        ;;

    clean)
        echo "Removing all containers and volumes..."
        read -p "Are you sure? This will delete all data! (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            $DOCKER_COMPOSE -f "$COMPOSE_FILE" down -v
            echo "✓ All containers and volumes removed"
        else
            echo "Cancelled"
        fi
        ;;

    backup-db)
        TIMESTAMP=$(date +%Y%m%d_%H%M%S)
        BACKUP_FILE="backups/docker-local/cold_db_local_${TIMESTAMP}.sql"
        mkdir -p backups/docker-local
        echo "Creating backup: $BACKUP_FILE"
        docker exec cold-storage-postgres pg_dump -U postgres -d cold_db --clean --if-exists --create > "$BACKUP_FILE"
        echo "✓ Backup created: $BACKUP_FILE"
        ;;

    restore-db)
        if [ -z "$2" ]; then
            echo "Usage: $0 restore-db <backup.sql>"
            exit 1
        fi
        echo "Restoring from: $2"
        docker exec -i cold-storage-postgres psql -U postgres -d postgres < "$2"
        echo "✓ Database restored"
        ;;

    stats)
        echo "Container Resource Usage:"
        echo "════════════════════════════════════════════════════════════"
        docker stats --no-stream $(docker ps --filter "name=cold-storage" --format "{{.Names}}")
        ;;

    urls)
        echo "Application URLs:"
        echo "════════════════════════════════════════════════════════════"
        echo "  Employee Portal:   http://localhost:8080"
        echo "  Customer Portal:   http://localhost:8081"
        echo "  Monitoring:        http://localhost:9090"
        echo "  PostgreSQL:        localhost:5433"
        echo "  Redis:             localhost:6379"
        echo "  TimescaleDB:       localhost:5434"
        ;;

    *)
        echo "Cold Storage Docker Management"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  start              Start all services"
        echo "  stop               Stop all services"
        echo "  restart            Restart all services"
        echo "  restart-app        Restart application only"
        echo "  logs [service]     View logs (all or specific service)"
        echo "  status             Show service status"
        echo "  shell [service]    Open shell in container (default: app-employee)"
        echo "  db                 Connect to PostgreSQL database"
        echo "  redis-cli          Connect to Redis CLI"
        echo "  rebuild            Rebuild and restart application"
        echo "  clean              Remove all containers and volumes"
        echo "  backup-db          Create database backup"
        echo "  restore-db <file>  Restore database from backup"
        echo "  stats              Show container resource usage"
        echo "  urls               Show application URLs"
        echo ""
        echo "Examples:"
        echo "  $0 logs app-employee           # View employee app logs"
        echo "  $0 shell postgres              # Open shell in postgres container"
        echo "  $0 backup-db                   # Create database backup"
        echo "  $0 restore-db backup.sql       # Restore from backup"
        ;;
esac
