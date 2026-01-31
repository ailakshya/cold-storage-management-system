#!/bin/bash
# R2 Object Migration Script
# Reorganizes Cloudflare R2 objects to match new year/thock structure

set -e

echo "=========================================="
echo "R2 Object Migration Script"
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

# Check for required environment variables
if [[ -z "$R2_ACCOUNT_ID" ]] || [[ -z "$R2_ACCESS_KEY_ID" ]] || [[ -z "$R2_SECRET_ACCESS_KEY" ]] || [[ -z "$R2_BUCKET_NAME" ]]; then
    error "Missing R2 credentials. Set R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME"
fi

# Configure AWS CLI for R2
export AWS_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
R2_ENDPOINT="https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com"

log "R2 Configuration:"
log "  - Account: $R2_ACCOUNT_ID"
log "  - Bucket: $R2_BUCKET_NAME"
log "  - Endpoint: $R2_ENDPOINT"
echo ""

# Step 1: Get list of objects that need migration from database
log "Step 1: Getting list of objects from database..."
sudo -u postgres psql -d cold_db -t -A -F'|' << 'SQL' > /tmp/r2_migration_list.txt
SELECT
    msq.id,
    msqb.r2_key as old_key,
    msq.r2_key as new_key
FROM media_sync_queue msq
JOIN media_sync_queue_r2_backup msqb ON msq.id = msqb.id
WHERE msq.r2_synced = true
  AND msqb.r2_key != msq.r2_key
ORDER BY msq.created_at;
SQL

migration_count=$(wc -l < /tmp/r2_migration_list.txt | tr -d ' ')
log "Found $migration_count objects to migrate"
echo ""

if [[ $migration_count -eq 0 ]]; then
    success "No objects need migration!"
    exit 0
fi

# Step 2: Test R2 connection
log "Step 2: Testing R2 connection..."
if aws s3 ls "s3://${R2_BUCKET_NAME}/" --endpoint-url "$R2_ENDPOINT" --max-items 1 > /dev/null 2>&1; then
    success "R2 connection successful"
else
    error "Cannot connect to R2. Check credentials and network."
fi
echo ""

# Step 3: Migrate objects
log "Step 3: Migrating R2 objects..."
log "This will copy objects to new locations and delete old ones"
echo ""

migrated=0
skipped=0
failed=0

while IFS='|' read -r id old_key new_key; do
    echo "[$((migrated+skipped+failed+1))/$migration_count] $old_key -> $new_key"

    # Check if new key already exists
    if aws s3 ls "s3://${R2_BUCKET_NAME}/${new_key}" --endpoint-url "$R2_ENDPOINT" > /dev/null 2>&1; then
        warn "  Destination already exists, skipping"
        ((skipped++))
        continue
    fi

    # Check if old key exists
    if ! aws s3 ls "s3://${R2_BUCKET_NAME}/${old_key}" --endpoint-url "$R2_ENDPOINT" > /dev/null 2>&1; then
        warn "  Source not found, skipping"
        ((skipped++))
        continue
    fi

    # Copy object to new location
    if aws s3 cp "s3://${R2_BUCKET_NAME}/${old_key}" "s3://${R2_BUCKET_NAME}/${new_key}" \
            --endpoint-url "$R2_ENDPOINT" \
            --only-show-errors 2>&1; then

        # Delete old object
        if aws s3 rm "s3://${R2_BUCKET_NAME}/${old_key}" \
                --endpoint-url "$R2_ENDPOINT" \
                --only-show-errors 2>&1; then
            success "  Migrated and cleaned up"
            ((migrated++))
        else
            warn "  Copied but failed to delete old object"
            ((migrated++))
        fi
    else
        error "  Failed to copy"
        ((failed++))
    fi

    # Progress update every 10 files
    if (( (migrated+skipped+failed) % 10 == 0 )); then
        echo ""
        log "Progress: $migrated migrated, $skipped skipped, $failed failed"
        echo ""
    fi
done < /tmp/r2_migration_list.txt

echo ""
echo "=========================================="
success "R2 MIGRATION COMPLETED!"
echo "=========================================="
echo ""
echo "Results:"
echo "  - Migrated: $migrated"
echo "  - Skipped: $skipped"
echo "  - Failed: $failed"
echo ""

if [[ $failed -gt 0 ]]; then
    warn "Some objects failed to migrate. Check logs above."
    exit 1
fi

# Step 4: Verify migration
log "Step 4: Verifying sample objects..."
sample_count=0
verified=0

while IFS='|' read -r id old_key new_key; do
    if aws s3 ls "s3://${R2_BUCKET_NAME}/${new_key}" --endpoint-url "$R2_ENDPOINT" > /dev/null 2>&1; then
        ((verified++))
    fi
    ((sample_count++))
    [[ $sample_count -ge 5 ]] && break
done < /tmp/r2_migration_list.txt

log "Verified $verified/$sample_count sample objects"
echo ""

success "Migration complete! New R2 structure:"
echo "  {year}/{thock}/{media_type}_{filename}"
echo ""
echo "Example:"
echo "  2026/1337-32/pickup_Thok_1337-32_Lakshya_pickup_GP2167...mp4"
echo ""

# Cleanup
rm -f /tmp/r2_migration_list.txt

echo "Note: Database backup table 'media_sync_queue_r2_backup' preserved for rollback"
echo "      Drop after 24 hours of successful operation"
