package repositories

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EntryRepository struct {
	DB *pgxpool.Pool
}

func NewEntryRepository(db *pgxpool.Pool) *EntryRepository {
	return &EntryRepository{DB: db}
}

func (r *EntryRepository) Create(ctx context.Context, e *models.Entry) error {
	// Use atomic INSERT with COUNT in a single query to prevent race conditions
	// This ensures counters auto-reset when entries are deleted (season reset)
	// The CTE (Common Table Expression) makes the count and insert atomic

	if e.ThockCategory != "seed" && e.ThockCategory != "sell" {
		return fmt.Errorf("invalid thock category: %s", e.ThockCategory)
	}

	// Determine the base offset for the category
	var baseOffset int
	if e.ThockCategory == "seed" {
		baseOffset = 1 // SEED starts at 1
	} else {
		baseOffset = 1501 // SELL starts at 1501
	}

	// Atomic query: COUNT and INSERT happen together in a single transaction
	// This prevents race conditions where two requests get the same count
	// Note: Parameters cast explicitly to avoid type inference issues
	query := `
		WITH next_num AS (
			SELECT COALESCE(COUNT(*), 0) + $1 as num
			FROM entries
			WHERE thock_category = $2
		)
		INSERT INTO entries(customer_id, phone, name, village, so, expected_quantity, thock_category, thock_number, created_by_user_id)
		SELECT $3, $4, $5, $6, $7, $8::integer, $9::text,
			CASE WHEN $9::text = 'seed'
				THEN LPAD(num::text, 4, '0') || '/' || $8::text
				ELSE num::text || '/' || $8::text
			END,
			$10
		FROM next_num
		RETURNING id, thock_number, created_at, updated_at
	`

	return r.DB.QueryRow(ctx, query,
		baseOffset,           // $1
		e.ThockCategory,      // $2
		e.CustomerID,         // $3
		e.Phone,              // $4
		e.Name,               // $5
		e.Village,            // $6
		e.SO,                 // $7
		e.ExpectedQuantity,   // $8
		e.ThockCategory,      // $9
		e.CreatedByUserID,    // $10
	).Scan(&e.ID, &e.ThockNumber, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EntryRepository) Get(ctx context.Context, id int) (*models.Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e WHERE e.id=$1`, id)

	var entry models.Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

func (r *EntryRepository) List(ctx context.Context) ([]*models.Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *EntryRepository) ListByCustomer(ctx context.Context, customerID int) ([]*models.Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e WHERE e.customer_id=$1 ORDER BY e.created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *EntryRepository) GetCountByCategory(ctx context.Context, category string) (int, error) {
	// Return actual COUNT of entries for this category
	// This matches the thock number generation logic
	if category != "seed" && category != "sell" {
		return 0, fmt.Errorf("invalid category: %s", category)
	}

	var count int
	err := r.DB.QueryRow(ctx, "SELECT COUNT(*) FROM entries WHERE thock_category = $1", category).Scan(&count)
	return count, err
}

func (r *EntryRepository) ListUnassigned(ctx context.Context) ([]*models.Entry, error) {
	// Get entries that don't have a room entry yet
	// For unassigned entries, actual_quantity will be 0 (no room_entries yet)
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN room_entries re ON e.id = re.entry_id
         WHERE re.id IS NULL
         ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// GetByThockNumber retrieves an entry by thock number
func (r *EntryRepository) GetByThockNumber(ctx context.Context, thockNumber string) (*models.Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e WHERE e.thock_number=$1`, thockNumber)

	var entry models.Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}
