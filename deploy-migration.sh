#!/bin/bash
set -euo pipefail

echo "=========================================="
echo "Media File Migration - Full Deployment"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $*"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

# Check we're in the right directory
if [[ ! -f "docker-compose.production.yml" ]]; then
    error "Must run from ~/cold-storage directory"
fi

# Step 1: Create directories
log "Step 1: Creating directories..."
mkdir -p migrations scripts backups
success "Directories created"

# Step 2: Create SQL migration file
log "Step 2: Creating SQL migration file..."
cat > migrations/030_migrate_media_paths.sql << 'EOF'
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
    SELECT COUNT(*) INTO flat_count
    FROM room_entry_media
    WHERE file_path ~ '^Room Config/[^/]+\.(jpg|jpeg|png|mp4|mov|MOV)$';

    IF flat_count > 0 THEN
        RAISE WARNING 'Found % room_entry_media records still in flat structure', flat_count;
    END IF;

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
EOF
success "SQL migration created"

# Step 3: Create bash migration script
log "Step 3: Creating file migration script..."
cat > scripts/migrate-media-files.sh << 'SCRIPT_EOF'
#!/bin/bash
set -euo pipefail

BASE_DIR="$HOME/cold-storage/shared/Room Config"
LOG_FILE="/tmp/media-migration-$(date +%Y%m%d_%H%M%S).log"
DRY_RUN="${DRY_RUN:-true}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; }
error() { echo -e "${RED}[ERROR]${NC} $*" | tee -a "$LOG_FILE"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $*" | tee -a "$LOG_FILE"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*" | tee -a "$LOG_FILE"; }

extract_year() {
    local thock="$1"
    local year=$(echo "$thock" | grep -oP 'TH-\K\d{4}' || echo "")
    if [[ -z "$year" ]]; then
        year=$(date +%Y)
    fi
    echo "$year"
}

extract_thock_from_filename() {
    local filename="$1"
    local thock=$(echo "$filename" | grep -oP 'TH-\d{4}-\d+' | head -1)
    if [[ -z "$thock" ]]; then
        local partial=$(echo "$filename" | grep -oP 'Thok_\d+-\d+' | head -1)
        if [[ -n "$partial" ]]; then
            thock=$(echo "$partial" | sed 's/Thok_/TH-/')
        fi
    fi
    echo "$thock"
}

migrate_files() {
    local total=0 migrated=0 skipped=0 errors=0
    log "Starting file migration from $BASE_DIR"
    log "Mode: $([ "$DRY_RUN" = "true" ] && echo "DRY RUN" || echo "ACTUAL EXECUTION")"

    while IFS= read -r -d '' file; do
        ((total++))
        local filename=$(basename "$file")
        local thock=$(extract_thock_from_filename "$filename")

        if [[ -z "$thock" ]]; then
            warn "Could not extract thock from: $filename - skipping"
            ((skipped++))
            continue
        fi

        local year=$(extract_year "$thock")
        local dest_dir="$BASE_DIR/$year/$thock"
        local dest_file="$dest_dir/$filename"

        if [[ "$file" == *"/$year/$thock/"* ]]; then
            ((skipped++))
            continue
        fi

        if [[ -f "$dest_file" ]]; then
            warn "Destination exists: $dest_file - skipping"
            ((skipped++))
            continue
        fi

        if [[ "$DRY_RUN" == "true" ]]; then
            log "[DRY RUN] Would move: $filename → $year/$thock/"
            ((migrated++))
        else
            mkdir -p "$dest_dir"
            if mv -n "$file" "$dest_file"; then
                log "Migrated: $filename → $year/$thock/"
                ((migrated++))
            else
                error "Failed to move: $file"
                ((errors++))
            fi
        fi

        if ((total % 100 == 0)); then
            log "Progress: $total processed, $migrated migrated, $skipped skipped, $errors errors"
        fi
    done < <(find "$BASE_DIR" -maxdepth 1 -type f \( -name "*.jpg" -o -name "*.jpeg" -o -name "*.png" -o -name "*.mp4" -o -name "*.MOV" -o -name "*.mov" \) -print0)

    success "Migration complete!"
    log "Total: $total | Migrated: $migrated | Skipped: $skipped | Errors: $errors"
    log "Log: $LOG_FILE"
}

verify_migration() {
    log "Verifying migration..."
    local flat_files=$(find "$BASE_DIR" -maxdepth 1 -type f \( -name "*.jpg" -o -name "*.mp4" \) 2>/dev/null | wc -l)
    if [[ "$flat_files" -gt 0 ]]; then
        warn "$flat_files files still in flat structure"
        return 1
    fi
    success "Verification passed: no files in flat structure"
    return 0
}

main() {
    log "========================================"
    log "Media File Migration Script"
    log "========================================"
    if [[ ! -d "$BASE_DIR" ]]; then
        error "Base directory not found: $BASE_DIR"
        exit 1
    fi
    migrate_files
    if [[ "$DRY_RUN" == "false" ]]; then
        verify_migration
    fi
}

main "$@"
SCRIPT_EOF

chmod +x scripts/migrate-media-files.sh
success "File migration script created and made executable"

# Step 4: Create database backup
log "Step 4: Creating database backup..."
docker compose -f docker-compose.production.yml exec postgres \
  pg_dump -U cold_user cold_db > backups/pre-migration-$(date +%Y%m%d_%H%M%S).sql
success "Database backup created: $(ls -lh backups/pre-migration-*.sql | tail -1)"

# Step 5: Create file system backup
log "Step 5: Creating file system backup..."
if [[ -d "shared/Room Config" ]]; then
    tar -czf backups/room-config-backup-$(date +%Y%m%d).tar.gz "shared/Room Config"
    success "File backup created: $(ls -lh backups/room-config-backup-*.tar.gz | tail -1)"
else
    warn "Room Config directory not found, skipping file backup"
fi

# Step 6: Run database migration
log "Step 6: Running database migration..."
docker compose -f docker-compose.production.yml exec postgres \
  psql -U cold_user -d cold_db -f migrations/030_migrate_media_paths.sql
success "Database migration completed"

# Step 7: Verify database update
log "Step 7: Verifying database update..."
docker compose -f docker-compose.production.yml exec postgres \
  psql -U cold_user -d cold_db -c \
  "SELECT
    (SELECT COUNT(*) FROM room_entry_media WHERE file_path LIKE 'Room Config/%/%/%') as room_migrated,
    (SELECT COUNT(*) FROM gate_pass_media WHERE file_path LIKE 'Room Config/%/%/%') as gate_migrated,
    (SELECT COUNT(*) FROM room_entry_media WHERE file_path ~ '^Room Config/[^/]+\.') as room_flat,
    (SELECT COUNT(*) FROM gate_pass_media WHERE file_path ~ '^Room Config/[^/]+\.') as gate_flat;"

# Step 8: Dry run file migration
log "Step 8: Running file migration DRY RUN..."
DRY_RUN=true bash scripts/migrate-media-files.sh
echo ""
warn "DRY RUN completed. Review the log file shown above."
echo ""

# Ask for confirmation
read -p "Do you want to proceed with ACTUAL file migration? (yes/no): " confirm
if [[ "$confirm" != "yes" ]]; then
    warn "Migration cancelled by user"
    exit 0
fi

# Step 9: Actual file migration
log "Step 9: Running ACTUAL file migration..."
DRY_RUN=false bash scripts/migrate-media-files.sh
success "File migration completed"

# Step 10: Verify file structure
log "Step 10: Verifying file structure..."
if [[ -d "shared/Room Config" ]]; then
    echo "Year folders created:"
    ls -la "shared/Room Config" | grep "^d" | grep -E "[0-9]{4}"

    # Check for any remaining flat files
    flat_count=$(find "shared/Room Config" -maxdepth 1 -type f | wc -l)
    if [[ $flat_count -eq 0 ]]; then
        success "No flat files remaining"
    else
        warn "$flat_count files still in flat structure"
    fi
fi

# Step 11: Restart backend
log "Step 11: Restarting backend..."
docker compose -f docker-compose.production.yml restart backend
success "Backend restarted"

# Step 12: Check logs
log "Step 12: Checking backend logs..."
sleep 3
docker compose -f docker-compose.production.yml logs backend | tail -30

echo ""
echo "=========================================="
success "MIGRATION COMPLETED SUCCESSFULLY!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Test file downloads through the web interface"
echo "2. Monitor logs for any errors: docker compose -f docker-compose.production.yml logs -f backend"
echo "3. After 24 hours of successful operation, clean up backup tables:"
echo "   docker compose -f docker-compose.production.yml exec postgres \\"
echo "     psql -U cold_user -d cold_db -c \\"
echo "     'DROP TABLE IF EXISTS room_entry_media_backup_030, gate_pass_media_backup_030, media_sync_queue_backup_030;'"
echo ""
