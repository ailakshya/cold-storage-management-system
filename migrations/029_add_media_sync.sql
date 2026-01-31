-- Migration 029: Add media sync queue and cloud sync tracking
-- Supports 3-2-1 backup strategy: Local disk + MinIO (NAS) + Cloudflare R2

-- Sync queue for background S3 uploads to both R2 and MinIO
CREATE TABLE IF NOT EXISTS media_sync_queue (
    id SERIAL PRIMARY KEY,

    -- Source identification
    media_source VARCHAR(20) NOT NULL CHECK (media_source IN ('room_entry', 'gate_pass')),
    media_id INTEGER NOT NULL,

    -- File info (denormalized for worker efficiency)
    local_file_path TEXT NOT NULL,
    r2_key TEXT NOT NULL,
    file_size BIGINT,

    -- Sync state
    sync_status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (sync_status IN ('pending', 'uploading', 'synced', 'failed', 'skipped')),

    -- Per-target sync tracking (independent)
    local_synced BOOLEAN NOT NULL DEFAULT TRUE,
    nas_synced BOOLEAN NOT NULL DEFAULT FALSE,
    r2_synced BOOLEAN NOT NULL DEFAULT FALSE,

    -- Where the file was originally uploaded (for fallback scenarios)
    primary_location VARCHAR(10) NOT NULL DEFAULT 'local'
        CHECK (primary_location IN ('local', 'nas', 'r2')),

    -- Retry logic
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    next_retry_at TIMESTAMP
);

-- Index for the background worker to pick pending items efficiently
CREATE INDEX IF NOT EXISTS idx_media_sync_pending ON media_sync_queue(sync_status, next_retry_at)
    WHERE sync_status IN ('pending', 'failed');

-- Index for looking up sync status by media source
CREATE INDEX IF NOT EXISTS idx_media_sync_source ON media_sync_queue(media_source, media_id);

-- Add cloud sync columns to existing media tables
ALTER TABLE room_entry_media ADD COLUMN IF NOT EXISTS cloud_synced BOOLEAN DEFAULT FALSE;
ALTER TABLE room_entry_media ADD COLUMN IF NOT EXISTS r2_key TEXT;

ALTER TABLE gate_pass_media ADD COLUMN IF NOT EXISTS cloud_synced BOOLEAN DEFAULT FALSE;
ALTER TABLE gate_pass_media ADD COLUMN IF NOT EXISTS r2_key TEXT;
