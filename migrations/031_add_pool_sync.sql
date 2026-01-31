-- Migration 031: Add storage pool sync to RustFS/MinIO
-- Syncs all files from local pools (bulk, highspeed, archives, backups) to NAS S3

CREATE TABLE IF NOT EXISTS pool_sync_queue (
    id            BIGSERIAL PRIMARY KEY,

    -- Pool identification
    pool_name     VARCHAR(20) NOT NULL
                  CHECK (pool_name IN ('bulk', 'highspeed', 'archives', 'backups')),

    -- File identity (relative path within the pool)
    relative_path TEXT NOT NULL,

    -- S3 destination key: {pool_name}/{relative_path}
    s3_key        TEXT NOT NULL,

    -- File metadata at time of enqueue (for change detection)
    file_size     BIGINT NOT NULL DEFAULT 0,
    file_mtime    TIMESTAMP NOT NULL,

    -- Sync state
    sync_status   VARCHAR(20) NOT NULL DEFAULT 'pending'
                  CHECK (sync_status IN ('pending', 'uploading', 'synced', 'failed', 'skipped')),

    -- Retry logic
    retry_count   INTEGER NOT NULL DEFAULT 0,
    max_retries   INTEGER NOT NULL DEFAULT 5,
    last_error    TEXT,

    -- Timestamps
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at    TIMESTAMP,
    completed_at  TIMESTAMP,
    next_retry_at TIMESTAMP,

    -- Prevent duplicate entries for the same file in the same pool
    UNIQUE (pool_name, relative_path)
);

-- Worker picks pending items efficiently
CREATE INDEX IF NOT EXISTS idx_pool_sync_pending
    ON pool_sync_queue (sync_status, next_retry_at)
    WHERE sync_status IN ('pending', 'failed');

-- Stats queries by pool
CREATE INDEX IF NOT EXISTS idx_pool_sync_pool
    ON pool_sync_queue (pool_name, sync_status);

-- Pool scan state: tracks the last-completed scan for each pool
CREATE TABLE IF NOT EXISTS pool_sync_scan_state (
    pool_name      VARCHAR(20) PRIMARY KEY
                   CHECK (pool_name IN ('bulk', 'highspeed', 'archives', 'backups')),
    last_scan_at   TIMESTAMP,
    files_found    BIGINT NOT NULL DEFAULT 0,
    files_enqueued BIGINT NOT NULL DEFAULT 0,
    scan_duration_ms BIGINT NOT NULL DEFAULT 0,
    is_scanning    BOOLEAN NOT NULL DEFAULT FALSE
);

-- Seed initial rows
INSERT INTO pool_sync_scan_state (pool_name) VALUES
    ('bulk'), ('highspeed'), ('archives'), ('backups')
ON CONFLICT DO NOTHING;
