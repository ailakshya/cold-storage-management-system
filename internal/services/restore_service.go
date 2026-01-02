package services

import (
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

	"cold-backend/internal/config"
	"cold-backend/migrations"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RestoreService handles point-in-time database restoration from R2
type RestoreService struct {
	pool             *pgxpool.Pool
	connStr          string
	pendingTokens    map[string]*RestoreToken
	tokenMu          sync.RWMutex
	lastRestoreTime  time.Time
	restoreCooldown  time.Duration
}

// RestoreToken holds confirmation token for restore operation
type RestoreToken struct {
	Token       string
	SnapshotKey string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	UserID      int
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

// RestorePreview contains details for restore confirmation
type RestorePreview struct {
	SnapshotKey       string    `json:"snapshot_key"`
	SnapshotTime      time.Time `json:"snapshot_time"`
	Size              int64     `json:"size"`
	SizeFormatted     string    `json:"size_formatted"`
	ConfirmationToken string    `json:"confirmation_token"`
	ExpiresIn         int       `json:"expires_in_seconds"`
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
func NewRestoreService(pool *pgxpool.Pool, connStr string) *RestoreService {
	return &RestoreService{
		pool:            pool,
		connStr:         connStr,
		pendingTokens:   make(map[string]*RestoreToken),
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

	// Parse key format: base/YYYY/MM/DD/HH/cold_db_YYYYMMDD_HHMMSS.sql
	keyRegex := regexp.MustCompile(`base/(\d{4})/(\d{2})/(\d{2})/\d{2}/cold_db_\d{8}_(\d{2})(\d{2})(\d{2})\.sql`)

	for _, obj := range allObjects {
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

	// Calculate earliest/latest times for each date
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

	return dates, len(allObjects), nil
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

// createPreRestoreBackup creates a backup of current state before restore
func (s *RestoreService) createPreRestoreBackup(ctx context.Context) (string, error) {
	client, err := s.getS3Client(ctx)
	if err != nil {
		return "", err
	}

	// Create backup using pg_dump
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("cold_prerestore_%s.sql", time.Now().Format("20060102_150405")))

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

	// Upload to R2 with special prefix
	now := time.Now()
	key := fmt.Sprintf("pre-restore/%s/cold_prerestore_%s.sql",
		now.Format("2006/01/02"),
		now.Format("20060102_150405"))

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(key),
		Body:   strings.NewReader(string(data)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload pre-restore backup: %w", err)
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
