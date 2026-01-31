-- Migration 031: Add storage pool sync to RustFS/MinIO
-- Syncs all files from local pools (bulk, highspeed, archives, backups) to NAS S3

DO $$ BEGIN
    CREATE TABLE IF NOT EXISTS pool_sync_queue (
        id            BIGSERIAL PRIMARY KEY,
        pool_name     VARCHAR(20) NOT NULL,
        relative_path TEXT NOT NULL,
        s3_key        TEXT NOT NULL,
        file_size     BIGINT NOT NULL DEFAULT 0,
        file_mtime    TIMESTAMP NOT NULL,
        sync_status   VARCHAR(20) NOT NULL DEFAULT 'pending',
        retry_count   INTEGER NOT NULL DEFAULT 0,
        max_retries   INTEGER NOT NULL DEFAULT 5,
        last_error    TEXT,
        created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        started_at    TIMESTAMP,
        completed_at  TIMESTAMP,
        next_retry_at TIMESTAMP,
        UNIQUE (pool_name, relative_path)
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_pool_sync_pending
    ON pool_sync_queue (sync_status, next_retry_at)
    WHERE sync_status IN ('pending', 'failed');

CREATE INDEX IF NOT EXISTS idx_pool_sync_pool
    ON pool_sync_queue (pool_name, sync_status);

DO $$ BEGIN
    CREATE TABLE IF NOT EXISTS pool_sync_scan_state (
        pool_name      VARCHAR(20) PRIMARY KEY,
        last_scan_at   TIMESTAMP,
        files_found    BIGINT NOT NULL DEFAULT 0,
        files_enqueued BIGINT NOT NULL DEFAULT 0,
        scan_duration_ms BIGINT NOT NULL DEFAULT 0,
        is_scanning    BOOLEAN NOT NULL DEFAULT FALSE
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

INSERT INTO pool_sync_scan_state (pool_name) VALUES
    ('bulk'), ('highspeed'), ('archives'), ('backups')
ON CONFLICT DO NOTHING;
