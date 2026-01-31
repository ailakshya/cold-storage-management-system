package repositories

import (
	"context"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PoolSyncRepository struct {
	pool *pgxpool.Pool
}

func NewPoolSyncRepository(pool *pgxpool.Pool) *PoolSyncRepository {
	return &PoolSyncRepository{pool: pool}
}

// UpsertFile inserts a new file or re-queues it if size/mtime changed.
func (r *PoolSyncRepository) UpsertFile(ctx context.Context, rec *models.PoolSyncRecord) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO pool_sync_queue
			(pool_name, relative_path, s3_key, file_size, file_mtime, sync_status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		ON CONFLICT (pool_name, relative_path) DO UPDATE SET
			file_size = EXCLUDED.file_size,
			file_mtime = EXCLUDED.file_mtime,
			s3_key = EXCLUDED.s3_key,
			sync_status = 'pending',
			retry_count = 0,
			last_error = NULL,
			next_retry_at = NULL
		WHERE pool_sync_queue.file_size != EXCLUDED.file_size
		   OR pool_sync_queue.file_mtime != EXCLUDED.file_mtime`,
		rec.PoolName, rec.RelativePath, rec.S3Key, rec.FileSize, rec.FileMtime,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// PickNext atomically claims the next pending record for processing.
func (r *PoolSyncRepository) PickNext(ctx context.Context) (*models.PoolSyncRecord, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE pool_sync_queue
		SET sync_status = 'uploading', started_at = NOW()
		WHERE id = (
			SELECT id FROM pool_sync_queue
			WHERE sync_status IN ('pending', 'failed')
			  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
			  AND retry_count < max_retries
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, pool_name, relative_path, s3_key, file_size, file_mtime,
			sync_status, retry_count, max_retries, last_error,
			created_at, started_at, completed_at, next_retry_at`)

	rec := &models.PoolSyncRecord{}
	err := row.Scan(
		&rec.ID, &rec.PoolName, &rec.RelativePath, &rec.S3Key, &rec.FileSize, &rec.FileMtime,
		&rec.SyncStatus, &rec.RetryCount, &rec.MaxRetries, &rec.LastError,
		&rec.CreatedAt, &rec.StartedAt, &rec.CompletedAt, &rec.NextRetryAt,
	)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// MarkSynced marks a record as successfully synced.
func (r *PoolSyncRepository) MarkSynced(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pool_sync_queue
		SET sync_status = 'synced', completed_at = NOW()
		WHERE id = $1`, id)
	return err
}

// MarkFailed increments retry count with exponential backoff.
func (r *PoolSyncRepository) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	backoffs := []time.Duration{
		30 * time.Second,
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
	}

	var retryCount int
	err := r.pool.QueryRow(ctx, `SELECT retry_count FROM pool_sync_queue WHERE id = $1`, id).Scan(&retryCount)
	if err != nil {
		return err
	}

	idx := retryCount
	if idx >= len(backoffs) {
		idx = len(backoffs) - 1
	}
	nextRetry := time.Now().Add(backoffs[idx])

	_, err = r.pool.Exec(ctx, `
		UPDATE pool_sync_queue
		SET sync_status = 'failed', retry_count = retry_count + 1,
		    last_error = $2, next_retry_at = $3
		WHERE id = $1`, id, errMsg, nextRetry)
	return err
}

// MarkSkipped marks a record as skipped (file not found, etc.).
func (r *PoolSyncRepository) MarkSkipped(ctx context.Context, id int64, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pool_sync_queue
		SET sync_status = 'skipped', last_error = $2, completed_at = NOW()
		WHERE id = $1`, id, reason)
	return err
}

// ResetFailed resets failed records back to pending. If poolName is empty, resets all pools.
func (r *PoolSyncRepository) ResetFailed(ctx context.Context, poolName string) (int64, error) {
	var tag interface{ RowsAffected() int64 }
	var err error
	if poolName != "" {
		tag, err = r.pool.Exec(ctx, `
			UPDATE pool_sync_queue
			SET sync_status = 'pending', retry_count = 0, last_error = NULL, next_retry_at = NULL
			WHERE sync_status = 'failed' AND pool_name = $1`, poolName)
	} else {
		tag, err = r.pool.Exec(ctx, `
			UPDATE pool_sync_queue
			SET sync_status = 'pending', retry_count = 0, last_error = NULL, next_retry_at = NULL
			WHERE sync_status = 'failed'`)
	}
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetStatsByPool returns per-pool aggregate sync counts.
func (r *PoolSyncRepository) GetStatsByPool(ctx context.Context) ([]models.PoolSyncStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			pool_name,
			COUNT(*) FILTER (WHERE sync_status = 'pending') as pending,
			COUNT(*) FILTER (WHERE sync_status = 'uploading') as uploading,
			COUNT(*) FILTER (WHERE sync_status = 'synced') as synced,
			COUNT(*) FILTER (WHERE sync_status = 'failed') as failed,
			COUNT(*) FILTER (WHERE sync_status = 'skipped') as skipped,
			COUNT(*) as total_files,
			COALESCE(SUM(file_size), 0) as total_size,
			COALESCE(SUM(file_size) FILTER (WHERE sync_status = 'synced'), 0) as synced_size
		FROM pool_sync_queue
		GROUP BY pool_name
		ORDER BY pool_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PoolSyncStats
	for rows.Next() {
		var s models.PoolSyncStats
		if err := rows.Scan(&s.PoolName, &s.Pending, &s.Uploading, &s.Synced, &s.Failed, &s.Skipped,
			&s.TotalFiles, &s.TotalSize, &s.SyncedSize); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetOverview returns aggregate stats across all pools.
func (r *PoolSyncRepository) GetOverview(ctx context.Context) (*models.PoolSyncOverview, error) {
	poolStats, err := r.GetStatsByPool(ctx)
	if err != nil {
		return nil, err
	}

	overview := &models.PoolSyncOverview{Pools: poolStats}
	for _, p := range poolStats {
		overview.TotalFiles += p.TotalFiles
		overview.TotalSize += p.TotalSize
		overview.SyncedFiles += p.Synced
		overview.SyncedSize += p.SyncedSize
		overview.PendingFiles += p.Pending
		overview.FailedFiles += p.Failed
	}
	return overview, nil
}

// GetScanStates returns scan state for all pools.
func (r *PoolSyncRepository) GetScanStates(ctx context.Context) ([]models.PoolScanState, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pool_name, last_scan_at, files_found, files_enqueued, scan_duration_ms, is_scanning
		FROM pool_sync_scan_state
		ORDER BY pool_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []models.PoolScanState
	for rows.Next() {
		var s models.PoolScanState
		if err := rows.Scan(&s.PoolName, &s.LastScanAt, &s.FilesFound, &s.FilesEnqueued, &s.ScanDurationMs, &s.IsScanning); err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	return states, nil
}

// SetScanning atomically sets the is_scanning flag. Returns false if already scanning.
func (r *PoolSyncRepository) SetScanning(ctx context.Context, poolName string, scanning bool) (bool, error) {
	if scanning {
		tag, err := r.pool.Exec(ctx, `
			UPDATE pool_sync_scan_state SET is_scanning = TRUE
			WHERE pool_name = $1 AND is_scanning = FALSE`, poolName)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE pool_sync_scan_state SET is_scanning = FALSE
		WHERE pool_name = $1`, poolName)
	return true, err
}

// UpdateScanState updates scan results for a pool.
func (r *PoolSyncRepository) UpdateScanState(ctx context.Context, poolName string, filesFound, filesEnqueued int64, durationMs int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE pool_sync_scan_state
		SET last_scan_at = NOW(), files_found = $2, files_enqueued = $3,
		    scan_duration_ms = $4, is_scanning = FALSE
		WHERE pool_name = $1`, poolName, filesFound, filesEnqueued, durationMs)
	return err
}

// GetRecentFailed returns recent failed records for admin inspection.
func (r *PoolSyncRepository) GetRecentFailed(ctx context.Context, poolName string, limit int) ([]models.PoolSyncRecord, error) {
	var rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
	}
	var err error

	if poolName != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, pool_name, relative_path, s3_key, file_size, file_mtime,
			       sync_status, retry_count, max_retries, last_error,
			       created_at, started_at, completed_at, next_retry_at
			FROM pool_sync_queue
			WHERE sync_status = 'failed' AND pool_name = $1
			ORDER BY started_at DESC NULLS LAST
			LIMIT $2`, poolName, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, pool_name, relative_path, s3_key, file_size, file_mtime,
			       sync_status, retry_count, max_retries, last_error,
			       created_at, started_at, completed_at, next_retry_at
			FROM pool_sync_queue
			WHERE sync_status = 'failed'
			ORDER BY started_at DESC NULLS LAST
			LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.PoolSyncRecord
	for rows.Next() {
		var r models.PoolSyncRecord
		if err := rows.Scan(&r.ID, &r.PoolName, &r.RelativePath, &r.S3Key, &r.FileSize, &r.FileMtime,
			&r.SyncStatus, &r.RetryCount, &r.MaxRetries, &r.LastError,
			&r.CreatedAt, &r.StartedAt, &r.CompletedAt, &r.NextRetryAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}
