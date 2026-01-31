-- Migration 030: Reorganize media file paths to year/thock structure
-- Safe to run multiple times (idempotent)

BEGIN;

-- Backup current state for rollback
CREATE TABLE IF NOT EXISTS room_entry_media_backup_030 AS
SELECT * FROM room_entry_media WHERE file_path NOT LIKE 'Room Config/%/%/%';

CREATE TABLE IF NOT EXISTS gate_pass_media_backup_030 AS
SELECT * FROM gate_pass_media WHERE file_path NOT LIKE 'Room Config/%/%/%';

CREATE TABLE IF NOT EXISTS media_sync_queue_backup_030 AS
SELECT * FROM media_sync_queue WHERE local_file_path NOT LIKE '%/Room Config/%/%/%';

-- Update room_entry_media: Room Config/file.jpg → Room Config/YEAR/THOCK/file.jpg
UPDATE room_entry_media
SET file_path = CONCAT(
    'Room Config/',
    -- Extract year from thock_number (e.g., "TH-2026-0042" → "2026")
    CASE
        WHEN split_part(thock_number, '-', 2) ~ '^[0-9]{4}$'
        THEN split_part(thock_number, '-', 2)
        ELSE EXTRACT(YEAR FROM CURRENT_DATE)::text
    END,
    '/',
    thock_number,
    '/',
    file_name
)
WHERE file_path NOT LIKE 'Room Config/%/%/%'  -- Skip already migrated
  AND file_path LIKE 'Room Config/%';         -- Only flat structure

-- Update gate_pass_media
UPDATE gate_pass_media
SET file_path = CONCAT(
    'Room Config/',
    CASE
        WHEN split_part(thock_number, '-', 2) ~ '^[0-9]{4}$'
        THEN split_part(thock_number, '-', 2)
        ELSE EXTRACT(YEAR FROM CURRENT_DATE)::text
    END,
    '/',
    thock_number,
    '/',
    file_name
)
WHERE file_path NOT LIKE 'Room Config/%/%/%'
  AND file_path LIKE 'Room Config/%';

-- Update media_sync_queue absolute paths
WITH media_info AS (
    SELECT
        'room_entry' as source,
        id,
        thock_number,
        file_name
    FROM room_entry_media
    UNION ALL
    SELECT
        'gate_pass' as source,
        id,
        thock_number,
        file_name
    FROM gate_pass_media
)
UPDATE media_sync_queue msq
SET local_file_path = CONCAT(
    '/mass-pool/shared/Room Config/',
    CASE
        WHEN split_part(m.thock_number, '-', 2) ~ '^[0-9]{4}$'
        THEN split_part(m.thock_number, '-', 2)
        ELSE EXTRACT(YEAR FROM CURRENT_DATE)::text
    END,
    '/',
    m.thock_number,
    '/',
    m.file_name
)
FROM media_info m
WHERE msq.media_source = m.source
  AND msq.media_id = m.id
  AND msq.local_file_path NOT LIKE '%/Room Config/%/%/%'
  AND msq.local_file_path LIKE '%/Room Config/%';

-- Verification
DO $$
DECLARE
    flat_count INT;
    room_updated INT;
    gate_updated INT;
    sync_updated INT;
BEGIN
    -- Check for remaining flat structure files
    SELECT COUNT(*) INTO flat_count
    FROM room_entry_media
    WHERE file_path ~ '^Room Config/[^/]+\.(jpg|jpeg|png|mp4|mov|MOV)$';

    IF flat_count > 0 THEN
        RAISE WARNING 'Found % room_entry_media records still in flat structure', flat_count;
    END IF;

    -- Count updated records
    SELECT COUNT(*) INTO room_updated FROM room_entry_media
    WHERE file_path LIKE 'Room Config/%/%/%';

    SELECT COUNT(*) INTO gate_updated FROM gate_pass_media
    WHERE file_path LIKE 'Room Config/%/%/%';

    SELECT COUNT(*) INTO sync_updated FROM media_sync_queue
    WHERE local_file_path LIKE '%/Room Config/%/%/%';

    RAISE NOTICE 'Migration complete:';
    RAISE NOTICE '  - room_entry_media: % records in new format', room_updated;
    RAISE NOTICE '  - gate_pass_media: % records in new format', gate_updated;
    RAISE NOTICE '  - media_sync_queue: % records in new format', sync_updated;
END $$;

COMMIT;

-- Rollback instructions (if needed):
-- BEGIN;
-- DELETE FROM room_entry_media WHERE file_path LIKE 'Room Config/%/%/%';
-- INSERT INTO room_entry_media SELECT * FROM room_entry_media_backup_030;
-- DELETE FROM gate_pass_media WHERE file_path LIKE 'Room Config/%/%/%';
-- INSERT INTO gate_pass_media SELECT * FROM gate_pass_media_backup_030;
-- DELETE FROM media_sync_queue WHERE local_file_path LIKE '%/Room Config/%/%/%';
-- INSERT INTO media_sync_queue SELECT * FROM media_sync_queue_backup_030;
-- COMMIT;
-- DROP TABLE room_entry_media_backup_030;
-- DROP TABLE gate_pass_media_backup_030;
-- DROP TABLE media_sync_queue_backup_030;
