package models

import "time"

// MediaSyncRecord represents a pending or completed media sync to cloud storage
type MediaSyncRecord struct {
	ID            int        `json:"id"`
	MediaSource   string     `json:"media_source"`   // "room_entry" or "gate_pass"
	MediaID       int        `json:"media_id"`        // FK to room_entry_media.id or gate_pass_media.id
	LocalFilePath string     `json:"local_file_path"` // Absolute path on disk
	R2Key         string     `json:"r2_key"`          // Target key in S3 buckets
	FileSize      int64      `json:"file_size"`
	SyncStatus    string     `json:"sync_status"` // pending, uploading, synced, failed, skipped

	// Per-target sync tracking (independent)
	LocalSynced     bool   `json:"local_synced"`
	NASSynced       bool   `json:"nas_synced"`
	R2Synced        bool   `json:"r2_synced"`
	PrimaryLocation string `json:"primary_location"` // where file was originally uploaded: local, nas, r2

	// Retry logic
	RetryCount int     `json:"retry_count"`
	MaxRetries int     `json:"max_retries"`
	LastError  *string `json:"last_error,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
}

// MediaSyncStats holds aggregate sync status counts for the admin dashboard
type MediaSyncStats struct {
	Pending    int   `json:"pending"`
	Uploading  int   `json:"uploading"`
	Synced     int   `json:"synced"`
	Failed     int   `json:"failed"`
	Skipped    int   `json:"skipped"`
	TotalFiles int   `json:"total_files"`
	TotalSize  int64 `json:"total_size_bytes"`
	SyncedSize int64 `json:"synced_size_bytes"`

	// Per-target counts
	NASSyncedCount int `json:"nas_synced_count"`
	R2SyncedCount  int `json:"r2_synced_count"`
}

// RestoreProgress tracks bulk restore progress
type RestoreProgress struct {
	TotalFiles    int    `json:"total_files"`
	RestoredFiles int    `json:"restored_files"`
	CurrentFile   string `json:"current_file"`
	BytesTotal    int64  `json:"bytes_total"`
	BytesRestored int64  `json:"bytes_restored"`
	Error         string `json:"error,omitempty"`
}
