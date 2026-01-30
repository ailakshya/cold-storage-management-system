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
			vehicle_confidence, avg_bag_confidence, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	return r.DB.QueryRow(ctx, query,
		session.GateID, session.StartedAt, session.EndedAt, session.DurationSeconds,
		session.EstimatedTotal, session.UniqueBagCount, session.BagClusterCount, session.PeakBagsInFrame,
		session.VehicleConfidence, session.AvgBagConfidence, session.Status,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)
}

// GetSessionByID retrieves a single detection session.
func (r *DetectionRepository) GetSessionByID(ctx context.Context, id int) (*models.DetectionSession, error) {
	query := `
		SELECT id, gate_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       matched_gate_pass_id, manual_count, count_discrepancy, notes,
		       created_at, updated_at
		FROM detection_sessions WHERE id = $1
	`
	s := &models.DetectionSession{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.GateID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
		&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
		&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
		&s.MatchedGatePassID, &s.ManualCount, &s.CountDiscrepancy, &s.Notes,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
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
		SELECT id, gate_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       matched_gate_pass_id, manual_count, count_discrepancy, notes,
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
			&s.ID, &s.GateID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
			&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
			&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
			&s.MatchedGatePassID, &s.ManualCount, &s.CountDiscrepancy, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	return sessions, total, nil
}

// UpdateSession updates gate pass linkage, manual count, notes, or status.
func (r *DetectionRepository) UpdateSession(ctx context.Context, id int, req *models.UpdateDetectionSessionRequest) error {
	// Calculate discrepancy if manual count is provided
	var discrepancy *int
	if req.ManualCount != nil {
		// Get estimated total first
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
			matched_gate_pass_id = COALESCE($2, matched_gate_pass_id),
			manual_count = COALESCE($3, manual_count),
			count_discrepancy = COALESCE($4, count_discrepancy),
			notes = COALESCE($5, notes),
			status = CASE WHEN $6 = '' THEN status ELSE $6 END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, req.MatchedGatePassID, req.ManualCount, discrepancy, req.Notes, req.Status)
	return err
}

// GetRecentByGate returns the most recent sessions for a gate (for live dashboard).
func (r *DetectionRepository) GetRecentByGate(ctx context.Context, gateID string, limit int) ([]*models.DetectionSession, error) {
	query := `
		SELECT id, gate_id, started_at, ended_at, duration_seconds,
		       estimated_total, unique_bag_count, bag_cluster_count, peak_bags_in_frame,
		       vehicle_confidence, avg_bag_confidence, status,
		       matched_gate_pass_id, manual_count, count_discrepancy, notes,
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
			&s.ID, &s.GateID, &s.StartedAt, &s.EndedAt, &s.DurationSeconds,
			&s.EstimatedTotal, &s.UniqueBagCount, &s.BagClusterCount, &s.PeakBagsInFrame,
			&s.VehicleConfidence, &s.AvgBagConfidence, &s.Status,
			&s.MatchedGatePassID, &s.ManualCount, &s.CountDiscrepancy, &s.Notes,
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
		WHERE started_at >= $1 AND started_at < $2 AND status = 'completed'
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
			"date":               date.Format("2006-01-02"),
			"gate_id":            gateID,
			"session_count":      sessionCount,
			"total_bags_detected": totalDetected,
			"total_bags_manual":  totalManual,
			"avg_confidence":     avgConf,
			"discrepancy_count":  discrepancyCount,
		})
	}
	return results, nil
}
