package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// MediaSyncService manages background sync of media files to R2 and NAS.
// Workers poll the media_sync_queue and upload files to configured backends.
type MediaSyncService struct {
	repo         *repositories.MediaSyncRepository
	r2Backend    *S3Backend
	nasBackend   *S3Backend
	localBaseDir string // absolute path to bulk storage root (e.g., /mass-pool/shared)
	workerCount  int
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewMediaSyncService creates a new sync service.
// r2Backend and nasBackend may be nil if not configured.
func NewMediaSyncService(
	repo *repositories.MediaSyncRepository,
	r2Backend, nasBackend *S3Backend,
	localBaseDir string,
) *MediaSyncService {
	return &MediaSyncService{
		repo:         repo,
		r2Backend:    r2Backend,
		nasBackend:   nasBackend,
		localBaseDir: localBaseDir,
		workerCount:  2,
		pollInterval: 5 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

// Start launches background sync workers.
func (s *MediaSyncService) Start() {
	if s.r2Backend == nil && s.nasBackend == nil {
		log.Println("[MediaSync] No cloud backends configured, sync workers disabled")
		return
	}

	log.Printf("[MediaSync] Starting %d sync workers (R2: %v, NAS: %v)",
		s.workerCount, s.r2Backend != nil, s.nasBackend != nil)

	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
}

// Stop gracefully shuts down all workers.
func (s *MediaSyncService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Println("[MediaSync] All workers stopped")
}

// ---------------------------------------------------------------------------
// Enqueue — called by handlers after saving media metadata
// ---------------------------------------------------------------------------

// EnqueueMedia creates a sync queue record for a newly uploaded media file.
// source is "room_entry" or "gate_pass".
func (s *MediaSyncService) EnqueueMedia(ctx context.Context, source string, mediaID int, filePath, fileName string, fileSize int64, thockNumber, mediaType string) error {
	if s.r2Backend == nil && s.nasBackend == nil {
		return nil // no targets configured, skip silently
	}

	// Construct R2 key: {thock_number}/{media_type}_{file_name}
	// All files for one thock in a single folder, prefixed by media_type
	r2Key := fmt.Sprintf("%s/%s_%s", thockNumber, mediaType, fileName)

	// Construct absolute local path
	localPath := filepath.Join(s.localBaseDir, filePath)

	record := &models.MediaSyncRecord{
		MediaSource:     source,
		MediaID:         mediaID,
		LocalFilePath:   localPath,
		R2Key:           r2Key,
		FileSize:        fileSize,
		LocalSynced:     true,
		PrimaryLocation: "local",
	}

	return s.repo.Enqueue(ctx, record)
}

// ---------------------------------------------------------------------------
// Background worker loop
// ---------------------------------------------------------------------------

func (s *MediaSyncService) worker(id int) {
	defer s.wg.Done()
	log.Printf("[MediaSync] Worker %d started", id)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Printf("[MediaSync] Worker %d stopping", id)
			return
		case <-ticker.C:
			s.processOne(context.Background(), id)
		}
	}
}

// processOne atomically claims and processes a single pending sync record.
func (s *MediaSyncService) processOne(ctx context.Context, workerID int) {
	record, err := s.repo.PickNext(ctx)
	if err != nil {
		// "no rows in result set" is normal when queue is empty
		if !strings.Contains(err.Error(), "no rows") {
			log.Printf("[MediaSync] Worker %d: PickNext error: %v", workerID, err)
		}
		return
	}

	log.Printf("[MediaSync] Worker %d: processing %s #%d → %s",
		workerID, record.MediaSource, record.MediaID, record.R2Key)

	// Resolve actual file on disk (handle video conversion renaming)
	localPath, r2Key, err := s.resolveLocalFile(record)
	if err != nil {
		// File missing — handle video conversion wait or skip
		s.handleMissingFile(ctx, record, workerID, err)
		return
	}
	record.R2Key = r2Key

	// Open file for upload
	f, err := os.Open(localPath)
	if err != nil {
		s.repo.MarkFailed(ctx, record.ID, "open file: "+err.Error())
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.repo.MarkFailed(ctx, record.ID, "stat file: "+err.Error())
		return
	}
	fileSize := info.Size()

	var syncErrors []string

	// Upload to NAS (MinIO on TrueNAS)
	if s.nasBackend != nil && !record.NASSynced {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			syncErrors = append(syncErrors, "NAS seek: "+err.Error())
		} else if err := s.nasBackend.Upload(ctx, record.R2Key, f, fileSize); err != nil {
			log.Printf("[MediaSync] Worker %d: NAS upload failed: %v", workerID, err)
			syncErrors = append(syncErrors, "NAS: "+err.Error())
		} else {
			s.repo.MarkNASSynced(ctx, record.ID)
			record.NASSynced = true
			log.Printf("[MediaSync] Worker %d: NAS synced %s", workerID, record.R2Key)
		}
	}

	// Upload to R2 (Cloudflare)
	if s.r2Backend != nil && !record.R2Synced {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			syncErrors = append(syncErrors, "R2 seek: "+err.Error())
		} else if err := s.r2Backend.Upload(ctx, record.R2Key, f, fileSize); err != nil {
			log.Printf("[MediaSync] Worker %d: R2 upload failed: %v", workerID, err)
			syncErrors = append(syncErrors, "R2: "+err.Error())
		} else {
			s.repo.MarkR2Synced(ctx, record.ID)
			record.R2Synced = true
			// Update source media table with cloud_synced flag + r2_key
			s.repo.UpdateMediaCloudSynced(ctx, record.MediaSource, record.MediaID, true, record.R2Key)
			log.Printf("[MediaSync] Worker %d: R2 synced %s", workerID, record.R2Key)
		}
	}

	// Determine final state
	nasOK := s.nasBackend == nil || record.NASSynced
	r2OK := s.r2Backend == nil || record.R2Synced

	if nasOK && r2OK {
		s.repo.MarkAllSynced(ctx, record.ID)
		log.Printf("[MediaSync] Worker %d: fully synced %s", workerID, record.R2Key)
	} else if len(syncErrors) > 0 {
		s.repo.MarkFailed(ctx, record.ID, strings.Join(syncErrors, "; "))
	}
}

// resolveLocalFile checks if the file exists on disk, handling video conversion
// where .MOV/.AVI/etc. may have been converted to .mp4.
func (s *MediaSyncService) resolveLocalFile(record *models.MediaSyncRecord) (localPath, r2Key string, err error) {
	localPath = record.LocalFilePath
	r2Key = record.R2Key

	if _, err := os.Stat(localPath); err == nil {
		return localPath, r2Key, nil
	}

	// Try .mp4 variant (video conversion changes extension)
	mp4Path := strings.TrimSuffix(localPath, filepath.Ext(localPath)) + ".mp4"
	if _, err := os.Stat(mp4Path); err == nil {
		// Update R2 key to match converted filename
		r2Key = strings.TrimSuffix(r2Key, filepath.Ext(r2Key)) + ".mp4"
		return mp4Path, r2Key, nil
	}

	return "", "", fmt.Errorf("file not found: %s", localPath)
}

// handleMissingFile decides whether to retry (video converting) or skip.
func (s *MediaSyncService) handleMissingFile(ctx context.Context, record *models.MediaSyncRecord, workerID int, fileErr error) {
	ext := strings.ToLower(filepath.Ext(record.LocalFilePath))
	videoExts := map[string]bool{
		".mov": true, ".avi": true, ".mkv": true, ".wmv": true,
		".mts": true, ".3gp": true, ".flv": true, ".webm": true, ".m4v": true,
	}

	if videoExts[ext] && record.RetryCount < 3 {
		log.Printf("[MediaSync] Worker %d: file not found (video may be converting), retrying: %s",
			workerID, record.LocalFilePath)
		s.repo.MarkRetry(ctx, record.ID, "file not found (video converting?)", 30*time.Second)
	} else {
		log.Printf("[MediaSync] Worker %d: file not found, skipping: %s", workerID, record.LocalFilePath)
		s.repo.MarkSkipped(ctx, record.ID, fileErr.Error())
	}
}

// ---------------------------------------------------------------------------
// Initial sync — enqueue all existing media that isn't in the sync queue
// ---------------------------------------------------------------------------

// RunInitialSync scans room_entry_media and gate_pass_media tables,
// enqueuing any records that don't yet have a sync queue entry.
func (s *MediaSyncService) RunInitialSync(ctx context.Context) (int, error) {
	if s.r2Backend == nil && s.nasBackend == nil {
		return 0, fmt.Errorf("no cloud backends configured")
	}

	enqueued := 0
	batchSize := 100

	// Room entry media
	for {
		items, err := s.repo.GetUnsyncedRoomEntryMedia(ctx, batchSize)
		if err != nil {
			return enqueued, fmt.Errorf("get unsynced room entry media: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			var fileSize int64
			if item.FileSize != nil {
				fileSize = *item.FileSize
			}

			r2Key := fmt.Sprintf("room-entry/%s/%s/%s", item.ThockNumber, item.MediaType, item.FileName)
			localPath := filepath.Join(s.localBaseDir, item.FilePath)

			record := &models.MediaSyncRecord{
				MediaSource:     "room_entry",
				MediaID:         item.ID,
				LocalFilePath:   localPath,
				R2Key:           r2Key,
				FileSize:        fileSize,
				LocalSynced:     true,
				PrimaryLocation: "local",
			}

			if err := s.repo.Enqueue(ctx, record); err != nil {
				log.Printf("[MediaSync] Failed to enqueue room_entry #%d: %v", item.ID, err)
				continue
			}
			enqueued++
		}
	}

	// Gate pass media
	for {
		items, err := s.repo.GetUnsyncedGatePassMedia(ctx, batchSize)
		if err != nil {
			return enqueued, fmt.Errorf("get unsynced gate pass media: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			var fileSize int64
			if item.FileSize != nil {
				fileSize = *item.FileSize
			}

			r2Key := fmt.Sprintf("gate-pass/%s/%s/%s", item.ThockNumber, item.MediaType, item.FileName)
			localPath := filepath.Join(s.localBaseDir, item.FilePath)

			record := &models.MediaSyncRecord{
				MediaSource:     "gate_pass",
				MediaID:         item.ID,
				LocalFilePath:   localPath,
				R2Key:           r2Key,
				FileSize:        fileSize,
				LocalSynced:     true,
				PrimaryLocation: "local",
			}

			if err := s.repo.Enqueue(ctx, record); err != nil {
				log.Printf("[MediaSync] Failed to enqueue gate_pass #%d: %v", item.ID, err)
				continue
			}
			enqueued++
		}
	}

	log.Printf("[MediaSync] Initial sync: enqueued %d files", enqueued)
	return enqueued, nil
}

// ---------------------------------------------------------------------------
// Admin operations
// ---------------------------------------------------------------------------

// GetStats returns aggregate sync status counts for the admin dashboard.
func (s *MediaSyncService) GetStats(ctx context.Context) (*models.MediaSyncStats, error) {
	return s.repo.GetStats(ctx)
}

// RetryAllFailed resets all failed sync records back to pending for retry.
func (s *MediaSyncService) RetryAllFailed(ctx context.Context) (int64, error) {
	return s.repo.ResetAllFailed(ctx)
}

// BulkRestore downloads all cloud-synced media files that are missing locally.
// Tries NAS first (faster, same network), then R2.
// Returns a RestoreProgress summary.
func (s *MediaSyncService) BulkRestore(ctx context.Context) (*models.RestoreProgress, error) {
	if s.r2Backend == nil && s.nasBackend == nil {
		return nil, fmt.Errorf("no cloud backends configured")
	}

	records, err := s.repo.GetSyncedR2Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get synced records: %w", err)
	}

	progress := &models.RestoreProgress{
		TotalFiles: len(records),
	}

	for _, rec := range records {
		// Check if local file already exists
		if _, err := os.Stat(rec.LocalFilePath); err == nil {
			progress.RestoredFiles++ // already exists, count as restored
			progress.BytesRestored += rec.FileSize
			continue
		}

		progress.CurrentFile = rec.R2Key

		// Try downloading from NAS first, then R2
		var reader io.ReadCloser
		var dlErr error

		if s.nasBackend != nil && rec.NASSynced {
			reader, _, dlErr = s.nasBackend.Download(ctx, rec.R2Key)
		}
		if reader == nil && s.r2Backend != nil && rec.R2Synced {
			reader, _, dlErr = s.r2Backend.Download(ctx, rec.R2Key)
		}

		if reader == nil {
			log.Printf("[MediaSync] BulkRestore: failed to download %s: %v", rec.R2Key, dlErr)
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(rec.LocalFilePath), 0755); err != nil {
			reader.Close()
			log.Printf("[MediaSync] BulkRestore: failed to create dir for %s: %v", rec.LocalFilePath, err)
			continue
		}

		// Write to local disk
		f, err := os.Create(rec.LocalFilePath)
		if err != nil {
			reader.Close()
			log.Printf("[MediaSync] BulkRestore: failed to create %s: %v", rec.LocalFilePath, err)
			continue
		}

		written, err := io.Copy(f, reader)
		f.Close()
		reader.Close()

		if err != nil {
			os.Remove(rec.LocalFilePath) // clean up partial file
			log.Printf("[MediaSync] BulkRestore: write failed for %s: %v", rec.LocalFilePath, err)
			continue
		}

		// Mark local as synced in the queue
		s.repo.MarkLocalSynced(ctx, rec.ID)

		progress.RestoredFiles++
		progress.BytesRestored += written
		log.Printf("[MediaSync] BulkRestore: restored %s (%.1f MB)", rec.R2Key, float64(written)/1024/1024)
	}

	progress.BytesTotal = progress.BytesRestored
	log.Printf("[MediaSync] BulkRestore complete: %d/%d files restored", progress.RestoredFiles, progress.TotalFiles)
	return progress, nil
}
