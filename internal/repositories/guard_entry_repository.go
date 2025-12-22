package repositories

import (
	"context"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GuardEntryRepository struct {
	DB *pgxpool.Pool
}

func NewGuardEntryRepository(db *pgxpool.Pool) *GuardEntryRepository {
	return &GuardEntryRepository{DB: db}
}

// Create creates a new guard entry
func (r *GuardEntryRepository) Create(ctx context.Context, entry *models.GuardEntry) error {
	query := `
		INSERT INTO guard_entries (customer_name, village, mobile, category, remarks, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, arrival_time, status, created_at, updated_at
	`
	return r.DB.QueryRow(ctx, query,
		entry.CustomerName,
		entry.Village,
		entry.Mobile,
		entry.Category,
		entry.Remarks,
		entry.CreatedByUserID,
	).Scan(&entry.ID, &entry.ArrivalTime, &entry.Status, &entry.CreatedAt, &entry.UpdatedAt)
}

// Get retrieves a guard entry by ID with user names
func (r *GuardEntryRepository) Get(ctx context.Context, id int) (*models.GuardEntry, error) {
	query := `
		SELECT g.id, g.customer_name, g.village, g.mobile, g.arrival_time,
		       g.category, COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at,
		       u1.name as created_by_name,
		       COALESCE(u2.name, '') as processed_by_name
		FROM guard_entries g
		LEFT JOIN users u1 ON g.created_by_user_id = u1.id
		LEFT JOIN users u2 ON g.processed_by_user_id = u2.id
		WHERE g.id = $1
	`
	var entry models.GuardEntry
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&entry.ID, &entry.CustomerName, &entry.Village, &entry.Mobile, &entry.ArrivalTime,
		&entry.Category, &entry.Remarks, &entry.Status,
		&entry.CreatedByUserID, &entry.ProcessedByUserID, &entry.ProcessedAt,
		&entry.CreatedAt, &entry.UpdatedAt,
		&entry.CreatedByUserName, &entry.ProcessedByUserName,
	)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// ListTodayByUser returns today's entries created by a specific user
func (r *GuardEntryRepository) ListTodayByUser(ctx context.Context, userID int) ([]*models.GuardEntry, error) {
	query := `
		SELECT g.id, g.customer_name, g.village, g.mobile, g.arrival_time,
		       g.category, COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at
		FROM guard_entries g
		WHERE g.created_by_user_id = $1
		  AND DATE(g.created_at) = CURRENT_DATE
		ORDER BY g.created_at DESC
	`
	rows, err := r.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.GuardEntry
	for rows.Next() {
		var entry models.GuardEntry
		err := rows.Scan(
			&entry.ID, &entry.CustomerName, &entry.Village, &entry.Mobile, &entry.ArrivalTime,
			&entry.Category, &entry.Remarks, &entry.Status,
			&entry.CreatedByUserID, &entry.ProcessedByUserID, &entry.ProcessedAt,
			&entry.CreatedAt, &entry.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// ListPending returns all pending guard entries with guard names (for entry room)
func (r *GuardEntryRepository) ListPending(ctx context.Context) ([]*models.GuardEntry, error) {
	query := `
		SELECT g.id, g.customer_name, g.village, g.mobile, g.arrival_time,
		       g.category, COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at,
		       u.name as created_by_name
		FROM guard_entries g
		LEFT JOIN users u ON g.created_by_user_id = u.id
		WHERE g.status = 'pending'
		ORDER BY g.arrival_time ASC
	`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.GuardEntry
	for rows.Next() {
		var entry models.GuardEntry
		err := rows.Scan(
			&entry.ID, &entry.CustomerName, &entry.Village, &entry.Mobile, &entry.ArrivalTime,
			&entry.Category, &entry.Remarks, &entry.Status,
			&entry.CreatedByUserID, &entry.ProcessedByUserID, &entry.ProcessedAt,
			&entry.CreatedAt, &entry.UpdatedAt,
			&entry.CreatedByUserName,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// MarkAsProcessed marks a guard entry as processed
func (r *GuardEntryRepository) MarkAsProcessed(ctx context.Context, id int, processedByUserID int) error {
	query := `
		UPDATE guard_entries
		SET status = 'processed',
		    processed_by_user_id = $2,
		    processed_at = $3,
		    updated_at = $3
		WHERE id = $1
	`
	now := time.Now()
	_, err := r.DB.Exec(ctx, query, id, processedByUserID, now)
	return err
}

// GetTodayCountByUser returns count of today's entries for a specific user
func (r *GuardEntryRepository) GetTodayCountByUser(ctx context.Context, userID int) (int, int, error) {
	// Returns (total, pending) counts
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'pending') as pending
		FROM guard_entries
		WHERE created_by_user_id = $1
		  AND DATE(created_at) = CURRENT_DATE
	`
	var total, pending int
	err := r.DB.QueryRow(ctx, query, userID).Scan(&total, &pending)
	return total, pending, err
}
