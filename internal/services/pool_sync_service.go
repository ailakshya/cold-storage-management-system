package services

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// PoolSyncService syncs all files from local storage pools to NAS (RustFS/MinIO).
// Each pool gets its own S3 prefix: bulk/..., highspeed/..., archives/..., backups/...
type PoolSyncService struct {
	repo       *repositories.PoolSyncRepository
	nasBackend *S3Backend
	pools      map[string]string // pool_name -> local_root_path

	workerCount  int
	pollInterval time.Duration

	// Per-pool scan intervals
	scanIntervals map[string]time.Duration

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewPoolSyncService creates a new pool sync service.
func NewPoolSyncService(
	repo *repositories.PoolSyncRepository,
	nasBackend *S3Backend,
	pools map[string]string,
) *PoolSyncService {
	scanIntervals := map[string]time.Duration{
		"bulk":      15 * time.Minute,
		"highspeed": 15 * time.Minute,
		"archives":  1 * time.Hour,
		"backups":   1 * time.Hour,
	}

	return &PoolSyncService{
		repo:          repo,
		nasBackend:    nasBackend,
		pools:         pools,
		workerCount:   3,
		pollInterval:  5 * time.Second,
		scanIntervals: scanIntervals,
		stopCh:        make(chan struct{}),
	}
}

// Start launches scanner goroutines and upload workers.
func (s *PoolSyncService) Start() {
	if s.nasBackend == nil {
		log.Println("[PoolSync] NAS backend not configured, pool sync disabled")
		return
	}

	log.Printf("[PoolSync] Starting %d upload workers for %d pools", s.workerCount, len(s.pools))

	// Start a scanner goroutine for each pool
	for poolName := range s.pools {
		s.wg.Add(1)
		go s.scanLoop(poolName)
	}

	// Start upload workers
	for i := 0; i < s.workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}
}

// Stop gracefully shuts down all goroutines.
func (s *PoolSyncService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	log.Println("[PoolSync] All workers stopped")
}

// ScanPool manually triggers a scan for a single pool.
func (s *PoolSyncService) ScanPool(ctx context.Context, poolName string) error {
	rootPath, ok := s.pools[poolName]
	if !ok {
		return nil
	}
	s.scanPoolDir(ctx, poolName, rootPath)
	return nil
}

// ScanAllPools manually triggers a scan for all pools.
func (s *PoolSyncService) ScanAllPools(ctx context.Context) {
	for poolName, rootPath := range s.pools {
		s.scanPoolDir(ctx, poolName, rootPath)
	}
}

// GetOverview returns aggregate stats for all pools.
func (s *PoolSyncService) GetOverview(ctx context.Context) (*models.PoolSyncOverview, error) {
	return s.repo.GetOverview(ctx)
}

// GetScanStates returns the last scan info for all pools.
func (s *PoolSyncService) GetScanStates(ctx context.Context) ([]models.PoolScanState, error) {
	return s.repo.GetScanStates(ctx)
}

// RetryFailed resets failed records back to pending.
func (s *PoolSyncService) RetryFailed(ctx context.Context, poolName string) (int64, error) {
	return s.repo.ResetFailed(ctx, poolName)
}

// GetRecentFailed returns recent failed records.
func (s *PoolSyncService) GetRecentFailed(ctx context.Context, poolName string, limit int) ([]models.PoolSyncRecord, error) {
	return s.repo.GetRecentFailed(ctx, poolName, limit)
}

// ---------------------------------------------------------------------------
// Scanner goroutine
// ---------------------------------------------------------------------------

func (s *PoolSyncService) scanLoop(poolName string) {
	defer s.wg.Done()

	rootPath, ok := s.pools[poolName]
	if !ok {
		return
	}

	interval := s.scanIntervals[poolName]
	log.Printf("[PoolSync] Scanner for '%s' started (interval: %s, root: %s)", poolName, interval, rootPath)

	// Run initial scan after a brief delay to let the server start up
	select {
	case <-s.stopCh:
		return
	case <-time.After(30 * time.Second):
	}

	s.scanPoolDir(context.Background(), poolName, rootPath)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Printf("[PoolSync] Scanner for '%s' stopping", poolName)
			return
		case <-ticker.C:
			s.scanPoolDir(context.Background(), poolName, rootPath)
		}
	}
}

// shouldSkip returns true if the file/dir should be excluded from sync.
func shouldSkip(name string, isDir bool) bool {
	// Skip hidden files/dirs
	if strings.HasPrefix(name, ".") {
		return true
	}
	// Skip thumbnail directories
	if isDir && (name == ".thumbs" || name == "thumbs" || name == "__MACOSX") {
		return true
	}
	// Skip temp files
	if !isDir {
		if strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".part") ||
			strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".swp") {
			return true
		}
	}
	return false
}

func (s *PoolSyncService) scanPoolDir(ctx context.Context, poolName, rootPath string) {
	// Prevent concurrent scans of the same pool
	acquired, err := s.repo.SetScanning(ctx, poolName, true)
	if err != nil {
		log.Printf("[PoolSync] Scanner '%s': failed to set scanning flag: %v", poolName, err)
		return
	}
	if !acquired {
		log.Printf("[PoolSync] Scanner '%s': already scanning, skipping", poolName)
		return
	}

	startTime := time.Now()
	var filesFound, filesEnqueued int64

	log.Printf("[PoolSync] Scanner '%s': starting scan of %s", poolName, rootPath)

	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}

		// Check for shutdown
		select {
		case <-s.stopCh:
			return filepath.SkipAll
		default:
		}

		if shouldSkip(d.Name(), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		filesFound++

		info, err := d.Info()
		if err != nil {
			return nil // skip files we can't stat
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}
		// Use forward slashes for S3 keys
		relPath = filepath.ToSlash(relPath)
		s3Key := poolName + "/" + relPath

		rec := &models.PoolSyncRecord{
			PoolName:     poolName,
			RelativePath: relPath,
			S3Key:        s3Key,
			FileSize:     info.Size(),
			FileMtime:    info.ModTime().UTC().Truncate(time.Second),
		}

		enqueued, err := s.repo.UpsertFile(ctx, rec)
		if err != nil {
			log.Printf("[PoolSync] Scanner '%s': upsert error for %s: %v", poolName, relPath, err)
			return nil
		}
		if enqueued {
			filesEnqueued++
		}

		return nil
	})

	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		log.Printf("[PoolSync] Scanner '%s': walk error: %v", poolName, err)
	}

	s.repo.UpdateScanState(ctx, poolName, filesFound, filesEnqueued, durationMs)

	log.Printf("[PoolSync] Scanner '%s': done in %dms — %d files found, %d enqueued",
		poolName, durationMs, filesFound, filesEnqueued)
}

// ---------------------------------------------------------------------------
// Upload worker
// ---------------------------------------------------------------------------

func (s *PoolSyncService) worker(id int) {
	defer s.wg.Done()
	log.Printf("[PoolSync] Worker %d started", id)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Printf("[PoolSync] Worker %d stopping", id)
			return
		case <-ticker.C:
			s.processOne(context.Background(), id)
		}
	}
}

func (s *PoolSyncService) processOne(ctx context.Context, workerID int) {
	record, err := s.repo.PickNext(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "no rows") {
			log.Printf("[PoolSync] Worker %d: PickNext error: %v", workerID, err)
		}
		return
	}

	rootPath, ok := s.pools[record.PoolName]
	if !ok {
		s.repo.MarkSkipped(ctx, record.ID, "unknown pool: "+record.PoolName)
		return
	}

	localPath := filepath.Join(rootPath, filepath.FromSlash(record.RelativePath))

	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.repo.MarkSkipped(ctx, record.ID, "file not found")
		} else {
			s.repo.MarkFailed(ctx, record.ID, "open: "+err.Error())
		}
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.repo.MarkFailed(ctx, record.ID, "stat: "+err.Error())
		return
	}

	err = s.nasBackend.Upload(ctx, record.S3Key, f, info.Size())
	if err != nil {
		log.Printf("[PoolSync] Worker %d: upload failed %s: %v", workerID, record.S3Key, err)
		s.repo.MarkFailed(ctx, record.ID, "upload: "+err.Error())
		return
	}

	s.repo.MarkSynced(ctx, record.ID)
	log.Printf("[PoolSync] Worker %d: synced %s (%.1f MB)", workerID, record.S3Key, float64(info.Size())/1024/1024)
}
