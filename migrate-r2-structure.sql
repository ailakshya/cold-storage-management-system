-- R2 Migration: Reorganize R2 keys to year/thock structure
-- This updates the r2_key column in media_sync_queue to match local file paths

BEGIN;

-- Backup current state
CREATE TABLE IF NOT EXISTS media_sync_queue_r2_backup AS
SELECT * FROM media_sync_queue WHERE r2_synced = true;

-- Update R2 keys to match new local file path structure
-- Format: {year}/{thock}/{media_type}_{filename}
UPDATE media_sync_queue msq
SET r2_key = CONCAT(
    CASE
        WHEN msq.media_source = 'gate_pass' THEN
            (SELECT EXTRACT(YEAR FROM gpm.created_at)::text FROM gate_pass_media gpm WHERE gpm.id = msq.media_id)
        WHEN msq.media_source = 'room_entry' THEN
            (SELECT EXTRACT(YEAR FROM rem.created_at)::text FROM room_entry_media rem WHERE rem.id = msq.media_id)
        ELSE '2026'
    END,
    '/',
    -- Get thock number
    (SELECT thock_number FROM gate_pass_media gpm WHERE msq.media_source = 'gate_pass' AND gpm.id = msq.media_id
     UNION ALL
     SELECT thock_number FROM room_entry_media rem WHERE msq.media_source = 'room_entry' AND rem.id = msq.media_id
     LIMIT 1),
    '/',
    -- Get media type prefix
    CASE
        WHEN msq.media_source = 'gate_pass' THEN
            (SELECT media_type FROM gate_pass_media gpm WHERE gpm.id = msq.media_id)
        WHEN msq.media_source = 'room_entry' THEN
            (SELECT media_type FROM room_entry_media rem WHERE rem.id = msq.media_id)
        ELSE 'unknown'
    END,
    '_',
    -- Get filename
    (SELECT file_name FROM gate_pass_media gpm WHERE msq.media_source = 'gate_pass' AND gpm.id = msq.media_id
     UNION ALL
     SELECT file_name FROM room_entry_media rem WHERE msq.media_source = 'room_entry' AND rem.id = msq.media_id
     LIMIT 1)
)
WHERE r2_synced = true
  AND (r2_key LIKE 'room-entry/%' OR r2_key LIKE 'gate-pass/%' OR r2_key LIKE '%/%/%');

-- Verification
DO $$
DECLARE
    old_format INT;
    new_format INT;
    total_synced INT;
BEGIN
    SELECT COUNT(*) INTO old_format
    FROM media_sync_queue
    WHERE r2_synced = true
      AND (r2_key LIKE 'room-entry/%' OR r2_key LIKE 'gate-pass/%');

    SELECT COUNT(*) INTO new_format
    FROM media_sync_queue
    WHERE r2_synced = true
      AND r2_key ~ '^\d{4}/[^/]+/[^/]+_';

    SELECT COUNT(*) INTO total_synced
    FROM media_sync_queue
    WHERE r2_synced = true;

    RAISE NOTICE 'R2 Migration Results:';
    RAISE NOTICE '  - Total synced: %', total_synced;
    RAISE NOTICE '  - New format: %', new_format;
    RAISE NOTICE '  - Old format remaining: %', old_format;

    IF old_format > 0 THEN
        RAISE WARNING 'Still have % records in old format - check thock_number format', old_format;
    END IF;
END $$;

-- Show sample of new keys
SELECT 'Sample new R2 keys:' as info;
SELECT media_source, r2_key
FROM media_sync_queue
WHERE r2_synced = true
ORDER BY created_at DESC
LIMIT 5;

COMMIT;

-- Rollback instructions (if needed):
-- BEGIN;
-- DELETE FROM media_sync_queue WHERE r2_synced = true;
-- INSERT INTO media_sync_queue SELECT * FROM media_sync_queue_r2_backup;
-- COMMIT;
-- DROP TABLE media_sync_queue_r2_backup;
