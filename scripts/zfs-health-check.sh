#!/bin/bash
#
# Storage Health Check Script
# Checks both ZFS pools (fast-pool) and mdraid arrays (mass-pool)
#
# Usage:
#   --deploy-gate   Quick health check (used by CI/CD before deploy)
#   --report        Full health report (JSON, for weekly cron)
#   --scrub         Trigger scrub/check on all storage
#
# Exit codes (--deploy-gate):
#   0 = All storage healthy
#   1 = Degraded (warn but allow deploy)
#   2 = Faulted/unavailable (abort deploy)
#
# Storage layout:
#   fast-pool  — ZFS mirror (2x 1TB SSD) → /fast-pool
#   md127      — mdraid RAID5 (3x 4TB HDD) + LVM → /mass-pool

set -euo pipefail

ZFS_POOLS=("fast-pool")
MD_ARRAYS=("md127")
REPORT_DIR="/mass-pool/backups/storage-health"

# ─── Deploy Gate ──────────────────────────────────────────────────────
deploy_gate() {
  local exit_code=0

  # --- Check ZFS pools ---
  echo "--- ZFS Pools ---"
  for pool in "${ZFS_POOLS[@]}"; do
    if ! zpool list "$pool" > /dev/null 2>&1; then
      echo "  ERROR: ZFS pool '$pool' not found"
      exit_code=2
      continue
    fi

    STATE=$(zpool list -H -o health "$pool")
    case "$STATE" in
      ONLINE)
        echo "  $pool: ONLINE"
        ;;
      DEGRADED)
        echo "  WARNING: $pool is DEGRADED — deploy will proceed but investigate immediately"
        [ $exit_code -lt 1 ] && exit_code=1
        ;;
      FAULTED|UNAVAIL|REMOVED)
        echo "  CRITICAL: $pool is $STATE — aborting deploy"
        exit_code=2
        ;;
      *)
        echo "  UNKNOWN: $pool state is '$STATE'"
        [ $exit_code -lt 1 ] && exit_code=1
        ;;
    esac

    # Check for scrub/data errors
    ERROR_LINE=$(zpool status "$pool" 2>/dev/null | grep "errors:" || echo "")
    if [ -n "$ERROR_LINE" ] && ! echo "$ERROR_LINE" | grep -q "No known data errors"; then
      echo "  WARNING: $pool has data errors: $ERROR_LINE"
      [ $exit_code -lt 1 ] && exit_code=1
    fi

    # Check disk usage
    CAPACITY=$(zpool list -H -o capacity "$pool" | tr -d '%')
    if [ "$CAPACITY" -gt 90 ]; then
      echo "  CRITICAL: $pool at ${CAPACITY}% capacity"
      exit_code=2
    elif [ "$CAPACITY" -gt 80 ]; then
      echo "  WARNING: $pool at ${CAPACITY}% capacity"
      [ $exit_code -lt 1 ] && exit_code=1
    else
      echo "  $pool capacity: ${CAPACITY}%"
    fi
  done

  # --- Check mdraid arrays ---
  echo "--- mdraid Arrays ---"
  for array in "${MD_ARRAYS[@]}"; do
    if [ ! -e "/dev/$array" ]; then
      echo "  ERROR: mdraid array /dev/$array not found"
      exit_code=2
      continue
    fi

    # Parse /proc/mdstat for array state
    if grep -q "$array" /proc/mdstat 2>/dev/null; then
      MD_LINE=$(grep -A1 "^$array" /proc/mdstat)
      # Check if all disks are present: [UUU] means all up, [_UU] means one down
      if echo "$MD_LINE" | grep -q '\[U*\]'; then
        DISK_STATE=$(echo "$MD_LINE" | grep -oP '\[.*?\]' | tail -1)
        if echo "$DISK_STATE" | grep -q '_'; then
          echo "  WARNING: $array has degraded disk(s): $DISK_STATE"
          [ $exit_code -lt 1 ] && exit_code=1
        else
          echo "  $array: HEALTHY $DISK_STATE"
        fi
      fi

      # Check if rebuilding
      if echo "$MD_LINE" | grep -q "recovery"; then
        echo "  INFO: $array is rebuilding"
        [ $exit_code -lt 1 ] && exit_code=1
      fi
    else
      echo "  ERROR: $array not found in /proc/mdstat"
      exit_code=2
    fi
  done

  # --- Check mount point disk usage ---
  echo "--- Disk Usage ---"
  for MOUNT in /mass-pool /fast-pool; do
    if mountpoint -q "$MOUNT" 2>/dev/null || [ -d "$MOUNT" ]; then
      USAGE=$(df -h "$MOUNT" 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%')
      if [ -n "$USAGE" ]; then
        if [ "$USAGE" -gt 90 ]; then
          echo "  CRITICAL: $MOUNT at ${USAGE}%"
          exit_code=2
        elif [ "$USAGE" -gt 80 ]; then
          echo "  WARNING: $MOUNT at ${USAGE}%"
          [ $exit_code -lt 1 ] && exit_code=1
        else
          echo "  $MOUNT: ${USAGE}% used"
        fi
      fi
    fi
  done

  return $exit_code
}

# ─── Full Report ──────────────────────────────────────────────────────
full_report() {
  mkdir -p "$REPORT_DIR"
  local REPORT_FILE="$REPORT_DIR/storage-health-$(date +%Y%m%d_%H%M%S).json"
  local TIMESTAMP
  TIMESTAMP=$(date -Iseconds)

  echo "Generating storage health report..."

  {
    echo "{"
    echo "  \"timestamp\": \"$TIMESTAMP\","

    # ZFS pools
    echo "  \"zfs_pools\": ["
    local first_pool=true
    for pool in "${ZFS_POOLS[@]}"; do
      if ! zpool list "$pool" > /dev/null 2>&1; then
        continue
      fi
      [ "$first_pool" = true ] || echo "    ,"
      first_pool=false

      STATE=$(zpool list -H -o health "$pool")
      SIZE=$(zpool list -H -o size "$pool")
      ALLOC=$(zpool list -H -o alloc "$pool")
      FREE=$(zpool list -H -o free "$pool")
      CAP=$(zpool list -H -o capacity "$pool" | tr -d '%')
      FRAG=$(zpool list -H -o frag "$pool" | tr -d '%')
      SCRUB_LINE=$(zpool status "$pool" | grep "scan:" | head -1 || echo "unknown")
      ERROR_LINE=$(zpool status "$pool" | grep "errors:" || echo "No known data errors")

      echo "    {"
      echo "      \"name\": \"$pool\","
      echo "      \"type\": \"zfs\","
      echo "      \"state\": \"$STATE\","
      echo "      \"size\": \"$SIZE\","
      echo "      \"allocated\": \"$ALLOC\","
      echo "      \"free\": \"$FREE\","
      echo "      \"capacity_pct\": $CAP,"
      echo "      \"fragmentation_pct\": $FRAG,"
      echo "      \"last_scrub\": \"$(echo "$SCRUB_LINE" | xargs)\","
      echo "      \"errors\": \"$(echo "$ERROR_LINE" | xargs)\""
      echo "    }"
    done
    echo "  ],"

    # mdraid arrays
    echo "  \"mdraid_arrays\": ["
    local first_md=true
    for array in "${MD_ARRAYS[@]}"; do
      [ "$first_md" = true ] || echo "    ,"
      first_md=false

      MD_DETAIL=$(sudo mdadm --detail "/dev/$array" 2>/dev/null || echo "")
      MD_STATE=$(echo "$MD_DETAIL" | grep "State :" | awk -F: '{print $2}' | xargs || echo "unknown")
      MD_LEVEL=$(echo "$MD_DETAIL" | grep "Raid Level" | awk -F: '{print $2}' | xargs || echo "unknown")
      MD_SIZE=$(echo "$MD_DETAIL" | grep "Array Size" | awk -F: '{print $2}' | xargs || echo "unknown")
      MD_ACTIVE=$(echo "$MD_DETAIL" | grep "Active Devices" | awk -F: '{print $2}' | xargs || echo "unknown")
      MD_TOTAL=$(echo "$MD_DETAIL" | grep "Raid Devices" | awk -F: '{print $2}' | xargs || echo "unknown")

      # Disk usage for mount point
      USAGE=$(df -h /mass-pool 2>/dev/null | tail -1 | awk '{print $5}' | tr -d '%' || echo "0")

      echo "    {"
      echo "      \"name\": \"$array\","
      echo "      \"type\": \"mdraid\","
      echo "      \"state\": \"$MD_STATE\","
      echo "      \"raid_level\": \"$MD_LEVEL\","
      echo "      \"array_size\": \"$MD_SIZE\","
      echo "      \"active_devices\": \"$MD_ACTIVE\","
      echo "      \"total_devices\": \"$MD_TOTAL\","
      echo "      \"mount_usage_pct\": $USAGE"
      echo "    }"
    done
    echo "  ],"

    # SMART status for all disks
    echo "  \"smart\": ["
    local first_disk=true
    for disk in /dev/sd[a-z]; do
      [ -b "$disk" ] || continue
      [ "$first_disk" = true ] || echo "    ,"
      first_disk=false

      SMART_HEALTH=$(sudo smartctl --health "$disk" 2>/dev/null | grep "SMART overall" | awk -F: '{print $2}' | xargs || echo "unknown")
      SMART_TEMP=$(sudo smartctl -A "$disk" 2>/dev/null | grep -i "temperature" | head -1 | awk '{print $NF}' || echo "unknown")
      POWER_HOURS=$(sudo smartctl -A "$disk" 2>/dev/null | grep "Power_On_Hours" | awk '{print $NF}' || echo "unknown")
      REALLOCATED=$(sudo smartctl -A "$disk" 2>/dev/null | grep "Reallocated_Sector" | awk '{print $NF}' || echo "unknown")

      echo "    {"
      echo "      \"device\": \"$disk\","
      echo "      \"health\": \"$SMART_HEALTH\","
      echo "      \"temperature\": \"$SMART_TEMP\","
      echo "      \"power_on_hours\": \"$POWER_HOURS\","
      echo "      \"reallocated_sectors\": \"$REALLOCATED\""
      echo "    }"
    done
    echo "  ]"

    echo "}"
  } > "$REPORT_FILE"

  echo "Report saved to: $REPORT_FILE"
  cat "$REPORT_FILE"
}

# ─── Scrub ────────────────────────────────────────────────────────────
trigger_scrub() {
  # ZFS scrub
  for pool in "${ZFS_POOLS[@]}"; do
    if zpool list "$pool" > /dev/null 2>&1; then
      echo "Starting ZFS scrub on $pool..."
      sudo zpool scrub "$pool"
      echo "  Scrub started on $pool"
    else
      echo "  ZFS pool $pool not found — skipping"
    fi
  done

  # mdraid check (equivalent of scrub)
  for array in "${MD_ARRAYS[@]}"; do
    if [ -e "/dev/$array" ]; then
      echo "Starting mdraid check on $array..."
      sudo bash -c "echo check > /sys/block/$array/md/sync_action" 2>/dev/null || \
        echo "  Could not start check on $array (may already be running)"
      echo "  Check started on $array"
    else
      echo "  mdraid array $array not found — skipping"
    fi
  done

  echo "All checks initiated."
  echo "Monitor ZFS: zpool status"
  echo "Monitor mdraid: cat /proc/mdstat"
}

# ─── Main ─────────────────────────────────────────────────────────────
case "${1:-}" in
  --deploy-gate)
    echo "Storage Deploy Gate Check"
    echo "========================="
    deploy_gate
    EXIT=$?
    if [ $EXIT -eq 0 ]; then
      echo "Result: ALL STORAGE HEALTHY"
    elif [ $EXIT -eq 1 ]; then
      echo "Result: DEGRADED — deploy allowed with warnings"
    else
      echo "Result: CRITICAL — deploy aborted"
    fi
    exit $EXIT
    ;;
  --report)
    full_report
    ;;
  --scrub)
    trigger_scrub
    ;;
  *)
    echo "Usage: $0 {--deploy-gate|--report|--scrub}"
    echo ""
    echo "  --deploy-gate   Quick check before deploy (exit 0=ok, 1=degraded, 2=critical)"
    echo "  --report        Full JSON health report (ZFS + mdraid + SMART)"
    echo "  --scrub         Trigger scrub/check on all storage"
    exit 1
    ;;
esac
