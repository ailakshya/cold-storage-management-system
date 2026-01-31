package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cold-backend/internal/models"
)

type DetectionRepository struct {
	DB *pgxpool.Pool
}

func NewDetectionRepository(db *pgxpool.Pool) *DetectionRepository {
	return &DetectionRepository{DB: db}
}

// CreateSession stores a completed detection session from the Python service.
func (r *DetectionRepository) CreateSession(ctx context.Context, session *models.DetectionSession) error {
	query := `
		INSERT INTO detection_sessions (
			gate_id, started_at, ended_at, duration_seconds,
			estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
			vehicle_confidence, avg_bag_confidence, status, video_path, video_size_bytes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`
	return r.DB.QueryRow(ctx, query,
		session.GateID, session.StartedAt, session.EndedAt, session.DurationSeconds,
		session.EstimatedTotal, session.UniqueBagCount, session.BagClusterCount, session.PeakBagsInFrame,
		session.VehicleConfidence, session.AvgBagConfidence, session.Status,
		session.VideoPath, session.VideoSizeBytes,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)
}

// GetSessionByID retrieves a single detection session with linked room entries.
func (r *DetectionRepository) GetSessionByID(ctx context.Context, id int) (*models.DetectionSession, error) {
	query := `
		SELECT id, gate_id, guard_entry_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       manual_count, count_discrepancy, video_path, video_size_bytes, notes,
		       created_at, updated_at
		FROM detection_sessions WHERE id = $1
	`
	s := &models.DetectionSession{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.GateID, &s.GuardEntryID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
		&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
		&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
		&s.ManualCount, &s.CountDiscrepancy, &s.VideoPath, &s.VideoSizeBytes, &s.Notes,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Load linked room entries
	s.LinkedRoomEntries, _ = r.GetLinkedRoomEntries(ctx, id)

	return s, nil
}

// ListSessions returns detection sessions with optional filtering.
func (r *DetectionRepository) ListSessions(ctx context.Context, gateID string, status string, limit, offset int) ([]*models.DetectionSession, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if gateID != "" {
		where += fmt.Sprintf(" AND gate_id = $%d", argIdx)
		args = append(args, gateID)
		argIdx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM detection_sessions " + where
	var total int
	err := r.DB.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Fetch rows
	dataQuery := fmt.Sprintf(`
		SELECT id, gate_id, guard_entry_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       manual_count, count_discrepancy, video_path, video_size_bytes, notes,
		       created_at, updated_at
		FROM detection_sessions %s
		ORDER BY started_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.DB.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	sessions := []*models.DetectionSession{}
	for rows.Next() {
		s := &models.DetectionSession{}
		if err := rows.Scan(
			&s.ID, &s.GateID, &s.GuardEntryID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
			&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
			&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
			&s.ManualCount, &s.CountDiscrepancy, &s.VideoPath, &s.VideoSizeBytes, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	return sessions, total, nil
}

// UpdateSession updates guard entry linkage, manual count, notes, or status.
func (r *DetectionRepository) UpdateSession(ctx context.Context, id int, req *models.UpdateDetectionSessionRequest) error {
	// Calculate discrepancy if manual count is provided
	var discrepancy *int
	if req.ManualCount != nil {
		var estimated int
		err := r.DB.QueryRow(ctx, "SELECT estimated_total FROM detection_sessions WHERE id = $1", id).Scan(&estimated)
		if err != nil {
			return err
		}
		d := *req.ManualCount - estimated
		discrepancy = &d
	}

	query := `
		UPDATE detection_sessions SET
			guard_entry_id = COALESCE($2, guard_entry_id),
			manual_count = COALESCE($3, manual_count),
			count_discrepancy = COALESCE($4, count_discrepancy),
			notes = COALESCE($5, notes),
			status = CASE WHEN $6 = '' THEN status ELSE $6 END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, req.GuardEntryID, req.ManualCount, discrepancy, req.Notes, req.Status)
	return err
}

// LinkRoomEntry links a detection session to a room entry (thock).
func (r *DetectionRepository) LinkRoomEntry(ctx context.Context, sessionID int, req *models.LinkRoomEntryRequest, userID int) (*models.DetectionRoomEntry, error) {
	query := `
		INSERT INTO detection_room_entries (session_id, room_entry_id, bag_count_for_entry, linked_by_user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id, room_entry_id) DO UPDATE SET
			bag_count_for_entry = COALESCE(EXCLUDED.bag_count_for_entry, detection_room_entries.bag_count_for_entry),
			linked_at = CURRENT_TIMESTAMP
		RETURNING id, linked_at
	`
	dre := &models.DetectionRoomEntry{
		SessionID:       sessionID,
		RoomEntryID:     req.RoomEntryID,
		BagCountForEntry: req.BagCountForEntry,
		LinkedByUserID:  &userID,
	}
	err := r.DB.QueryRow(ctx, query, sessionID, req.RoomEntryID, req.BagCountForEntry, userID).
		Scan(&dre.ID, &dre.LinkedAt)
	if err != nil {
		return nil, err
	}
	return dre, nil
}

// UnlinkRoomEntry removes a link between session and room entry.
func (r *DetectionRepository) UnlinkRoomEntry(ctx context.Context, sessionID, roomEntryID int) error {
	_, err := r.DB.Exec(ctx,
		"DELETE FROM detection_room_entries WHERE session_id = $1 AND room_entry_id = $2",
		sessionID, roomEntryID,
	)
	return err
}

// GetLinkedRoomEntries returns all room entries linked to a session, with thock info.
func (r *DetectionRepository) GetLinkedRoomEntries(ctx context.Context, sessionID int) ([]models.DetectionRoomEntry, error) {
	query := `
		SELECT dre.id, dre.session_id, dre.room_entry_id, dre.bag_count_for_entry,
		       dre.linked_by_user_id, dre.linked_at,
		       re.thock_number, c.name, re.room_no
		FROM detection_room_entries dre
		JOIN room_entries re ON re.id = dre.room_entry_id
		JOIN entries e ON e.id = re.entry_id
		JOIN customers c ON c.id = e.customer_id
		WHERE dre.session_id = $1
		ORDER BY dre.linked_at
	`
	rows, err := r.DB.Query(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.DetectionRoomEntry
	for rows.Next() {
		var dre models.DetectionRoomEntry
		if err := rows.Scan(
			&dre.ID, &dre.SessionID, &dre.RoomEntryID, &dre.BagCountForEntry,
			&dre.LinkedByUserID, &dre.LinkedAt,
			&dre.ThockNumber, &dre.CustomerName, &dre.RoomNo,
		); err != nil {
			return nil, err
		}
		entries = append(entries, dre)
	}
	return entries, nil
}

// GetSessionsByRoomEntry finds all detection sessions linked to a specific room entry (thock).
// Used by the item locator to show detection media for a thock.
func (r *DetectionRepository) GetSessionsByRoomEntry(ctx context.Context, roomEntryID int) ([]*models.DetectionSession, error) {
	query := `
		SELECT ds.id, ds.gate_id, ds.guard_entry_id, ds.started_at, ds.ended_at, ds.duration_seconds,
		       ds.estimated_total, ds.unique_bag_count, ds.bag_cluster_count, ds.peak_bags_in_frame,
		       ds.vehicle_confidence, ds.avg_bag_confidence, ds.status,
		       ds.manual_count, ds.count_discrepancy, ds.video_path, ds.video_size_bytes, ds.notes,
		       ds.created_at, ds.updated_at
		FROM detection_sessions ds
		JOIN detection_room_entries dre ON dre.session_id = ds.id
		WHERE dre.room_entry_id = $1
		ORDER BY ds.started_at DESC
	`
	rows, err := r.DB.Query(ctx, query, roomEntryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.DetectionSession
	for rows.Next() {
		s := &models.DetectionSession{}
		if err := rows.Scan(
			&s.ID, &s.GateID, &s.GuardEntryID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
			&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
			&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
			&s.ManualCount, &s.CountDiscrepancy, &s.VideoPath, &s.VideoSizeBytes, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetRecentByGate returns the most recent sessions for a gate.
func (r *DetectionRepository) GetRecentByGate(ctx context.Context, gateID string, limit int) ([]*models.DetectionSession, error) {
	query := `
		SELECT id, gate_id, guard_entry_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       manual_count, count_discrepancy, video_path, video_size_bytes, notes,
		       created_at, updated_at
		FROM detection_sessions
		WHERE gate_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`
	rows, err := r.DB.Query(ctx, query, gateID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []*models.DetectionSession{}
	for rows.Next() {
		s := &models.DetectionSession{}
		if err := rows.Scan(
			&s.ID, &s.GateID, &s.GuardEntryID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
			&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
			&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
			&s.ManualCount, &s.CountDiscrepancy, &s.VideoPath, &s.VideoSizeBytes, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetDailySummary returns aggregated detection stats for a date range.
func (r *DetectionRepository) GetDailySummary(ctx context.Context, from, to time.Time) ([]map[string]interface{}, error) {
	query := `
		SELECT
			DATE(started_at) as date,
			gate_id,
			COUNT(*) as session_count,
			SUM(estimated_total) as total_bags_detected,
			SUM(manual_count) as total_bags_manual,
			AVG(avg_bag_confidence) as avg_confidence,
			SUM(CASE WHEN count_discrepancy IS NOT NULL AND ABS(count_discrepancy) > (estimated_total * 0.1) THEN 1 ELSE 0 END) as discrepancy_count
		FROM detection_sessions
		WHERE started_at >= $1 AND started_at < $2 AND status IN ('completed', 'verified')
		GROUP BY DATE(started_at), gate_id
		ORDER BY date DESC, gate_id
	`
	rows, err := r.DB.Query(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []map[string]interface{}{}
	for rows.Next() {
		var date time.Time
		var gateID string
		var sessionCount int
		var totalDetected, totalManual *int
		var avgConf *float64
		var discrepancyCount int

		if err := rows.Scan(&date, &gateID, &sessionCount, &totalDetected, &totalManual, &avgConf, &discrepancyCount); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"date":                date.Format("2006-01-02"),
			"gate_id":             gateID,
			"session_count":       sessionCount,
			"total_bags_detected": totalDetected,
			"total_bags_manual":   totalManual,
			"avg_confidence":      avgConf,
			"discrepancy_count":   discrepancyCount,
		})
	}
	return results, nil
}
