package models

import "time"

// PoolSyncRecord represents a file queued for sync to NAS (RustFS/MinIO).
type PoolSyncRecord struct {
	ID           int64      `json:"id"`
	PoolName     string     `json:"pool_name"`
	RelativePath string     `json:"relative_path"`
	S3Key        string     `json:"s3_key"`
	FileSize     int64      `json:"file_size"`
	FileMtime    time.Time  `json:"file_mtime"`
	SyncStatus   string     `json:"sync_status"`
	RetryCount   int        `json:"retry_count"`
	MaxRetries   int        `json:"max_retries"`
	LastError    *string    `json:"last_error,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
}

// PoolSyncStats holds per-pool aggregate sync counts.
type PoolSyncStats struct {
	PoolName   string `json:"pool_name"`
	Pending    int    `json:"pending"`
	Uploading  int    `json:"uploading"`
	Synced     int    `json:"synced"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	TotalFiles int    `json:"total_files"`
	TotalSize  int64  `json:"total_size_bytes"`
	SyncedSize int64  `json:"synced_size_bytes"`
}

// PoolSyncOverview aggregates all pools into one summary.
type PoolSyncOverview struct {
	Pools        []PoolSyncStats `json:"pools"`
	TotalFiles   int             `json:"total_files"`
	TotalSize    int64           `json:"total_size_bytes"`
	SyncedFiles  int             `json:"synced_files"`
	SyncedSize   int64           `json:"synced_size_bytes"`
	PendingFiles int             `json:"pending_files"`
	FailedFiles  int             `json:"failed_files"`
}

// PoolScanState tracks last scan for a pool.
type PoolScanState struct {
	PoolName       string     `json:"pool_name"`
	LastScanAt     *time.Time `json:"last_scan_at,omitempty"`
	FilesFound     int64      `json:"files_found"`
	FilesEnqueued  int64      `json:"files_enqueued"`
	ScanDurationMs int64      `json:"scan_duration_ms"`
	IsScanning     bool       `json:"is_scanning"`
}
