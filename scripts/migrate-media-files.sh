#!/bin/bash
set -euo pipefail

# Configuration
BASE_DIR="$HOME/cold-storage/shared/Room Config"
LOG_FILE="/tmp/media-migration-$(date +%Y%m%d_%H%M%S).log"
DRY_RUN="${DRY_RUN:-true}"  # Set to 'false' for actual execution

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"; }
error() { echo -e "${RED}[ERROR]${NC} $*" | tee -a "$LOG_FILE"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $*" | tee -a "$LOG_FILE"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*" | tee -a "$LOG_FILE"; }

# Extract year from thock number (e.g., "TH-2026-0042" → "2026")
extract_year() {
    local thock="$1"
    local year=$(echo "$thock" | grep -oP 'TH-\K\d{4}' || echo "")

    if [[ -z "$year" ]]; then
        year=$(date +%Y)  # Fallback to current year
    fi

    echo "$year"
}

# Extract thock from filename (e.g., "Thok_1328-2_..." → "TH-1328-2")
extract_thock_from_filename() {
    local filename="$1"

    # Try format: TH-2026-0042
    local thock=$(echo "$filename" | grep -oP 'TH-\d{4}-\d+' | head -1)

    if [[ -z "$thock" ]]; then
        # Try format: Thok_1328-2 (extract from filename, query DB for full format)
        local partial=$(echo "$filename" | grep -oP 'Thok_\d+-\d+' | head -1)

        if [[ -n "$partial" ]]; then
            # Convert "Thok_1328-2" → "TH-1328-2"
            thock=$(echo "$partial" | sed 's/Thok_/TH-/')
        fi
    fi

    echo "$thock"
}

# Main migration
migrate_files() {
    local total=0
    local migrated=0
    local skipped=0
    local errors=0

    log "Starting file migration from $BASE_DIR"
    log "Mode: $([ "$DRY_RUN" = "true" ] && echo "DRY RUN" || echo "ACTUAL EXECUTION")"

    # Find all media files in flat structure (maxdepth 1 = not in subdirs)
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

        # Skip if already in year/thock structure
        if [[ "$file" == *"/$year/$thock/"* ]]; then
            ((skipped++))
            continue
        fi

        # Skip if destination already exists
        if [[ -f "$dest_file" ]]; then
            warn "Destination exists: $dest_file - skipping"
            ((skipped++))
            continue
        fi

        # Perform migration
        if [[ "$DRY_RUN" == "true" ]]; then
            log "[DRY RUN] Would move: $filename → $year/$thock/"
            ((migrated++))
        else
            # Create directory
            mkdir -p "$dest_dir"

            # Move file (preserve timestamps)
            if mv -n "$file" "$dest_file"; then
                log "Migrated: $filename → $year/$thock/"
                ((migrated++))
            else
                error "Failed to move: $file"
                ((errors++))
            fi
        fi

        # Progress every 100 files
        if ((total % 100 == 0)); then
            log "Progress: $total processed, $migrated migrated, $skipped skipped, $errors errors"
        fi

    done < <(find "$BASE_DIR" -maxdepth 1 -type f \( -name "*.jpg" -o -name "*.jpeg" -o -name "*.png" -o -name "*.mp4" -o -name "*.MOV" -o -name "*.mov" \) -print0)

    success "Migration complete!"
    log "Total: $total | Migrated: $migrated | Skipped: $skipped | Errors: $errors"
    log "Log: $LOG_FILE"
}

# Verification
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

# Main
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
