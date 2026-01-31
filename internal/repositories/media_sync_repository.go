package repositories

import (
	"context"
	"fmt"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MediaSyncRepository struct {
	pool *pgxpool.Pool
}

func NewMediaSyncRepository(pool *pgxpool.Pool) *MediaSyncRepository {
	return &MediaSyncRepository{pool: pool}
}

// Enqueue inserts a new sync record into the queue
func (r *MediaSyncRepository) Enqueue(ctx context.Context, record *models.MediaSyncRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO media_sync_queue
			(media_source, media_id, local_file_path, r2_key, file_size,
			 sync_status, local_synced, nas_synced, r2_synced, primary_location)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6, false, false, $7)`,
		record.MediaSource, record.MediaID, record.LocalFilePath, record.R2Key, record.FileSize,
		record.LocalSynced, record.PrimaryLocation,
	)
	return err
}

// PickNext atomically claims the next pending record for processing.
// Uses FOR UPDATE SKIP LOCKED for safe concurrent workers.
func (r *MediaSyncRepository) PickNext(ctx context.Context) (*models.MediaSyncRecord, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'uploading', started_at = NOW()
		WHERE id = (
			SELECT id FROM media_sync_queue
			WHERE sync_status IN ('pending', 'failed')
			  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
			  AND retry_count < max_retries
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, media_source, media_id, local_file_path, r2_key, file_size,
			sync_status, local_synced, nas_synced, r2_synced, primary_location,
			retry_count, max_retries, last_error, created_at, started_at, completed_at, next_retry_at`)

	rec := &models.MediaSyncRecord{}
	err := row.Scan(
		&rec.ID, &rec.MediaSource, &rec.MediaID, &rec.LocalFilePath, &rec.R2Key, &rec.FileSize,
		&rec.SyncStatus, &rec.LocalSynced, &rec.NASSynced, &rec.R2Synced, &rec.PrimaryLocation,
		&rec.RetryCount, &rec.MaxRetries, &rec.LastError, &rec.CreatedAt, &rec.StartedAt, &rec.CompletedAt, &rec.NextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// MarkNASSynced marks the NAS target as synced for a record
func (r *MediaSyncRepository) MarkNASSynced(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET nas_synced = true,
		    sync_status = CASE WHEN r2_synced AND local_synced THEN 'synced' ELSE sync_status END,
		    completed_at = CASE WHEN r2_synced AND local_synced THEN NOW() ELSE completed_at END
		WHERE id = $1`, id)
	return err
}

// MarkR2Synced marks the R2 target as synced for a record
func (r *MediaSyncRepository) MarkR2Synced(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET r2_synced = true,
		    sync_status = CASE WHEN nas_synced AND local_synced THEN 'synced' ELSE sync_status END,
		    completed_at = CASE WHEN nas_synced AND local_synced THEN NOW() ELSE completed_at END
		WHERE id = $1`, id)
	return err
}

// MarkLocalSynced marks the local target as synced (for files originally uploaded to NAS/R2)
func (r *MediaSyncRepository) MarkLocalSynced(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET local_synced = true,
		    sync_status = CASE WHEN nas_synced AND r2_synced THEN 'synced' ELSE sync_status END,
		    completed_at = CASE WHEN nas_synced AND r2_synced THEN NOW() ELSE completed_at END
		WHERE id = $1`, id)
	return err
}

// MarkAllSynced marks the record as fully synced across all targets
func (r *MediaSyncRepository) MarkAllSynced(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'synced', local_synced = true, nas_synced = true, r2_synced = true,
		    completed_at = NOW()
		WHERE id = $1`, id)
	return err
}

// MarkFailed increments retry count and sets next_retry_at with exponential backoff
func (r *MediaSyncRepository) MarkFailed(ctx context.Context, id int, errMsg string) error {
	// Exponential backoff: 30s, 1m, 5m, 15m, 1h
	backoffs := []time.Duration{
		30 * time.Second,
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
	}

	// Get current retry count to determine backoff
	var retryCount int
	err := r.pool.QueryRow(ctx, `SELECT retry_count FROM media_sync_queue WHERE id = $1`, id).Scan(&retryCount)
	if err != nil {
		return err
	}

	backoffIdx := retryCount
	if backoffIdx >= len(backoffs) {
		backoffIdx = len(backoffs) - 1
	}
	nextRetry := time.Now().Add(backoffs[backoffIdx])

	_, err = r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'failed', retry_count = retry_count + 1,
		    last_error = $2, next_retry_at = $3
		WHERE id = $1`, id, errMsg, nextRetry)
	return err
}

// MarkRetry sets a specific retry delay (e.g., for video conversion waiting)
func (r *MediaSyncRepository) MarkRetry(ctx context.Context, id int, errMsg string, delay time.Duration) error {
	nextRetry := time.Now().Add(delay)
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'failed', retry_count = retry_count + 1,
		    last_error = $2, next_retry_at = $3
		WHERE id = $1`, id, errMsg, nextRetry)
	return err
}

// MarkSkipped marks a record as skipped (e.g., file not found after all retries)
func (r *MediaSyncRepository) MarkSkipped(ctx context.Context, id int, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'skipped', last_error = $2, completed_at = NOW()
		WHERE id = $1`, id, reason)
	return err
}

// ResetPending resets a record back to pending (for manual retry)
func (r *MediaSyncRepository) ResetPending(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'pending', retry_count = 0, last_error = NULL, next_retry_at = NULL
		WHERE id = $1`, id)
	return err
}

// ResetAllFailed resets all failed records back to pending
func (r *MediaSyncRepository) ResetAllFailed(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue
		SET sync_status = 'pending', retry_count = 0, last_error = NULL, next_retry_at = NULL
		WHERE sync_status = 'failed'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetStats returns aggregate sync status counts
func (r *MediaSyncRepository) GetStats(ctx context.Context) (*models.MediaSyncStats, error) {
	stats := &models.MediaSyncStats{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE sync_status = 'pending') as pending,
			COUNT(*) FILTER (WHERE sync_status = 'uploading') as uploading,
			COUNT(*) FILTER (WHERE sync_status = 'synced') as synced,
			COUNT(*) FILTER (WHERE sync_status = 'failed') as failed,
			COUNT(*) FILTER (WHERE sync_status = 'skipped') as skipped,
			COUNT(*) as total_files,
			COALESCE(SUM(file_size), 0) as total_size,
			COALESCE(SUM(file_size) FILTER (WHERE sync_status = 'synced'), 0) as synced_size,
			COUNT(*) FILTER (WHERE nas_synced = true) as nas_synced_count,
			COUNT(*) FILTER (WHERE r2_synced = true) as r2_synced_count
		FROM media_sync_queue
	`).Scan(
		&stats.Pending, &stats.Uploading, &stats.Synced, &stats.Failed, &stats.Skipped,
		&stats.TotalFiles, &stats.TotalSize, &stats.SyncedSize,
		&stats.NASSyncedCount, &stats.R2SyncedCount,
	)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// UnsyncedMedia holds metadata for a media file not yet in the sync queue.
type UnsyncedMedia struct {
	ID          int
	FilePath    string
	FileName    string
	FileSize    *int64
	ThockNumber string
	MediaType   string
}

// GetUnsyncedRoomEntryMedia returns room entry media that don't have sync queue entries
func (r *MediaSyncRepository) GetUnsyncedRoomEntryMedia(ctx context.Context, limit int) ([]UnsyncedMedia, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rem.id, rem.file_path, rem.file_name, rem.file_size, rem.thock_number, rem.media_type
		FROM room_entry_media rem
		LEFT JOIN media_sync_queue msq ON msq.media_source = 'room_entry' AND msq.media_id = rem.id
		WHERE msq.id IS NULL
		ORDER BY rem.id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UnsyncedMedia
	for rows.Next() {
		var m UnsyncedMedia
		if err := rows.Scan(&m.ID, &m.FilePath, &m.FileName, &m.FileSize, &m.ThockNumber, &m.MediaType); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
}

// GetUnsyncedGatePassMedia returns gate pass media that don't have sync queue entries
func (r *MediaSyncRepository) GetUnsyncedGatePassMedia(ctx context.Context, limit int) ([]UnsyncedMedia, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT gpm.id, gpm.file_path, gpm.file_name, gpm.file_size, gpm.thock_number, gpm.media_type
		FROM gate_pass_media gpm
		LEFT JOIN media_sync_queue msq ON msq.media_source = 'gate_pass' AND msq.media_id = gpm.id
		WHERE msq.id IS NULL
		ORDER BY gpm.id ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UnsyncedMedia
	for rows.Next() {
		var m UnsyncedMedia
		if err := rows.Scan(&m.ID, &m.FilePath, &m.FileName, &m.FileSize, &m.ThockNumber, &m.MediaType); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
}

// UpdateMediaCloudSynced updates the cloud_synced flag on the source media table
func (r *MediaSyncRepository) UpdateMediaCloudSynced(ctx context.Context, source string, mediaID int, synced bool, r2Key string) error {
	var table string
	switch source {
	case "room_entry":
		table = "room_entry_media"
	case "gate_pass":
		table = "gate_pass_media"
	default:
		return fmt.Errorf("unknown media source: %s", source)
	}

	_, err := r.pool.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET cloud_synced = $1, r2_key = $2 WHERE id = $3`, table),
		synced, r2Key, mediaID)
	return err
}

// UpdateNASSynced updates nas_synced flag for a specific sync record
func (r *MediaSyncRepository) UpdateNASSynced(ctx context.Context, source string, mediaID int, synced bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue SET nas_synced = $1
		WHERE media_source = $2 AND media_id = $3`, synced, source, mediaID)
	return err
}

// UpdateR2SyncedFlag updates r2_synced flag for a specific sync record
func (r *MediaSyncRepository) UpdateR2SyncedFlag(ctx context.Context, source string, mediaID int, synced bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE media_sync_queue SET r2_synced = $1
		WHERE media_source = $2 AND media_id = $3`, synced, source, mediaID)
	return err
}

// GetSyncedR2Keys returns all synced R2 keys (for restore operations)
func (r *MediaSyncRepository) GetSyncedR2Keys(ctx context.Context) ([]models.MediaSyncRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, media_source, media_id, local_file_path, r2_key, file_size,
		       local_synced, nas_synced, r2_synced
		FROM media_sync_queue
		WHERE r2_synced = true OR nas_synced = true
		ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.MediaSyncRecord
	for rows.Next() {
		var rec models.MediaSyncRecord
		if err := rows.Scan(&rec.ID, &rec.MediaSource, &rec.MediaID, &rec.LocalFilePath, &rec.R2Key, &rec.FileSize,
			&rec.LocalSynced, &rec.NASSynced, &rec.R2Synced); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}
