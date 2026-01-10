-- Migration 026: Add snapshot tracking for change-based backups
-- This enables efficient backups by only creating snapshots when data changes

-- Table to track snapshot metadata
CREATE TABLE IF NOT EXISTS snapshot_metadata (
    id SERIAL PRIMARY KEY,
    snapshot_type VARCHAR(20) NOT NULL DEFAULT 'r2', -- 'r2', 'local'
    snapshot_key VARCHAR(500) NOT NULL, -- Path in R2 or local filesystem
    db_version BIGINT NOT NULL, -- pg_current_xact_id() at snapshot time
    data_checksum VARCHAR(64), -- Optional MD5/SHA hash of key tables
    size_bytes BIGINT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    season VARCHAR(20), -- e.g., '2025-26' for season-based retention
    UNIQUE(snapshot_type, snapshot_key)
);

-- Table to track last change time per table (for efficient change detection)
CREATE TABLE IF NOT EXISTS table_change_tracking (
    table_name VARCHAR(100) PRIMARY KEY,
    last_modified TIMESTAMPTZ DEFAULT NOW(),
    row_count BIGINT DEFAULT 0,
    checksum VARCHAR(64) -- Optional checksum
);

-- Initialize tracking for main tables
INSERT INTO table_change_tracking (table_name, last_modified, row_count) VALUES
    ('entries', NOW(), 0),
    ('room_entries', NOW(), 0),
    ('customers', NOW(), 0),
    ('gate_passes', NOW(), 0),
    ('rent_payments', NOW(), 0),
    ('ledger_entries', NOW(), 0)
ON CONFLICT (table_name) DO NOTHING;

-- Function to update change tracking on any modification
CREATE OR REPLACE FUNCTION update_change_tracking()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE table_change_tracking
    SET last_modified = NOW(),
        row_count = (SELECT COUNT(*) FROM ONLY NEW.*)
    WHERE table_name = TG_TABLE_NAME;

    IF NOT FOUND THEN
        INSERT INTO table_change_tracking (table_name, last_modified, row_count)
        VALUES (TG_TABLE_NAME, NOW(), 1);
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Simple change tracking trigger (lightweight - just updates timestamp)
CREATE OR REPLACE FUNCTION track_table_change()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE table_change_tracking SET last_modified = NOW() WHERE table_name = TG_TABLE_NAME;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Add triggers to main tables
DO $$
DECLARE
    t TEXT;
BEGIN
    FOR t IN SELECT unnest(ARRAY['entries', 'room_entries', 'customers', 'gate_passes', 'rent_payments', 'ledger_entries'])
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS track_changes_%I ON %I', t, t);
        EXECUTE format('CREATE TRIGGER track_changes_%I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH STATEMENT EXECUTE FUNCTION track_table_change()', t, t);
    END LOOP;
END $$;

-- View to get current change state (for snapshot comparison)
CREATE OR REPLACE VIEW v_db_change_state AS
SELECT
    MAX(last_modified) as last_modified,
    pg_current_xact_id()::TEXT as xact_id,
    (SELECT SUM(row_count) FROM table_change_tracking) as total_rows
FROM table_change_tracking;

-- Index for faster snapshot queries
CREATE INDEX IF NOT EXISTS idx_snapshot_metadata_created ON snapshot_metadata(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_snapshot_metadata_season ON snapshot_metadata(season);
