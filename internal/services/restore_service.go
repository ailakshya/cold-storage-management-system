package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"

	"cold-backend/internal/config"
	"cold-backend/migrations"
)

// StorageFileInfo represents file metadata from storage provider
type StorageFileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// StorageProvider defines the interface for file storage operations
type StorageProvider interface {
	StoreBackup(filename string, data []byte) (string, error)
	GetBackup(filename string) ([]byte, error)
	ListBackups() ([]StorageFileInfo, error)
	DeleteBackup(filename string) error
	// We might need list capability too if we want to replace ListAvailableDates logic
}

// RestoreService handles point-in-time database restoration from R2
type RestoreService struct {
	pool            *pgxpool.Pool
	connStr         string
	storage         StorageProvider // Replaces backupDir string
	pendingTokens   map[string]*RestoreToken
	tokenMu         sync.RWMutex
	lastRestoreTime time.Time
	restoreCooldown time.Duration
}

// RestoreToken holds confirmation token for restore operation
type RestoreToken struct {
	Token       string
	SnapshotKey string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	UserID      int
	IsLocal     bool // True if restoring from local file
}

// RestoreDate represents available restore points for a date
type RestoreDate struct {
	Date         string `json:"date"`
	Count        int    `json:"count"`
	LatestTime   string `json:"latest_time"`
	EarliestTime string `json:"earliest_time"`
}

// Snapshot represents a single backup snapshot
type Snapshot struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	Timestamp    string    `json:"timestamp"` // HH:MM:SS
}

// LocalBackup represents a local backup file
type LocalBackup struct {
	Filename      string    `json:"filename"`
	Size          int64     `json:"size"`
	SizeFormatted string    `json:"size_formatted"`
	ModTime       time.Time `json:"mod_time"`
	ModTimeStr    string    `json:"mod_time_str"`
}

// RestorePreview contains details for restore confirmation
type RestorePreview struct {
	SnapshotKey       string    `json:"snapshot_key"`
	SnapshotTime      time.Time `json:"snapshot_time"`
	Size              int64     `json:"size"`
	SizeFormatted     string    `json:"size_formatted"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresIn         int       `json:"expires_in_seconds"`
	IsLocal           bool      `json:"is_local"`
}

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Success          bool      `json:"success"`
	RestoredAt       time.Time `json:"restored_at"`
	SnapshotKey      string    `json:"snapshot_key"`
	PreRestoreBackup string    `json:"pre_restore_backup"`
	Message          string    `json:"message"`
}

// NewRestoreService creates a new restore service
func NewRestoreService(pool *pgxpool.Pool, connStr string, storage StorageProvider) *RestoreService {
	return &RestoreService{
		pool:            pool,
		connStr:         connStr,
		storage:         storage,
		pendingTokens:   make(map[string]*RestoreToken),
		tokenMu:         sync.RWMutex{},
		restoreCooldown: 5 * time.Minute,
	}
}

// getS3Client creates an S3 client configured for R2
func (s *RestoreService) getS3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to configure S3 client: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	return client, nil
}

// formatBytes converts a byte count into a human-readable format.
func formatBytes(b int64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
	)

	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// ListLocalBackups returns all local backup files
func (s *RestoreService) ListLocalBackups() ([]LocalBackup, error) {
	files, err := s.storage.ListBackups()
	if err != nil {
		return nil, fmt.Errorf("failed to list local backups: %w", err)
	}

	var backups []LocalBackup
	for _, f := range files {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".sql") && !strings.HasSuffix(name, ".dump") && !strings.HasSuffix(name, ".tar") && !strings.HasSuffix(name, ".gz") {
			continue
		}

		backups = append(backups, LocalBackup{
			Filename:      f.Name,
			Size:          f.Size,
			SizeFormatted: formatBytes(f.Size),
			ModTime:       f.ModTime,
			ModTimeStr:    f.ModTime.Format("2006-01-02 15:04:05"),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime.After(backups[j].ModTime)
	})

	return backups, nil
}

// ListAvailableDates returns all dates that have available backups
func (s *RestoreService) ListAvailableDates(ctx context.Context) ([]RestoreDate, int, error) {
	client, err := s.getS3Client(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Collect all objects
	var allObjects []struct {
		Key          string
		LastModified time.Time
	}
	var continuationToken *string

	for {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(config.R2BucketName),
			Prefix:            aws.String("base/"),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list backups: %w", err)
		}

		for _, obj := range result.Contents {
			if obj.Key != nil && obj.LastModified != nil {
				allObjects = append(allObjects, struct {
					Key          string
					LastModified time.Time
				}{*obj.Key, *obj.LastModified})
			}
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Group by date
	dateMap := make(map[string]*RestoreDate)
	dateTimeMap := make(map[string][]time.Time)

	// Parse filename format: cold_db_YYYYMMDD_HHMMSS.sql
	// We rely on the filename for date/time, ignoring the directory structure to be more robust
	keyRegex := regexp.MustCompile(`cold_db_(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})\.sql`)

	for _, obj := range allObjects {
		// Only consider files in "base/" directory to ignore pre-restore backups
		if !strings.HasPrefix(obj.Key, "base/") {
			continue
		}

		matches := keyRegex.FindStringSubmatch(obj.Key)
		if matches == nil {
			continue
		}

		dateStr := fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
		timeStr := fmt.Sprintf("%s:%s:%s", matches[4], matches[5], matches[6])

		if _, exists := dateMap[dateStr]; !exists {
			dateMap[dateStr] = &RestoreDate{
				Date:  dateStr,
				Count: 0,
			}
		}

		dateMap[dateStr].Count++

		// Parse time for earliest/latest calculation
		t, _ := time.Parse("15:04:05", timeStr)
		dateTimeMap[dateStr] = append(dateTimeMap[dateStr], t)
	}

	// Calculate earliest/latest times for each date from R2
	for dateStr, times := range dateTimeMap {
		if len(times) == 0 {
			continue
		}

		sort.Slice(times, func(i, j int) bool {
			return times[i].Before(times[j])
		})

		dateMap[dateStr].EarliestTime = times[0].Format("15:04:05")
		dateMap[dateStr].LatestTime = times[len(times)-1].Format("15:04:05")
	}

	// MERGE LOCAL BACKUPS
	// Also scan local backup directory via storage interface
	localFiles, err := s.storage.ListBackups()
	if err == nil {
		for _, f := range localFiles {
			if !strings.HasSuffix(f.Name, ".sql") {
				continue
			}

			// Parse filename: cold_db_YYYYMMDD_HHMMSS.sql
			matches := keyRegex.FindStringSubmatch(f.Name)
			if matches == nil {
				continue
			}

			dateStr := fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
			// Check if this date already exists from R2
			if _, exists := dateMap[dateStr]; !exists {
				dateMap[dateStr] = &RestoreDate{
					Date:  dateStr,
					Count: 0,
				}
			}

			// We verify if this specific file is already counted
			// Check if this timestamp is already recorded for this date
			// Re-parsing time for local file
			timeStr := fmt.Sprintf("%s:%s:%s", matches[4], matches[5], matches[6])
			t, _ := time.Parse("15:04:05", timeStr)

			isDuplicate := false
			if times, ok := dateTimeMap[dateStr]; ok {
				for _, existingTime := range times {
					if existingTime.Format("15:04:05") == timeStr {
						isDuplicate = true
						break
					}
				}
			}

			if !isDuplicate {
				dateMap[dateStr].Count++
				dateTimeMap[dateStr] = append(dateTimeMap[dateStr], t)
			}
		}
	}

	// Recalculate earliest/latest times for each date (including new local ones)
	for dateStr, times := range dateTimeMap {
		if len(times) == 0 {
			continue
		}
		sort.Slice(times, func(i, j int) bool {
			return times[i].Before(times[j])
		})
		dateMap[dateStr].EarliestTime = times[0].Format("15:04:05")
		dateMap[dateStr].LatestTime = times[len(times)-1].Format("15:04:05")
	}

	// Convert to slice and sort by date (newest first)
	var dates []RestoreDate
	for _, d := range dateMap {
		dates = append(dates, *d)
	}

	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Date > dates[j].Date
	})

	// If we found zero R2 objects, return total based on local
	totalCount := len(allObjects)
	if totalCount == 0 && len(dates) > 0 {
		// Calculate total from local dates
		for _, d := range dates {
			totalCount += d.Count
		}
	}

	return dates, totalCount, nil
}

// ListSnapshotsForDate returns all snapshots for a specific date
func (s *RestoreService) ListSnapshotsForDate(ctx context.Context, date string) ([]Snapshot, error) {
	client, err := s.getS3Client(ctx)
	if err != nil {
		return nil, err
	}

	// Parse date
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	// Build prefix for the date: base/YYYY/MM/DD/
	prefix := fmt.Sprintf("base/%s/", parsedDate.Format("2006/01/02"))

	var snapshots []Snapshot
	var continuationToken *string

	for {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(config.R2BucketName),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list snapshots: %w", err)
		}

		// Parse key format to extract time
		keyRegex := regexp.MustCompile(`cold_db_\d{8}_(\d{2})(\d{2})(\d{2})\.sql`)

		for _, obj := range result.Contents {
			if obj.Key == nil || obj.Size == nil || obj.LastModified == nil {
				continue
			}

			matches := keyRegex.FindStringSubmatch(*obj.Key)
			if matches == nil {
				continue
			}

			timeStr := fmt.Sprintf("%s:%s:%s", matches[1], matches[2], matches[3])

			snapshots = append(snapshots, Snapshot{
				Key:          *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				Timestamp:    timeStr,
			})
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp > snapshots[j].Timestamp
	})

	// MERGE LOCAL BACKUPS
	// Also scan local backup directory via storage interface
	localFiles, err := s.storage.ListBackups()
	if err == nil {
		targetDate := strings.ReplaceAll(date, "-", "") // YYYYMMDD

		for _, f := range localFiles {
			// Filename format: cold_db_YYYYMMDD_HHMMSS.sql
			name := f.Name
			if !strings.HasPrefix(name, "cold_db_"+targetDate) {
				continue
			}

			// Extract time
			matches := regexp.MustCompile(`cold_db_\d{8}_(\d{2})(\d{2})(\d{2})\.sql`).FindStringSubmatch(name)
			if matches == nil {
				continue
			}

			timeStr := fmt.Sprintf("%s:%s:%s", matches[1], matches[2], matches[3])

			// Check for duplicate (if R2 already has it)
			isDuplicate := false
			for _, snap := range snapshots {
				if snap.Timestamp == timeStr {
					isDuplicate = true
					break
				}
			}

			if !isDuplicate {
				// Format size
				snapshots = append(snapshots, Snapshot{
					Key:          name, // Use filename as key for local
					Size:         f.Size,
					LastModified: f.ModTime,
					Timestamp:    timeStr,
				})
			}
		}

		// Sort again after merging
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].Timestamp > snapshots[j].Timestamp
		})
	}

	return snapshots, nil
}

// FindClosestSnapshot finds the snapshot closest to the target time
func (s *RestoreService) FindClosestSnapshot(ctx context.Context, targetTime time.Time) (*Snapshot, error) {
	date := targetTime.Format("2006-01-02")
	snapshots, err := s.ListSnapshotsForDate(ctx, date)
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found for date %s", date)
	}

	targetTimeOnly := targetTime.Format("15:04:05")

	var closest *Snapshot
	var minDiff time.Duration = 24 * time.Hour

	for i := range snapshots {
		snap := &snapshots[i]
		snapTime, _ := time.Parse("15:04:05", snap.Timestamp)
		targetParsed, _ := time.Parse("15:04:05", targetTimeOnly)

		diff := snapTime.Sub(targetParsed)
		if diff < 0 {
			diff = -diff
		}

		if diff < minDiff {
			minDiff = diff
			closest = snap
		}
	}

	return closest, nil
}

// PreviewLocalRestore generates a preview and confirmation token for local restore
func (s *RestoreService) PreviewLocalRestore(filename string, userID int) (*RestorePreview, error) {
	// Verify file exists and get metadata via ListBackups
	files, err := s.storage.ListBackups()
	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	var targetFile *StorageFileInfo
	for _, f := range files {
		if f.Name == filename {
			targetFile = &f
			break
		}
	}

	if targetFile == nil {
		return nil, fmt.Errorf("backup file not found: %s", filename)
	}

	// Generate confirmation token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Store token with 5-minute expiry
	s.tokenMu.Lock()
	s.pendingTokens[token] = &RestoreToken{
		Token:       token,
		SnapshotKey: filename, // Using filename as key for local
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		UserID:      userID,
		IsLocal:     true,
	}
	s.tokenMu.Unlock()

	// Format size
	var sizeFormatted string
	if targetFile.Size < 1024 {
		sizeFormatted = fmt.Sprintf("%d B", targetFile.Size)
	} else if targetFile.Size < 1024*1024 {
		sizeFormatted = fmt.Sprintf("%.2f KB", float64(targetFile.Size)/1024)
	} else {
		sizeFormatted = fmt.Sprintf("%.2f MB", float64(targetFile.Size)/(1024*1024))
	}

	return &RestorePreview{
		SnapshotKey:       filename,
		SnapshotTime:      targetFile.ModTime,
		Size:              targetFile.Size,
		SizeFormatted:     sizeFormatted,
		ConfirmationToken: token,
		ExpiresIn:         300, // 5 minutes
		IsLocal:           true,
	}, nil
}

// ExecuteLocalRestore performs the actual restore operation from local file
func (s *RestoreService) ExecuteLocalRestore(ctx context.Context, filename, confirmationToken string, userID int) (*RestoreResult, error) {
	// Check rate limiting
	if time.Since(s.lastRestoreTime) < s.restoreCooldown {
		remaining := s.restoreCooldown - time.Since(s.lastRestoreTime)
		return nil, fmt.Errorf("rate limited: please wait %v before restoring again", remaining.Round(time.Second))
	}

	// Validate token
	s.tokenMu.Lock()
	token, exists := s.pendingTokens[confirmationToken]
	if exists {
		delete(s.pendingTokens, confirmationToken)
	}
	s.tokenMu.Unlock()

	if !exists {
		return nil, fmt.Errorf("invalid or expired confirmation token")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("confirmation token has expired")
	}

	if token.SnapshotKey != filename {
		return nil, fmt.Errorf("backup filename does not match confirmation token")
	}

	if token.UserID != userID {
		return nil, fmt.Errorf("token was not created by this user")
	}

	if !token.IsLocal {
		return nil, fmt.Errorf("token is for cloud restore, not local")
	}

	// Update last restore time
	s.lastRestoreTime = time.Now()

	log.Printf("[Restore] Starting local restore from %s by user %d", filename, userID)

	// Step 1: Create pre-restore backup
	preRestoreKey, err := s.createPreRestoreBackup(ctx)
	if err != nil {
		log.Printf("[Restore] Warning: failed to create pre-restore backup: %v", err)
	} else {
		log.Printf("[Restore] Created pre-restore backup: %s", preRestoreKey)
	}

	// Step 2: Verify file existence
	// With storage API, we don't check file path prefix, we just try to get it
	// But PREVIEW needs file info without reading full content if possible.
	// Our StorageProvider interface currently only has GetBackup (reads full content).
	// We might need to add StatBackup to interface or just use GetBackup and check error.
	// For now, let's use GetBackup to verify existence (fine for small files, maybe slow for large dumps if we read all).

	// OPTIMIZATION: If we only want to check existence, we should add Stat/Info to interface.
	// But FileManagerHandler.GetBackup reads file.
	// Let's assume for now we just proceed to restore. The restore command needs a file path?
	// WAIT. psql restore command needs a FILE PATH. `exec.Command("psql", ... "-f", filePath)`
	// If `s.storage` is an API (FileManagerHandler), it returns BYTES.
	// `psql` cannot read from bytes in memory via flag easily (needs stdin).

	// We should write the bytes to a TEMP file for `psql` to use.

	log.Println("[Restore] Verifying backup availability...")
	backupData, err := s.storage.GetBackup(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backup: %w", err)
	}

	// Create temp file for psql
	tmpFile := filepath.Join(os.TempDir(), "restore_temp_"+filename)
	if err := os.WriteFile(tmpFile, backupData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp restore file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Step 3: Clean database (drop all tables)
	log.Println("[Restore] Cleaning database...")
	cleanupSQL := `
DO $$
DECLARE
    r RECORD;
BEGIN
    SET session_replication_role = 'replica';
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
    SET session_replication_role = 'origin';
END $$;
`
	cleanCmd := exec.Command("psql", s.connStr, "-c", cleanupSQL)
	cleanOutput, cleanErr := cleanCmd.CombinedOutput()
	if cleanErr != nil {
		log.Printf("[Restore] Warning: cleanup failed: %v - %s", cleanErr, string(cleanOutput))
	}

	// Step 4: Apply schema
	log.Println("[Restore] Creating database schema...")
	schemaSQL, err := migrations.FS.ReadFile("001_complete_schema.sql")
	if err != nil {
		log.Printf("[Restore] Warning: could not read schema file: %v", err)
	} else {
		schemaTmpFile := "/tmp/cold_schema.sql"
		if err := os.WriteFile(schemaTmpFile, schemaSQL, 0644); err != nil {
			log.Printf("[Restore] Warning: could not write schema file: %v", err)
		} else {
			schemaCmd := exec.Command("psql", s.connStr, "-f", schemaTmpFile)
			schemaOutput, schemaErr := schemaCmd.CombinedOutput()
			os.Remove(schemaTmpFile)
			if schemaErr != nil {
				log.Printf("[Restore] Warning: schema creation had issues: %v - %s", schemaErr, string(schemaOutput))
			}
		}
	}

	// Step 5: Restore data from local backup (temp file)
	log.Println("[Restore] Restoring data from local backup...")
	cmd := exec.Command("psql", s.connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("restore failed: %w\nOutput: %s", err, string(output))
	}

	// Check for PostgreSQL errors
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR:") {
		return nil, fmt.Errorf("restore completed with errors:\n%s", outputStr)
	}

	log.Printf("[Restore] Local restore completed successfully")

	return &RestoreResult{
		Success:          true,
		RestoredAt:       time.Now(),
		SnapshotKey:      filename,
		PreRestoreBackup: preRestoreKey,
		Message:          "Database restored successfully from local backup",
	}, nil
}

// PreviewRestore generates a preview and confirmation token for restore
func (s *RestoreService) PreviewRestore(ctx context.Context, snapshotKey string, userID int) (*RestorePreview, error) {
	client, err := s.getS3Client(ctx)
	if err != nil {
		return nil, err
	}

	// Get object metadata
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(snapshotKey),
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}

	// Generate confirmation token
	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Store token with 5-minute expiry
	s.tokenMu.Lock()
	s.pendingTokens[token] = &RestoreToken{
		Token:       token,
		SnapshotKey: snapshotKey,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		UserID:      userID,
		IsLocal:     false,
	}
	s.tokenMu.Unlock()

	// Format size
	var sizeFormatted string
	if *head.ContentLength < 1024 {
		sizeFormatted = fmt.Sprintf("%d B", *head.ContentLength)
	} else if *head.ContentLength < 1024*1024 {
		sizeFormatted = fmt.Sprintf("%.2f KB", float64(*head.ContentLength)/1024)
	} else {
		sizeFormatted = fmt.Sprintf("%.2f MB", float64(*head.ContentLength)/(1024*1024))
	}

	return &RestorePreview{
		SnapshotKey:       snapshotKey,
		SnapshotTime:      *head.LastModified,
		Size:              *head.ContentLength,
		SizeFormatted:     sizeFormatted,
		ConfirmationToken: token,
		ExpiresIn:         300, // 5 minutes
		IsLocal:           false,
	}, nil
}

// ExecuteRestore performs the actual restore operation
func (s *RestoreService) ExecuteRestore(ctx context.Context, snapshotKey, confirmationToken string, userID int) (*RestoreResult, error) {
	// Check rate limiting
	if time.Since(s.lastRestoreTime) < s.restoreCooldown {
		remaining := s.restoreCooldown - time.Since(s.lastRestoreTime)
		return nil, fmt.Errorf("rate limited: please wait %v before restoring again", remaining.Round(time.Second))
	}

	// Validate token
	s.tokenMu.Lock()
	token, exists := s.pendingTokens[confirmationToken]
	if exists {
		delete(s.pendingTokens, confirmationToken)
	}
	s.tokenMu.Unlock()

	if !exists {
		return nil, fmt.Errorf("invalid or expired confirmation token")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("confirmation token has expired")
	}

	if token.SnapshotKey != snapshotKey {
		return nil, fmt.Errorf("snapshot key does not match confirmation token")
	}

	if token.UserID != userID {
		return nil, fmt.Errorf("token was not created by this user")
	}

	if token.IsLocal {
		return nil, fmt.Errorf("token is for local restore, not cloud")
	}

	// Update last restore time
	s.lastRestoreTime = time.Now()

	log.Printf("[Restore] Starting point-in-time restore from %s by user %d", snapshotKey, userID)

	// Step 1: Create pre-restore backup
	preRestoreKey, err := s.createPreRestoreBackup(ctx)
	if err != nil {
		log.Printf("[Restore] Warning: failed to create pre-restore backup: %v", err)
		// Continue anyway - don't block restore
	} else {
		log.Printf("[Restore] Created pre-restore backup: %s", preRestoreKey)
	}

	// Step 2: Download snapshot from R2
	client, err := s.getS3Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(snapshotKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}
	defer resp.Body.Close()

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("cold_restore_%s.sql", time.Now().Format("20060102_150405")))
	f, err := os.Create(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	bytesWritten, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		return nil, fmt.Errorf("failed to save snapshot: %w", err)
	}

	log.Printf("[Restore] Downloaded snapshot: %.2f KB", float64(bytesWritten)/1024)
	defer os.Remove(tmpFile)

	// Step 3: Clean database (drop all tables)
	log.Println("[Restore] Cleaning database...")
	cleanupSQL := `
DO $$
DECLARE
    r RECORD;
BEGIN
    SET session_replication_role = 'replica';
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
    SET session_replication_role = 'origin';
END $$;
`
	cleanCmd := exec.Command("psql", s.connStr, "-c", cleanupSQL)
	cleanOutput, cleanErr := cleanCmd.CombinedOutput()
	if cleanErr != nil {
		log.Printf("[Restore] Warning: cleanup failed: %v - %s", cleanErr, string(cleanOutput))
	}

	// Step 4: Apply schema
	log.Println("[Restore] Creating database schema...")
	schemaSQL, err := migrations.FS.ReadFile("001_complete_schema.sql")
	if err != nil {
		log.Printf("[Restore] Warning: could not read schema file: %v", err)
	} else {
		schemaTmpFile := "/tmp/cold_schema.sql"
		if err := os.WriteFile(schemaTmpFile, schemaSQL, 0644); err != nil {
			log.Printf("[Restore] Warning: could not write schema file: %v", err)
		} else {
			schemaCmd := exec.Command("psql", s.connStr, "-f", schemaTmpFile)
			schemaOutput, schemaErr := schemaCmd.CombinedOutput()
			os.Remove(schemaTmpFile)
			if schemaErr != nil {
				log.Printf("[Restore] Warning: schema creation had issues: %v - %s", schemaErr, string(schemaOutput))
			}
		}
	}

	// Step 5: Restore data from snapshot
	log.Println("[Restore] Restoring data from snapshot...")
	cmd := exec.Command("psql", s.connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil, fmt.Errorf("restore failed: %w\nOutput: %s", err, string(output))
	}

	// Check for PostgreSQL errors
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR:") {
		return nil, fmt.Errorf("restore completed with errors:\n%s", outputStr)
	}

	log.Printf("[Restore] Point-in-time restore completed successfully")

	return &RestoreResult{
		Success:          true,
		RestoredAt:       time.Now(),
		SnapshotKey:      snapshotKey,
		PreRestoreBackup: preRestoreKey,
		Message:          "Database restored successfully",
	}, nil
}

// CreateLocalBackup creates a local backup file only
// Returns: filename, localPath, tempFilePath, error
func (s *RestoreService) CreateLocalBackup(ctx context.Context) (string, string, string, error) {
	// Create backup
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("cold_db_%s.sql", timestamp)
	tmpFile := filepath.Join(os.TempDir(), filename)

	// Try pg_dump first
	// Log the command for debugging (without exposing sensitive info if possible, but connStr is needed)
	log.Printf("[Backup] Attempting pg_dump to %s", tmpFile)
	cmd := exec.Command("pg_dump", s.connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	var data []byte

	if err != nil {
		log.Printf("[Backup] pg_dump failed (%v). Output: %s", err, string(output))
		log.Printf("[Backup] Initiating manual backup fallback...")

		// Fallback to manual backup
		data, err = s.createManualBackup(ctx)
		if err != nil {
			return "", "", "", fmt.Errorf("backup failed completely (pg_dump: %s, manual: %v)", string(output), err)
		}
		log.Printf("[Backup] Manual backup generated successfully (%d bytes)", len(data))

		// Write to temp file for consistency
		if err := os.WriteFile(tmpFile, data, 0644); err != nil {
			return "", "", "", fmt.Errorf("failed to write temp backup file: %w", err)
		}
	} else {
		log.Printf("[Backup] pg_dump succeeded")
		// Read pg_dump output
		data, err = os.ReadFile(tmpFile)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to read backup file: %w", err)
		}
	}

	// We DON'T remove temp file here anymore, caller must do it
	// defer os.Remove(tmpFile)

	// Save to local backup directory via API
	storedPath, err := s.storage.StoreBackup(filename, data)
	if err != nil {
		log.Printf("[Backup] Warning: failed to save local backup via storage API: %v", err)
		// Return error but also return paths so R2 can try (storedPath might be empty)
		return filename, "", tmpFile, fmt.Errorf("failed to save local backup: %w", err)
	} else {
		log.Printf("[Backup] Saved local backup to %s", storedPath)
	}

	return filename, storedPath, tmpFile, nil
}

// CreateBackup creates a new backup (local + R2)
func (s *RestoreService) CreateBackup(ctx context.Context) (string, error) {
	filename, localPath, tmpFile, err := s.CreateLocalBackup(ctx)

	// If we have a temp file, we can proceed with upload even if local save failed
	if tmpFile != "" {
		defer os.Remove(tmpFile)
	}

	// We only error out if we have NO data (both local failed and no temp file returned?)
	// Actually CreateLocalBackup returns error if local save failed.
	// But if tmpFile is set, we can ignore that error for R2 purposes.
	if err != nil && tmpFile == "" {
		return "", err
	}

	// If error exists but we have tmpFile, just log it
	if err != nil {
		log.Printf("[Backup] Local save failed (%v), but proceeding with R2 upload via temp file", err)
	}

	// Upload to R2
	client, err := s.getS3Client(ctx)
	if err != nil {
		log.Printf("[Backup] Warning: failed to configure S3 for backup: %v", err)
		return filename + " (Local Only)", nil
	}

	// Determine which file to read
	var f *os.File
	var fileErr error

	// Try opening local path first
	f, fileErr = os.Open(localPath)
	if fileErr != nil {
		// Fallback to temp file
		log.Printf("[Backup] Could not read local file (%v), using temp file %s", fileErr, tmpFile)
		f, fileErr = os.Open(tmpFile)
		if fileErr != nil {
			log.Printf("[Backup] Warning: failed to open backup for upload: %v", fileErr)
			return filename + " (Local Only)", nil
		}
	}
	defer f.Close()

	fileInfo, _ := f.Stat()

	now := time.Now()
	// Format: base/YYYY/MM/DD/HH/cold_db_...
	key := fmt.Sprintf("base/%s/%s/%s/%s/%s",
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		now.Format("15"), // Hour
		filename)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(config.R2BucketName),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(fileInfo.Size()),
	})
	if err != nil {
		log.Printf("[Backup] Warning: failed to upload backup to R2: %v", err)
		return filename + " (Local Only)", nil
	}

	return key, nil
}

// DeleteLocalBackup deletes a local backup file
func (s *RestoreService) DeleteLocalBackup(filename string) error {
	// Validate filename to prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename")
	}

	if err := s.storage.DeleteBackup(filename); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	log.Printf("[Backup] Deleted local backup: %s", filename)
	return nil
}

// createPreRestoreBackup creates a backup of current state before restore
func (s *RestoreService) createPreRestoreBackup(ctx context.Context) (string, error) {
	// Create backup using pg_dump
	timestamp := time.Now().Format("20060102_150405")
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("cold_prerestore_%s.sql", timestamp))

	cmd := exec.Command("pg_dump", s.connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pg_dump failed: %w\nOutput: %s", err, string(output))
	}
	defer os.Remove(tmpFile)

	// Read the backup file
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to read backup file: %w", err)
	}

	// Save to local backup directory via API
	localFilename := fmt.Sprintf("cold_prerestore_%s.sql", timestamp)
	storedPath, err := s.storage.StoreBackup(localFilename, data)
	if err != nil {
		log.Printf("[Restore] Warning: failed to save local pre-restore backup: %v", err)
	} else {
		log.Printf("[Restore] Saved local pre-restore backup to %s", storedPath)
	}

	// Upload to R2 with special prefix
	client, err := s.getS3Client(ctx)
	if err != nil {
		// Log error but treat success if local saved
		log.Printf("[Restore] Warning: failed to configure S3 for pre-restore backup: %v", err)
		return localFilename + " (Local Only)", nil
	}

	now := time.Now()
	key := fmt.Sprintf("pre-restore/%s/cold_prerestore_%s.sql",
		now.Format("2006/01/02"),
		timestamp)

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(data)),
	})
	if err != nil {
		log.Printf("[Restore] Warning: failed to upload pre-restore backup to R2: %v", err)
		return localFilename + " (Local Only)", nil
	}

	return key, nil
}

// CleanupExpiredTokens removes expired confirmation tokens
func (s *RestoreService) CleanupExpiredTokens() {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	now := time.Now()
	for token, data := range s.pendingTokens {
		if now.After(data.ExpiresAt) {
			delete(s.pendingTokens, token)
		}
	}
}

// createManualBackup creates a SQL dump manually by querying the database
// This is used as a fallback when pg_dump is not available
func (s *RestoreService) createManualBackup(ctx context.Context) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("-- Cold Storage Database Backup (Manual Fallback)\n")
	buffer.WriteString(fmt.Sprintf("-- Generated: %s\n\n", time.Now().Format(time.RFC3339)))

	// Get all tables in public schema
	rows, err := s.pool.Query(ctx, "SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			continue
		}
		// Skip migration table usually
		if table == "schema_migrations" {
			continue
		}
		tables = append(tables, table)
	}

	for _, table := range tables {
		buffer.WriteString(fmt.Sprintf("\n-- Table: %s\n", table))

		// Get columns
		colRows, err := s.pool.Query(ctx, fmt.Sprintf(`
			SELECT column_name, data_type 
			FROM information_schema.columns 
			WHERE table_name = '%s' 
			ORDER BY ordinal_position`, table))
		if err != nil {
			log.Printf("[Backup] Warning: failed to get columns for %s: %v", table, err)
			continue
		}

		var cols []string
		for colRows.Next() {
			var colName, dataType string
			colRows.Scan(&colName, &dataType)
			cols = append(cols, colName)
		}
		colRows.Close()

		if len(cols) == 0 {
			continue
		}

		// Get data
		dataRows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			log.Printf("[Backup] Warning: failed to get data for %s: %v", table, err)
			continue
		}

		for dataRows.Next() {
			// Create a slice of interface{} to hold the row values
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := dataRows.Scan(valuePtrs...); err != nil {
				continue
			}

			buffer.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (", table, strings.Join(cols, ", ")))
			for i, v := range values {
				if i > 0 {
					buffer.WriteString(", ")
				}
				if v == nil {
					buffer.WriteString("NULL")
				} else {
					switch val := v.(type) {
					case []byte:
						buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''")))
					case string:
						buffer.WriteString(fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''")))
					case time.Time:
						// Use microsecond precision for timestamp
						buffer.WriteString(fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05.999999")))
					case bool:
						buffer.WriteString(fmt.Sprintf("%t", val))
					case int, int64, int32, float64, float32:
						buffer.WriteString(fmt.Sprintf("%v", val))
					default:
						// Handle other types as string
						buffer.WriteString(fmt.Sprintf("'%v'", val))
					}
				}
			}
			buffer.WriteString(");\n")
		}
		dataRows.Close()
	}

	return buffer.Bytes(), nil
}
