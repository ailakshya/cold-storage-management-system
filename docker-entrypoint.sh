#!/bin/sh

# Docker entrypoint script
# Fixes permissions for volume-mounted directories and starts the application

echo "[Entrypoint] Fixing permissions for volume directories..."

# Fix ownership of volume-mounted directories if they exist
if [ -d "/mass-pool" ]; then
    chown -R appuser:appuser /mass-pool 2>/dev/null || true
    echo "[Entrypoint] Fixed permissions for /mass-pool"
fi

if [ -d "/fast-pool" ]; then
    chown -R appuser:appuser /fast-pool 2>/dev/null || true
    echo "[Entrypoint] Fixed permissions for /fast-pool"
fi

if [ -d "/app/backups" ]; then
    chown -R appuser:appuser /app/backups 2>/dev/null || true
    echo "[Entrypoint] Fixed permissions for /app/backups"
fi

echo "[Entrypoint] Switching to appuser and starting application..."

# Switch to appuser and execute the original command
exec su-exec appuser "$@"
