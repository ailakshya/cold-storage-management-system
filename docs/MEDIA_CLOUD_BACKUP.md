# Media Cloud Backup System (3-2-1 Strategy)

**Version:** 1.0.0
**Last Updated:** January 31, 2026

---

## Overview

The media cloud backup system provides redundancy for all media files (room entry photos/videos, gate pass photos/videos) using a **3-2-1 backup strategy**:

| Copy | Location | Storage | Connection |
|------|----------|---------|------------|
| **1 (Primary)** | Production server | RAIDZ1 (3x4TB HDD, ~7.5TB) | Local disk |
| **2 (NAS)** | TrueNAS at remote site | RustFS on ZFS (10TB) | S3 API over internet |
| **3 (Cloud)** | Cloudflare R2 | Object storage | S3 API over internet |

**Key design**: RustFS (TrueNAS) and Cloudflare R2 both expose S3-compatible APIs, so the same Go code (`aws-sdk-go-v2`) handles both backends — just different endpoints and credentials.

---

## Architecture

### Upload Flow (async, non-blocking)

```
Phone/Browser → POST /api/files/upload → Local Disk (/mass-pool/shared)
                                            │
                                            ▼
                                  Save metadata to DB
                                            │
                                            ▼ (async)
                                  Enqueue to media_sync_queue
                                            │
                             ┌──────────────┼──────────────┐
                             ▼                              ▼
                   Upload to RustFS (NAS)       Upload to R2 (Cloud)
                   via S3 API                   via S3 API
```

Uploads to local disk remain instant. Background workers sync to NAS and R2 independently after save. If either target is down, that target retries with exponential backoff while the other continues normally.

### Download Fallback Chain

```
Request for media file:
  1. Try LOCAL disk → found? Serve it (fastest)
  2. Local missing → Try NAS (RustFS) → found? Stream it
  3. NAS unreachable → Try R2 (Cloud) → found? Stream it
  4. All failed → 404 Not Found
```

### Network Resilience

Each target has independent sync flags (`r2_synced`, `nas_synced`, `local_synced`):

- **Normal**: Worker syncs to NAS + R2
- **Internet down**: Local disk works, queue accumulates, catches up when internet returns
- **NAS down**: R2 syncs normally, NAS catches up when it recovers
- **Local disk full**: Upload fails (future: fallback to NAS/R2 as primary)

---

## Files

### New Files Created

| File | Purpose |
|------|---------|
| `internal/models/media_sync.go` | `MediaSyncRecord`, `MediaSyncStats`, `RestoreProgress` structs |
| `internal/repositories/media_sync_repository.go` | DB queries: Enqueue, PickNext (`FOR UPDATE SKIP LOCKED`), MarkSynced/Failed, GetStats |
| `internal/services/media_sync_service.go` | Background workers, EnqueueMedia, RunInitialSync, BulkRestore |
| `internal/services/storage_backend.go` | `StorageBackend` interface, `LocalBackend`, `S3Backend`, factory functions |
| `internal/handlers/media_sync_handler.go` | Admin API: sync status, initial sync trigger, retry, restore |
| `migrations/029_add_media_sync.sql` | `media_sync_queue` table, indexes, ALTER TABLE for `cloud_synced`/`r2_key` |

### Modified Files

| File | Change |
|------|--------|
| `internal/config/r2_config.go` | Added `R2MediaBucketName`, `NASConfig` struct, `LoadNASConfig()` |
| `internal/handlers/file_manager_handler.go` | S3 dispatch for list/download/upload/delete/move, cloud fallback, cross-storage transfers |
| `internal/http/router.go` | Added `/api/admin/media-sync/*` routes |
| `cmd/server/main.go` | Init S3 backends, wire MediaSyncService + workers, inject into handlers |
| `docker-compose.production.yml` | Added `NAS_S3_*` env vars |
| `templates/admin_file_manager.html` | Added R2/NAS tabs, cloud-aware display, dynamic dropdown |

---

## Database Schema

### `media_sync_queue` Table

```sql
CREATE TABLE media_sync_queue (
    id SERIAL PRIMARY KEY,
    media_source VARCHAR(20) NOT NULL,   -- 'room_entry' or 'gate_pass'
    media_id INTEGER NOT NULL,           -- FK to source media table
    local_file_path TEXT NOT NULL,        -- Absolute path on disk
    r2_key TEXT NOT NULL,                 -- S3 key in both R2 and NAS buckets
    file_size BIGINT,
    sync_status VARCHAR(20) DEFAULT 'pending',  -- pending/uploading/synced/failed/skipped
    local_synced BOOLEAN DEFAULT TRUE,
    nas_synced BOOLEAN DEFAULT FALSE,
    r2_synced BOOLEAN DEFAULT FALSE,
    primary_location VARCHAR(10) DEFAULT 'local',  -- where file was originally saved
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    next_retry_at TIMESTAMP
);
```

### Added Columns on Existing Tables

```sql
ALTER TABLE room_entry_media ADD COLUMN cloud_synced BOOLEAN DEFAULT FALSE;
ALTER TABLE room_entry_media ADD COLUMN r2_key TEXT;
ALTER TABLE gate_pass_media  ADD COLUMN cloud_synced BOOLEAN DEFAULT FALSE;
ALTER TABLE gate_pass_media  ADD COLUMN r2_key TEXT;
```

---

## S3 Key Structure

Both R2 and NAS use the same bucket name (`cold-media`) and key structure:

```
cold-media/
  room-entry/{thock_number}/{media_type}/{file_name}
  gate-pass/{thock_number}/{media_type}/{file_name}
```

Example: `room-entry/TH-2026-0042/entry/IMG_20260131_143022.jpg`

---

## Background Sync Workers

### Worker Loop

2 workers poll every 5 seconds:

1. `SELECT ... FROM media_sync_queue WHERE status IN ('pending','failed') AND next_retry_at <= NOW() ORDER BY created_at LIMIT 1 FOR UPDATE SKIP LOCKED`
2. Check local file exists (if not, retry with 30s delay for video conversion)
3. Upload to NAS (RustFS) → mark `nas_synced = true`
4. Upload to R2 (Cloudflare) → mark `r2_synced = true`
5. If both synced → `sync_status = 'synced'`, update source table `cloud_synced = true`
6. On failure → increment `retry_count`, exponential backoff (30s, 1m, 5m, 15m, 1h), max 5 retries

### Video Conversion Handling

When a video file (.MOV, .AVI, etc.) is enqueued, ffmpeg may still be converting it to .mp4. The worker:
1. Checks for the original file path
2. If not found, checks for the `.mp4` variant (converted filename)
3. If neither exists and it's a video extension, retries with 30s delay up to 3 times
4. After 3 retries, marks as skipped

---

## Admin API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/admin/media-sync/status` | Pending/synced/failed counts, total size, per-target counts |
| `POST` | `/api/admin/media-sync/initial-sync` | Enqueue all existing unsynced media (room_entry + gate_pass) |
| `POST` | `/api/admin/media-sync/retry-failed` | Reset all failed items back to pending for retry |
| `POST` | `/api/admin/media-sync/restore` | Bulk restore: download all cloud-synced files missing locally |

### Example: Check Sync Status

```bash
curl -s https://your-domain/api/admin/media-sync/status \
  -H "Cookie: session=..." | jq .
```

Response:
```json
{
  "pending": 12,
  "uploading": 2,
  "synced": 1543,
  "failed": 3,
  "skipped": 1,
  "total_files": 1561,
  "total_size_bytes": 42949672960,
  "synced_size_bytes": 41489612800,
  "nas_synced_count": 1540,
  "r2_synced_count": 1543
}
```

---

## Unified File Manager

The Storage Manager UI (`/admin/files`) now includes R2 and NAS as browsable storage locations alongside local pools:

| Root | Label | Backend |
|------|-------|---------|
| `bulk` | Bulk Storage | Local: `/mass-pool/shared` |
| `highspeed` | High Speed | Local: `/fast-pool/data` |
| `archives` | Archives | Local: `/mass-pool/archives` |
| `backups` | Backups | Local: `/mass-pool/backups` |
| `trash` | Trash Can | Local: `/mass-pool/trash` |
| `r2` | Cloudflare R2 | S3: `cold-media` bucket |
| `nas` | NAS (RustFS) | S3: `cold-media` bucket |

### Operations

All file operations work across storage backends:

- **List**: Browse files in R2/NAS like local directories
- **Download**: Stream files directly from S3 to browser
- **Upload**: Upload files directly to R2/NAS
- **Delete**: Permanent delete from S3 (no trash for cloud storage)
- **Move**: Cross-storage transfers (Local ↔ R2 ↔ NAS)

### Delete Behavior

Deleting from one storage location does NOT cascade to others:
- Delete from local → NAS and R2 copies remain
- Delete from NAS → Local and R2 copies remain
- Delete from R2 → Local and NAS copies remain

---

## Configuration

### Cloudflare R2

Configured via constants in `internal/config/r2_config.go`:

```go
const (
    R2Endpoint        = "https://<account-id>.r2.cloudflarestorage.com"
    R2AccessKey        = "<access-key>"
    R2SecretKey        = "<secret-key>"
    R2MediaBucketName = "cold-media"  // Separate from DB backup bucket
)
```

### NAS (RustFS/MinIO)

Configured via environment variables (set in `docker-compose.production.yml`):

```yaml
environment:
  NAS_S3_ENDPOINT: "http://<truenas-ip>:9000"
  NAS_S3_ACCESS_KEY: "<rustfs-access-key>"
  NAS_S3_SECRET_KEY: "<rustfs-secret-key>"
  NAS_S3_BUCKET: "cold-media"
```

If `NAS_S3_ENDPOINT` is not set, the NAS backend is disabled and only R2 is used.

---

## Deployment Steps

### 1. Create R2 Bucket

In Cloudflare dashboard → R2 → Create bucket → Name: `cold-media`

Or via rclone on production server:
```bash
rclone mkdir r2:cold-media
```

### 2. Run Database Migration

```bash
psql -U cold_user -d cold_db -f migrations/029_add_media_sync.sql
```

### 3. Set Up RustFS on TrueNAS (when network available)

1. Install RustFS from TrueNAS Apps catalog
2. Create `cold-media` bucket via RustFS console
3. Create access key and secret key
4. Set up port forwarding from TrueNAS router to expose RustFS API port

### 4. Configure Environment Variables

Add NAS credentials to production `.env` or `docker-compose.production.yml`:

```bash
NAS_S3_ENDPOINT=http://<truenas-public-ip>:<port>
NAS_S3_ACCESS_KEY=<rustfs-access-key>
NAS_S3_SECRET_KEY=<rustfs-secret-key>
NAS_S3_BUCKET=cold-media
```

### 5. Deploy

```bash
docker compose -f docker-compose.production.yml up -d --build
```

### 6. Run Initial Sync

After deployment, trigger initial sync to enqueue all existing media:

```bash
curl -X POST https://your-domain/api/admin/media-sync/initial-sync \
  -H "Cookie: session=..."
```

---

## Disaster Recovery

### Scenario: Local Disk Failure

1. Media files are automatically served from NAS/R2 via download fallback
2. Run bulk restore to re-download all files to new local disk:
   ```bash
   curl -X POST https://your-domain/api/admin/media-sync/restore \
     -H "Cookie: session=..."
   ```
3. Restore tries NAS first (faster, same network) then R2

### Scenario: NAS Failure

1. R2 copies are unaffected
2. Replace NAS hardware, reinstall RustFS
3. Files will sync from local → new NAS via background workers

### Scenario: R2 Outage

1. NAS copies are unaffected, local copies are unaffected
2. Sync to R2 will queue and retry automatically when R2 recovers

---

## Cost Estimate

### Cloudflare R2

- Storage: $0.015/GB/month, no egress fees
- ~500 entries/season x ~80MB media each = ~40GB/season
- Annual cost: ~$20-25/year

### RustFS on TrueNAS

- Free (self-hosted), using existing 10TB NAS
- Only cost is electricity + internet bandwidth

---

## StorageBackend Interface

All storage operations go through the `StorageBackend` interface:

```go
type StorageBackend interface {
    List(ctx context.Context, prefix string) ([]StorageObject, error)
    Download(ctx context.Context, key string) (io.ReadCloser, int64, error)
    Upload(ctx context.Context, key string, reader io.Reader, size int64) error
    Delete(ctx context.Context, key string) error
    Stat(ctx context.Context, key string) (*StorageObject, error)
    Exists(ctx context.Context, key string) (bool, error)
    Move(ctx context.Context, srcKey, dstKey string) error
    Name() string
}
```

Implementations:
- **`LocalBackend`**: Wraps `os.*` calls for local filesystem
- **`S3Backend`**: Wraps `aws-sdk-go-v2` S3 client (shared by R2 and RustFS)

Factory functions:
- `NewR2MediaBackend()` → R2 with hardcoded credentials
- `NewNASBackend(nasCfg)` → RustFS/MinIO with env var credentials

---

## Monitoring

Check sync health via the admin API:

```bash
# Quick status check
curl -s /api/admin/media-sync/status | jq '{pending, failed, synced, total_files}'

# If failed count is high, retry all failed
curl -X POST /api/admin/media-sync/retry-failed
```

Workers log activity to stdout with `[MediaSync]` prefix:
```
[MediaSync] Worker 0: processing room_entry #1234 → room-entry/TH-2026-0042/entry/IMG.jpg
[MediaSync] Worker 0: R2 synced room-entry/TH-2026-0042/entry/IMG.jpg
[MediaSync] Worker 0: fully synced room-entry/TH-2026-0042/entry/IMG.jpg
```
