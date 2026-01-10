package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// SnapshotService handles change-based snapshots
type SnapshotService struct {
	db               *sql.DB
	localSnapshotDir string
	currentSeason    string
}

// NewSnapshotService creates a new snapshot service
func NewSnapshotService(db *sql.DB) *SnapshotService {
	// Determine current season (e.g., "2025-26" for Oct 2025 - Sep 2026)
	now := time.Now()
	year := now.Year()
	if now.Month() < 10 { // Before October, use previous year's season
		year--
	}
	season := fmt.Sprintf("%d-%02d", year, (year+1)%100)

	return &SnapshotService{
		db:               db,
		localSnapshotDir: "/var/lib/cold-storage/snapshots",
		currentSeason:    season,
	}
}

// DBChangeState represents the current state of database changes
type DBChangeState struct {
	LastModified time.Time
	XactID       string
	TotalRows    int64
}

// GetCurrentChangeState returns the current database change state
func (s *SnapshotService) GetCurrentChangeState(ctx context.Context) (*DBChangeState, error) {
	var state DBChangeState
	var lastModified sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(last_modified), pg_current_xact_id()::TEXT, COALESCE(SUM(row_count), 0)
		FROM table_change_tracking
	`).Scan(&lastModified, &state.XactID, &state.TotalRows)

	if err != nil {
		return nil, fmt.Errorf("failed to get change state: %w", err)
	}

	if lastModified.Valid {
		state.LastModified = lastModified.Time
	}

	return &state, nil
}

// GetLastSnapshotTime returns the last snapshot time for the given type
func (s *SnapshotService) GetLastSnapshotTime(ctx context.Context, snapshotType string) (time.Time, error) {
	var lastSnapshot sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(created_at) FROM snapshot_metadata WHERE snapshot_type = $1
	`, snapshotType).Scan(&lastSnapshot)

	if err != nil && err != sql.ErrNoRows {
		return time.Time{}, fmt.Errorf("failed to get last snapshot time: %w", err)
	}

	if lastSnapshot.Valid {
		return lastSnapshot.Time, nil
	}

	return time.Time{}, nil
}

// ShouldSnapshot checks if a new snapshot is needed based on changes
func (s *SnapshotService) ShouldSnapshot(ctx context.Context, snapshotType string) (bool, error) {
	// Get current change state
	state, err := s.GetCurrentChangeState(ctx)
	if err != nil {
		// If we can't check, default to creating snapshot
		log.Printf("[Snapshot] Warning: Could not get change state: %v, proceeding with snapshot", err)
		return true, nil
	}

	// Get last snapshot time
	lastSnapshot, err := s.GetLastSnapshotTime(ctx, snapshotType)
	if err != nil {
		log.Printf("[Snapshot] Warning: Could not get last snapshot time: %v, proceeding with snapshot", err)
		return true, nil
	}

	// If no previous snapshot, create one
	if lastSnapshot.IsZero() {
		log.Printf("[Snapshot] No previous %s snapshot found, creating first snapshot", snapshotType)
		return true, nil
	}

	// If changes occurred after last snapshot, create new one
	if state.LastModified.After(lastSnapshot) {
		log.Printf("[Snapshot] Changes detected (last change: %v, last snapshot: %v), creating snapshot",
			state.LastModified.Format(time.RFC3339), lastSnapshot.Format(time.RFC3339))
		return true, nil
	}

	log.Printf("[Snapshot] No changes since last %s snapshot at %v, skipping",
		snapshotType, lastSnapshot.Format(time.RFC3339))
	return false, nil
}

// RecordSnapshot records a successful snapshot in the metadata table
func (s *SnapshotService) RecordSnapshot(ctx context.Context, snapshotType, snapshotKey string, sizeBytes int64) error {
	state, err := s.GetCurrentChangeState(ctx)
	if err != nil {
		state = &DBChangeState{XactID: "unknown"}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO snapshot_metadata (snapshot_type, snapshot_key, db_version, size_bytes, season)
		VALUES ($1, $2, pg_current_xact_id(), $3, $4)
		ON CONFLICT (snapshot_type, snapshot_key) DO UPDATE SET
			db_version = pg_current_xact_id(),
			size_bytes = EXCLUDED.size_bytes,
			created_at = NOW()
	`, snapshotType, snapshotKey, sizeBytes, s.currentSeason)

	if err != nil {
		return fmt.Errorf("failed to record snapshot: %w", err)
	}

	log.Printf("[Snapshot] Recorded %s snapshot: %s (xact: %s, size: %d bytes, season: %s)",
		snapshotType, snapshotKey, state.XactID, sizeBytes, s.currentSeason)

	return nil
}

// CreateLocalSnapshot creates a local filesystem snapshot
func (s *SnapshotService) CreateLocalSnapshot(ctx context.Context, backupData []byte) (string, error) {
	// Check if snapshot is needed
	shouldCreate, err := s.ShouldSnapshot(ctx, "local")
	if err != nil {
		log.Printf("[Snapshot] Warning checking if snapshot needed: %v", err)
	}
	if !shouldCreate {
		return "", nil // No snapshot needed
	}

	// Create directory structure: /var/lib/cold-storage/snapshots/{season}/YYYY/MM/DD/HH/
	now := time.Now()
	dir := filepath.Join(s.localSnapshotDir, s.currentSeason,
		now.Format("2006"), now.Format("01"), now.Format("02"), now.Format("15"))

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Write snapshot file
	filename := fmt.Sprintf("cold_db_%s.sql", now.Format("20060102_150405"))
	filepath := filepath.Join(dir, filename)

	if err := os.WriteFile(filepath, backupData, 0644); err != nil {
		return "", fmt.Errorf("failed to write snapshot: %w", err)
	}

	// Record snapshot
	if err := s.RecordSnapshot(ctx, "local", filepath, int64(len(backupData))); err != nil {
		log.Printf("[Snapshot] Warning: Failed to record snapshot metadata: %v", err)
	}

	log.Printf("[Snapshot] Created local snapshot: %s (%d bytes)", filepath, len(backupData))
	return filepath, nil
}

// CleanupOldSeasons removes snapshots from seasons older than the previous one
func (s *SnapshotService) CleanupOldSeasons(ctx context.Context) error {
	// Parse current season to get the year
	var currentYear int
	fmt.Sscanf(s.currentSeason, "%d-", &currentYear)

	// Keep current season and previous season
	keepSeasons := []string{
		s.currentSeason,
		fmt.Sprintf("%d-%02d", currentYear-1, currentYear%100),
	}

	log.Printf("[Snapshot] Keeping seasons: %v", keepSeasons)

	// Get seasons to delete
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT season FROM snapshot_metadata
		WHERE season NOT IN ($1, $2) AND snapshot_type = 'local'
	`, keepSeasons[0], keepSeasons[1])
	if err != nil {
		return fmt.Errorf("failed to query old seasons: %w", err)
	}
	defer rows.Close()

	var oldSeasons []string
	for rows.Next() {
		var season string
		if err := rows.Scan(&season); err != nil {
			continue
		}
		oldSeasons = append(oldSeasons, season)
	}

	// Delete old season directories
	for _, season := range oldSeasons {
		seasonDir := filepath.Join(s.localSnapshotDir, season)
		if err := os.RemoveAll(seasonDir); err != nil {
			log.Printf("[Snapshot] Warning: Failed to remove old season %s: %v", season, err)
		} else {
			log.Printf("[Snapshot] Removed old season snapshots: %s", season)
		}

		// Remove from metadata
		s.db.ExecContext(ctx, `DELETE FROM snapshot_metadata WHERE season = $1 AND snapshot_type = 'local'`, season)
	}

	return nil
}

// GetSnapshotStats returns statistics about snapshots
func (s *SnapshotService) GetSnapshotStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count by type
	rows, err := s.db.QueryContext(ctx, `
		SELECT snapshot_type, COUNT(*), COALESCE(SUM(size_bytes), 0), MAX(created_at)
		FROM snapshot_metadata
		GROUP BY snapshot_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	typeStats := make(map[string]map[string]interface{})
	for rows.Next() {
		var snapshotType string
		var count int64
		var totalSize int64
		var lastCreated sql.NullTime

		if err := rows.Scan(&snapshotType, &count, &totalSize, &lastCreated); err != nil {
			continue
		}

		typeStats[snapshotType] = map[string]interface{}{
			"count":      count,
			"total_size": totalSize,
			"last":       lastCreated.Time,
		}
	}
	stats["by_type"] = typeStats

	// Current season stats
	var seasonCount int64
	var seasonSize int64
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM snapshot_metadata WHERE season = $1
	`, s.currentSeason).Scan(&seasonCount, &seasonSize)

	stats["current_season"] = map[string]interface{}{
		"name":       s.currentSeason,
		"count":      seasonCount,
		"total_size": seasonSize,
	}

	// Change state
	state, _ := s.GetCurrentChangeState(ctx)
	if state != nil {
		stats["db_state"] = map[string]interface{}{
			"last_modified": state.LastModified,
			"xact_id":       state.XactID,
			"total_rows":    state.TotalRows,
		}
	}

	return stats, nil
}

// GetLocalSnapshotDir returns the configured local snapshot directory
func (s *SnapshotService) GetLocalSnapshotDir() string {
	return s.localSnapshotDir
}

// SetLocalSnapshotDir sets the local snapshot directory
func (s *SnapshotService) SetLocalSnapshotDir(dir string) {
	s.localSnapshotDir = dir
}

// GetCurrentSeason returns the current season string
func (s *SnapshotService) GetCurrentSeason() string {
	return s.currentSeason
}

// InitializeChangeTracking initializes the change tracking tables
func (s *SnapshotService) InitializeChangeTracking(ctx context.Context) error {
	// Update row counts for tracked tables
	tables := []string{"entries", "room_entries", "customers", "gate_passes", "rent_payments", "ledger_entries"}

	for _, table := range tables {
		query := fmt.Sprintf(`
			INSERT INTO table_change_tracking (table_name, last_modified, row_count)
			VALUES ($1, NOW(), (SELECT COUNT(*) FROM %s))
			ON CONFLICT (table_name) DO UPDATE SET
				row_count = (SELECT COUNT(*) FROM %s)
		`, table, table)

		if _, err := s.db.ExecContext(ctx, query, table); err != nil {
			log.Printf("[Snapshot] Warning: Failed to initialize tracking for %s: %v", table, err)
		}
	}

	log.Printf("[Snapshot] Initialized change tracking for %d tables", len(tables))
	return nil
}
