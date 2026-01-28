#!/bin/sh

# Docker entrypoint script
# Verifies top-level directory ownership and starts the application as appuser.
#
# NOTE: ZFS datasets should be created with UID 1000 ownership (one-time setup).
# This script only checks top-level dirs — it does NOT do recursive chown,
# which would be extremely slow on large ZFS pools (7.5TB+).

echo "[Entrypoint] Checking volume directory ownership..."

for dir in /mass-pool/shared /mass-pool/archives /mass-pool/trash /mass-pool/backups /fast-pool/data; do
  if [ -d "$dir" ]; then
    # Only fix top-level dir ownership, not recursive
    chown appuser:appuser "$dir" 2>/dev/null || true
  fi
done

# Fix app directory ownership
chown -R appuser:appuser /app 2>/dev/null || true

echo "[Entrypoint] Switching to appuser and starting application..."

# Switch to appuser and execute the original command
exec su-exec appuser "$@"
