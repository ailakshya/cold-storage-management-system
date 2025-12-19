package repositories

import (
	"context"
	"encoding/json"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SeasonRequestRepository handles season request database operations
type SeasonRequestRepository struct {
	pool *pgxpool.Pool
}

// NewSeasonRequestRepository creates a new season request repository
func NewSeasonRequestRepository(pool *pgxpool.Pool) *SeasonRequestRepository {
	return &SeasonRequestRepository{pool: pool}
}

// Create creates a new season request
func (r *SeasonRequestRepository) Create(ctx context.Context, req *models.SeasonRequest) (*models.SeasonRequest, error) {
	query := `
		INSERT INTO season_requests (status, initiated_by_user_id, season_name, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, status, initiated_by_user_id, initiated_at, season_name, notes, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		"pending",
		req.InitiatedByUserID,
		req.SeasonName,
		req.Notes,
	).Scan(
		&req.ID,
		&req.Status,
		&req.InitiatedByUserID,
		&req.InitiatedAt,
		&req.SeasonName,
		&req.Notes,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return req, nil
}

// GetByID retrieves a season request by ID
func (r *SeasonRequestRepository) GetByID(ctx context.Context, id int) (*models.SeasonRequest, error) {
	query := `
		SELECT
			sr.id, sr.status, sr.initiated_by_user_id, sr.initiated_at,
			sr.approved_by_user_id, sr.approved_at,
			COALESCE(sr.archive_location, ''), sr.records_archived, COALESCE(sr.error_message, ''),
			COALESCE(sr.season_name, ''), COALESCE(sr.notes, ''), sr.created_at, sr.updated_at,
			COALESCE(u1.name, '') as initiated_by_name,
			COALESCE(u2.name, '') as approved_by_name
		FROM season_requests sr
		LEFT JOIN users u1 ON sr.initiated_by_user_id = u1.id
		LEFT JOIN users u2 ON sr.approved_by_user_id = u2.id
		WHERE sr.id = $1
	`

	var req models.SeasonRequest
	var recordsArchived []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&req.ID,
		&req.Status,
		&req.InitiatedByUserID,
		&req.InitiatedAt,
		&req.ApprovedByUserID,
		&req.ApprovedAt,
		&req.ArchiveLocation,
		&recordsArchived,
		&req.ErrorMessage,
		&req.SeasonName,
		&req.Notes,
		&req.CreatedAt,
		&req.UpdatedAt,
		&req.InitiatedByName,
		&req.ApprovedByName,
	)

	if err != nil {
		return nil, err
	}

	if recordsArchived != nil {
		raw := json.RawMessage(recordsArchived)
		req.RecordsArchived = &raw
	}

	return &req, nil
}

// GetPending retrieves all pending season requests
func (r *SeasonRequestRepository) GetPending(ctx context.Context) ([]*models.SeasonRequest, error) {
	query := `
		SELECT
			sr.id, sr.status, sr.initiated_by_user_id, sr.initiated_at,
			sr.season_name, sr.notes, sr.created_at, sr.updated_at,
			u.name as initiated_by_name
		FROM season_requests sr
		LEFT JOIN users u ON sr.initiated_by_user_id = u.id
		WHERE sr.status = 'pending'
		ORDER BY sr.initiated_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*models.SeasonRequest
	for rows.Next() {
		var req models.SeasonRequest
		err := rows.Scan(
			&req.ID,
			&req.Status,
			&req.InitiatedByUserID,
			&req.InitiatedAt,
			&req.SeasonName,
			&req.Notes,
			&req.CreatedAt,
			&req.UpdatedAt,
			&req.InitiatedByName,
		)
		if err != nil {
			return nil, err
		}
		requests = append(requests, &req)
	}

	return requests, nil
}

// GetAll retrieves all season requests (history)
func (r *SeasonRequestRepository) GetAll(ctx context.Context) ([]*models.SeasonRequest, error) {
	query := `
		SELECT
			sr.id, sr.status, sr.initiated_by_user_id, sr.initiated_at,
			sr.approved_by_user_id, sr.approved_at,
			COALESCE(sr.archive_location, ''), sr.records_archived, COALESCE(sr.error_message, ''),
			COALESCE(sr.season_name, ''), COALESCE(sr.notes, ''), sr.created_at, sr.updated_at,
			COALESCE(u1.name, '') as initiated_by_name,
			COALESCE(u2.name, '') as approved_by_name
		FROM season_requests sr
		LEFT JOIN users u1 ON sr.initiated_by_user_id = u1.id
		LEFT JOIN users u2 ON sr.approved_by_user_id = u2.id
		ORDER BY sr.initiated_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*models.SeasonRequest
	for rows.Next() {
		var req models.SeasonRequest
		var recordsArchived []byte

		err := rows.Scan(
			&req.ID,
			&req.Status,
			&req.InitiatedByUserID,
			&req.InitiatedAt,
			&req.ApprovedByUserID,
			&req.ApprovedAt,
			&req.ArchiveLocation,
			&recordsArchived,
			&req.ErrorMessage,
			&req.SeasonName,
			&req.Notes,
			&req.CreatedAt,
			&req.UpdatedAt,
			&req.InitiatedByName,
			&req.ApprovedByName,
		)
		if err != nil {
			return nil, err
		}

		if recordsArchived != nil {
			raw := json.RawMessage(recordsArchived)
			req.RecordsArchived = &raw
		}

		requests = append(requests, &req)
	}

	return requests, nil
}

// UpdateStatus updates the status of a season request
func (r *SeasonRequestRepository) UpdateStatus(ctx context.Context, id int, status string, approvedByUserID *int) error {
	// Use separate queries to avoid parameter type inference issues
	if approvedByUserID != nil {
		query := `
			UPDATE season_requests
			SET status = $1,
				approved_by_user_id = $2,
				approved_at = CURRENT_TIMESTAMP,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`
		_, err := r.pool.Exec(ctx, query, status, *approvedByUserID, id)
		return err
	}

	query := `
		UPDATE season_requests
		SET status = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, status, id)
	return err
}

// UpdateCompletion updates the completion details of a season request
func (r *SeasonRequestRepository) UpdateCompletion(ctx context.Context, id int, status string, archiveLocation string, recordsArchived *models.RecordsArchivedSummary, errorMsg string) error {
	var recordsJSON []byte
	var err error
	if recordsArchived != nil {
		recordsJSON, err = json.Marshal(recordsArchived)
		if err != nil {
			return err
		}
	}

	query := `
		UPDATE season_requests
		SET status = $1,
			archive_location = $2,
			records_archived = $3,
			error_message = $4,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $5
	`

	_, err = r.pool.Exec(ctx, query, status, archiveLocation, recordsJSON, errorMsg, id)
	return err
}

// RejectRequest rejects a season request
func (r *SeasonRequestRepository) RejectRequest(ctx context.Context, id int, rejectedByUserID int, reason string) error {
	query := `
		UPDATE season_requests
		SET status = 'rejected',
			approved_by_user_id = $1,
			approved_at = CURRENT_TIMESTAMP,
			error_message = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`

	_, err := r.pool.Exec(ctx, query, rejectedByUserID, reason, id)
	return err
}
