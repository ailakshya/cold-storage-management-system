package repositories

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SkipRange represents a range of thock numbers to skip
type SkipRange struct {
	From int
	To   int
}

type EntryRepository struct {
	DB *pgxpool.Pool
}

func NewEntryRepository(db *pgxpool.Pool) *EntryRepository {
	return &EntryRepository{DB: db}
}

func (r *EntryRepository) Create(ctx context.Context, e *models.Entry) error {
	// Delegate to CreateWithSkipRanges with no skip ranges
	return r.CreateWithSkipRanges(ctx, e, nil)
}

// CreateWithSkipRanges creates an entry with thock number that skips specified ranges
func (r *EntryRepository) CreateWithSkipRanges(ctx context.Context, e *models.Entry, skipRanges []SkipRange) error {
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

	// If no skip ranges, use the original simple query
	if len(skipRanges) == 0 {
		query := `
			WITH next_num AS (
				SELECT COALESCE(COUNT(*), 0) + $1 as num
				FROM entries
				WHERE thock_category = $2
			)
			INSERT INTO entries(customer_id, phone, name, village, so, expected_quantity, thock_category, thock_number, remark, created_by_user_id)
			SELECT $3, $4, $5, $6, $7, $8::integer, $9::text,
				CASE WHEN $9::text = 'seed'
					THEN LPAD(num::text, 4, '0') || '/' || $8::text
					ELSE num::text || '/' || $8::text
				END,
				$10,
				$11
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
			e.Remark,             // $10
			e.CreatedByUserID,    // $11
		).Scan(&e.ID, &e.ThockNumber, &e.CreatedAt, &e.UpdatedAt)
	}

	// With skip ranges, use MAX thock number to find the highest used number
	// This correctly handles entries created after skip ranges
	var maxThock int
	query := `
		SELECT COALESCE(MAX(
			CAST(SPLIT_PART(thock_number, '/', 1) AS INTEGER)
		), $1 - 1)
		FROM entries
		WHERE thock_category = $2
	`
	err := r.DB.QueryRow(ctx, query, baseOffset, e.ThockCategory).Scan(&maxThock)
	if err != nil {
		return fmt.Errorf("failed to get max thock number: %w", err)
	}

	// Next number is max + 1
	nextNum := maxThock + 1

	// Apply skip ranges - if next number is in a skip range, jump past it
	for {
		inSkipRange := false
		for _, sr := range skipRanges {
			if nextNum >= sr.From && nextNum <= sr.To {
				nextNum = sr.To + 1
				inSkipRange = true
				break
			}
		}
		if !inSkipRange {
			break
		}
	}

	// Format the thock number
	var thockNumber string
	if e.ThockCategory == "seed" {
		thockNumber = fmt.Sprintf("%04d/%d", nextNum, e.ExpectedQuantity)
	} else {
		thockNumber = fmt.Sprintf("%d/%d", nextNum, e.ExpectedQuantity)
	}

	// Insert with the calculated thock number
	insertQuery := `
		INSERT INTO entries(customer_id, phone, name, village, so, expected_quantity, thock_category, thock_number, remark, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, thock_number, created_at, updated_at
	`

	return r.DB.QueryRow(ctx, insertQuery,
		e.CustomerID,         // $1
		e.Phone,              // $2
		e.Name,               // $3
		e.Village,            // $4
		e.SO,                 // $5
		e.ExpectedQuantity,   // $6
		e.ThockCategory,      // $7
		thockNumber,          // $8
		e.Remark,             // $9
		e.CreatedByUserID,    // $10
	).Scan(&e.ID, &e.ThockNumber, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EntryRepository) Get(ctx context.Context, id int) (*models.Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE((SELECT SUM(quantity) FROM room_entries WHERE entry_id = e.id), 0) as actual_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e WHERE e.id=$1`, id)

	var entry models.Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

func (r *EntryRepository) List(ctx context.Context) ([]*models.Entry, error) {
	// OPTIMIZED: Use LEFT JOIN with subquery aggregate instead of N+1 subqueries
	// Before: 500 entries = 500 subqueries
	// After: Single query with JOIN aggregate
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE(rq.total_qty, 0) as actual_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark,
		        e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN (
             SELECT entry_id, SUM(quantity) as total_qty
             FROM room_entries
             GROUP BY entry_id
         ) rq ON e.id = rq.entry_id
         ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *EntryRepository) ListByCustomer(ctx context.Context, customerID int) ([]*models.Entry, error) {
	// OPTIMIZED: Use LEFT JOIN with subquery aggregate
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE(rq.total_qty, 0) as actual_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark,
		        e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN (
             SELECT entry_id, SUM(quantity) as total_qty
             FROM room_entries
             GROUP BY entry_id
         ) rq ON e.id = rq.entry_id
         WHERE e.customer_id=$1
         ORDER BY e.created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// ListSince returns entries created after the given timestamp (for delta refresh)
func (r *EntryRepository) ListSince(ctx context.Context, since string) ([]*models.Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        COALESCE(rq.total_qty, 0) as actual_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark,
		        e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN (
             SELECT entry_id, SUM(quantity) as total_qty
             FROM room_entries
             GROUP BY entry_id
         ) rq ON e.id = rq.entry_id
         WHERE e.created_at > $1::timestamptz
         ORDER BY e.created_at DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
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

// GetMaxThockNumber returns the highest thock number for a category
func (r *EntryRepository) GetMaxThockNumber(ctx context.Context, category string) (int, error) {
	if category != "seed" && category != "sell" {
		return 0, fmt.Errorf("invalid category: %s", category)
	}

	// Default starting values
	baseOffset := 1
	if category == "sell" {
		baseOffset = 1501
	}

	var maxThock int
	query := `
		SELECT COALESCE(MAX(
			CAST(SPLIT_PART(thock_number, '/', 1) AS INTEGER)
		), $1 - 1)
		FROM entries
		WHERE thock_category = $2
	`
	err := r.DB.QueryRow(ctx, query, baseOffset, category).Scan(&maxThock)
	return maxThock, err
}

func (r *EntryRepository) ListUnassigned(ctx context.Context) ([]*models.Entry, error) {
	// Get entries that don't have a room entry yet
	// For unassigned entries, actual_quantity will be 0 (no room_entries yet)
	// OPTIMIZED: No subquery needed - unassigned means 0 quantity
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        0 as actual_quantity,
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark,
		        e.created_by_user_id, e.created_at, e.updated_at
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
			&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
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
		        e.thock_category, e.thock_number, COALESCE(e.remark, '') as remark, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e WHERE e.thock_number=$1`, thockNumber)

	var entry models.Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.ActualQuantity, &entry.ThockCategory, &entry.ThockNumber, &entry.Remark, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

// Update updates an existing entry (recalculates thock_number if category or quantity changes)
func (r *EntryRepository) Update(ctx context.Context, e *models.Entry, oldCategory string, oldQty int) error {
	// Check if we need to regenerate thock_number
	categoryChanged := oldCategory != e.ThockCategory
	qtyChanged := oldQty != e.ExpectedQuantity

	if categoryChanged {
		// Category changed - need new thock number based on new category count
		var baseOffset int
		if e.ThockCategory == "seed" {
			baseOffset = 1
		} else {
			baseOffset = 1501
		}

		// Get count of entries in new category and generate new thock number
		query := `
			WITH next_num AS (
				SELECT COALESCE(COUNT(*), 0)::integer + $1::integer as num
				FROM entries
				WHERE thock_category = $2::text
			)
			UPDATE entries
			SET name=$3::text, phone=$4::text, village=$5::text, so=$6::text,
			    expected_quantity=$7::integer, remark=$8::text, thock_category=$9::text,
			    thock_number = CASE WHEN $9::text = 'seed'
			        THEN LPAD((SELECT num FROM next_num)::text, 4, '0') || '/' || $7::integer::text
			        ELSE (SELECT num FROM next_num)::text || '/' || $7::integer::text
			    END,
			    updated_at=NOW()
			WHERE id=$10::integer`
		_, err := r.DB.Exec(ctx, query, baseOffset, e.ThockCategory, e.Name, e.Phone, e.Village, e.SO,
			e.ExpectedQuantity, e.Remark, e.ThockCategory, e.ID)
		return err
	} else if qtyChanged {
		// Only quantity changed - update the quantity part of thock_number
		query := `
			UPDATE entries
			SET name=$1::text, phone=$2::text, village=$3::text, so=$4::text,
			    expected_quantity=$5::integer, remark=$6::text, thock_category=$7::text,
			    thock_number = CASE WHEN thock_category = 'seed'
			        THEN LPAD(SPLIT_PART(thock_number, '/', 1), 4, '0') || '/' || $5::integer::text
			        ELSE SPLIT_PART(thock_number, '/', 1) || '/' || $5::integer::text
			    END,
			    updated_at=NOW()
			WHERE id=$8::integer`
		_, err := r.DB.Exec(ctx, query, e.Name, e.Phone, e.Village, e.SO,
			e.ExpectedQuantity, e.Remark, e.ThockCategory, e.ID)
		return err
	} else {
		// No category or quantity change - simple update
		query := `UPDATE entries SET name=$1::text, phone=$2::text, village=$3::text, so=$4::text,
		          expected_quantity=$5::integer, remark=$6::text, thock_category=$7::text, updated_at=NOW()
		          WHERE id=$8::integer`
		_, err := r.DB.Exec(ctx, query, e.Name, e.Phone, e.Village, e.SO,
			e.ExpectedQuantity, e.Remark, e.ThockCategory, e.ID)
		return err
	}
}
