#!/bin/bash
set -euo pipefail

echo "=========================================="
echo "Media File Migration - Bare Metal PostgreSQL"
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

# PostgreSQL credentials
PGUSER="${PGUSER:-cold_user}"
PGDATABASE="${PGDATABASE:-cold_db}"
PGHOST="${PGHOST:-localhost}"

# Determine base directory for Room Config files
if [[ -d "shared/Room Config" ]]; then
    ROOM_CONFIG_BASE="shared/Room Config"
elif [[ -d "/mass-pool/shared/Room Config" ]]; then
    ROOM_CONFIG_BASE="/mass-pool/shared/Room Config"
elif [[ -d "$HOME/cold-storage/shared/Room Config" ]]; then
    ROOM_CONFIG_BASE="$HOME/cold-storage/shared/Room Config"
else
    warn "Room Config directory not found, will create it if needed"
    ROOM_CONFIG_BASE="shared/Room Config"
fi

log "Using Room Config base: $ROOM_CONFIG_BASE"

# Step 1: Create directories
log "Step 1: Creating directories..."
mkdir -p migrations scripts backups
mkdir -p "$ROOM_CONFIG_BASE"
success "Directories created"

# Step 2: Create SQL migration file
log "Step 2: Creating SQL migration file..."
cat > migrations/030_migrate_media_paths.sql << 'EOF'
-- Migration 030: Reorganize media file paths to year/thock structure
BEGIN;

CREATE TABLE IF NOT EXISTS room_entry_media_backup_030 AS
SELECT * FROM room_entry_media WHERE file_path NOT LIKE 'Room Config/%/%/%';

CREATE TABLE IF NOT EXISTS gate_pass_media_backup_030 AS
SELECT * FROM gate_pass_media WHERE file_path NOT LIKE 'Room Config/%/%/%';

CREATE TABLE IF NOT EXISTS media_sync_queue_backup_030 AS
SELECT * FROM media_sync_queue WHERE local_file_path NOT LIKE '%/Room Config/%/%/%';

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

WITH media_info AS (
    SELECT 'room_entry' as source, id, thock_number, file_name FROM room_entry_media
    UNION ALL
    SELECT 'gate_pass' as source, id, thock_number, file_name FROM gate_pass_media
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

# Step 3: Create file migration script
log "Step 3: Creating file migration script..."
cat > scripts/migrate-media-files.sh << 'SCRIPT_EOF'
#!/bin/bash
set -euo pipefail

# Detect Room Config directory
if [[ -d "$HOME/cold-storage/shared/Room Config" ]]; then
    BASE_DIR="$HOME/cold-storage/shared/Room Config"
elif [[ -d "/mass-pool/shared/Room Config" ]]; then
    BASE_DIR="/mass-pool/shared/Room Config"
elif [[ -d "shared/Room Config" ]]; then
    BASE_DIR="shared/Room Config"
else
    echo "Error: Room Config directory not found"
    exit 1
fi

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

main() {
    log "========================================"
    log "Media File Migration Script"
    log "========================================"
    log "Using directory: $BASE_DIR"
    if [[ ! -d "$BASE_DIR" ]]; then
        error "Base directory not found: $BASE_DIR"
        exit 1
    fi
    migrate_files
}

main "$@"
SCRIPT_EOF

chmod +x scripts/migrate-media-files.sh
success "File migration script created and made executable"

# Step 4: Create database backup
log "Step 4: Creating database backup..."
pg_dump -U "$PGUSER" -h "$PGHOST" "$PGDATABASE" > backups/pre-migration-$(date +%Y%m%d_%H%M%S).sql
success "Database backup created: $(ls -lh backups/pre-migration-*.sql | tail -1 | awk '{print $9, $5}')"

# Step 5: Create file system backup (only if Room Config exists and has files)
log "Step 5: Creating file system backup..."
if [[ -d "$ROOM_CONFIG_BASE" ]] && [[ $(find "$ROOM_CONFIG_BASE" -maxdepth 1 -type f | wc -l) -gt 0 ]]; then
    tar -czf backups/room-config-backup-$(date +%Y%m%d).tar.gz "$ROOM_CONFIG_BASE"
    success "File backup created: $(ls -lh backups/room-config-backup-*.tar.gz | tail -1 | awk '{print $9, $5}')"
else
    warn "Room Config directory empty or not found, skipping file backup"
fi

# Step 6: Run database migration
log "Step 6: Running database migration..."
psql -U "$PGUSER" -h "$PGHOST" -d "$PGDATABASE" -f migrations/030_migrate_media_paths.sql
success "Database migration completed"

# Step 7: Verify database update
log "Step 7: Verifying database update..."
psql -U "$PGUSER" -h "$PGHOST" -d "$PGDATABASE" -c \
  "SELECT
    (SELECT COUNT(*) FROM room_entry_media WHERE file_path LIKE 'Room Config/%/%/%') as room_migrated,
    (SELECT COUNT(*) FROM gate_pass_media WHERE file_path LIKE 'Room Config/%/%/%') as gate_migrated,
    (SELECT COUNT(*) FROM room_entry_media WHERE file_path ~ '^Room Config/[^/]+\.') as room_flat,
    (SELECT COUNT(*) FROM gate_pass_media WHERE file_path ~ '^Room Config/[^/]+\.') as gate_flat;"

# Step 8: Check if there are files to migrate
file_count=$(find "$ROOM_CONFIG_BASE" -maxdepth 1 -type f 2>/dev/null | wc -l)
if [[ $file_count -eq 0 ]]; then
    warn "No files found in $ROOM_CONFIG_BASE to migrate"
    echo ""
    success "Database migration completed successfully!"
    echo "No file migration needed - directory is empty or already organized."
    exit 0
fi

# Step 9: Dry run file migration
log "Step 8: Running file migration DRY RUN (found $file_count files)..."
DRY_RUN=true bash scripts/migrate-media-files.sh | tail -20
echo ""
warn "DRY RUN completed. Check the log file above for details."
echo ""

# Ask for confirmation
read -p "Do you want to proceed with ACTUAL file migration? (yes/no): " confirm
if [[ "$confirm" != "yes" ]]; then
    warn "Migration cancelled by user"
    exit 0
fi

# Step 10: Actual file migration
log "Step 9: Running ACTUAL file migration..."
DRY_RUN=false bash scripts/migrate-media-files.sh
success "File migration completed"

# Step 11: Verify file structure
log "Step 10: Verifying file structure..."
if [[ -d "$ROOM_CONFIG_BASE" ]]; then
    echo "Year folders created:"
    ls -la "$ROOM_CONFIG_BASE" | grep "^d" | grep -E "[0-9]{4}" || echo "  (checking for year folders...)"

    flat_count=$(find "$ROOM_CONFIG_BASE" -maxdepth 1 -type f 2>/dev/null | wc -l)
    if [[ $flat_count -eq 0 ]]; then
        success "No flat files remaining"
    else
        warn "$flat_count files still in flat structure"
    fi
fi

echo ""
echo "=========================================="
success "MIGRATION COMPLETED SUCCESSFULLY!"
echo "=========================================="
echo ""
echo "Cleanup (run after 24 hours of successful operation):"
echo "  psql -U $PGUSER -d $PGDATABASE -c \\"
echo "    'DROP TABLE IF EXISTS room_entry_media_backup_030, gate_pass_media_backup_030, media_sync_queue_backup_030;'"
echo ""
