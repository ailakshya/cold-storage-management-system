package repositories

import (
	"context"
	"errors"
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

// getNextTokenNumber gets the next token number for today, skipping any lost tokens
func (r *GuardEntryRepository) getNextTokenNumber(ctx context.Context) (int, error) {
	// Get the max token number used today (from entries or skipped)
	query := `
		SELECT COALESCE(
			GREATEST(
				(SELECT COALESCE(MAX(token_number), 0) FROM guard_entries WHERE DATE(created_at) = CURRENT_DATE),
				(SELECT COALESCE(MAX(token_number), 0) FROM skipped_tokens WHERE skip_date = CURRENT_DATE)
			), 0
		) + 1
	`
	var tokenNumber int
	err := r.DB.QueryRow(ctx, query).Scan(&tokenNumber)
	return tokenNumber, err
}

// SkipToken marks a token number as skipped (lost physical token)
func (r *GuardEntryRepository) SkipToken(ctx context.Context, tokenNumber int, reason string, userID int) error {
	query := `
		INSERT INTO skipped_tokens (token_number, skip_date, reason, skipped_by_user_id)
		VALUES ($1, CURRENT_DATE, $2, $3)
		ON CONFLICT (token_number, skip_date) DO NOTHING
	`
	_, err := r.DB.Exec(ctx, query, tokenNumber, reason, userID)
	return err
}

// GetNextAvailableToken returns the next token number that will be assigned
func (r *GuardEntryRepository) GetNextAvailableToken(ctx context.Context) (int, error) {
	return r.getNextTokenNumber(ctx)
}

// GetTodaySkippedTokens returns all skipped tokens for today
func (r *GuardEntryRepository) GetTodaySkippedTokens(ctx context.Context) ([]int, error) {
	query := `
		SELECT token_number FROM skipped_tokens
		WHERE skip_date = CURRENT_DATE
		ORDER BY token_number
	`
	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []int
	for rows.Next() {
		var token int
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// Create creates a new guard entry with auto-generated token number
func (r *GuardEntryRepository) Create(ctx context.Context, entry *models.GuardEntry) error {
	// Get next token number for today
	tokenNumber, err := r.getNextTokenNumber(ctx)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO guard_entries (token_number, customer_id, family_member_id, customer_name, so, village, mobile, driver_no, seed_quantity, sell_quantity,
			seed_qty_1, seed_qty_2, seed_qty_3, seed_qty_4, sell_qty_1, sell_qty_2, sell_qty_3, sell_qty_4,
			remarks, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id, arrival_time, status, created_at, updated_at
	`
	err = r.DB.QueryRow(ctx, query,
		tokenNumber,
		entry.CustomerID,
		entry.FamilyMemberID,
		entry.CustomerName,
		entry.SO,
		entry.Village,
		entry.Mobile,
		entry.DriverNo,
		entry.SeedQuantity,
		entry.SellQuantity,
		entry.SeedQty1,
		entry.SeedQty2,
		entry.SeedQty3,
		entry.SeedQty4,
		entry.SellQty1,
		entry.SellQty2,
		entry.SellQty3,
		entry.SellQty4,
		entry.Remarks,
		entry.CreatedByUserID,
	).Scan(&entry.ID, &entry.ArrivalTime, &entry.Status, &entry.CreatedAt, &entry.UpdatedAt)

	if err == nil {
		entry.TokenNumber = tokenNumber
	}
	return err
}

// Get retrieves a guard entry by ID with user names
func (r *GuardEntryRepository) Get(ctx context.Context, id int) (*models.GuardEntry, error) {
	query := `
		SELECT g.id, COALESCE(g.token_number, 0) as token_number,
		       g.customer_name, COALESCE(g.so, '') as so, g.village, g.mobile, COALESCE(g.driver_no, '') as driver_no,
		       g.arrival_time, COALESCE(g.seed_quantity, 0) as seed_quantity, COALESCE(g.sell_quantity, 0) as sell_quantity,
		       COALESCE(g.seed_qty_1, 0), COALESCE(g.seed_qty_2, 0), COALESCE(g.seed_qty_3, 0), COALESCE(g.seed_qty_4, 0),
		       COALESCE(g.sell_qty_1, 0), COALESCE(g.sell_qty_2, 0), COALESCE(g.sell_qty_3, 0), COALESCE(g.sell_qty_4, 0),
		       COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at,
		       COALESCE(g.seed_processed, false) as seed_processed,
		       COALESCE(g.sell_processed, false) as sell_processed,
		       u1.name as created_by_name,
		       COALESCE(u2.name, '') as processed_by_name
		FROM guard_entries g
		LEFT JOIN users u1 ON g.created_by_user_id = u1.id
		LEFT JOIN users u2 ON g.processed_by_user_id = u2.id
		WHERE g.id = $1
	`
	var entry models.GuardEntry
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&entry.ID, &entry.TokenNumber,
		&entry.CustomerName, &entry.SO, &entry.Village, &entry.Mobile, &entry.DriverNo,
		&entry.ArrivalTime, &entry.SeedQuantity, &entry.SellQuantity,
		&entry.SeedQty1, &entry.SeedQty2, &entry.SeedQty3, &entry.SeedQty4,
		&entry.SellQty1, &entry.SellQty2, &entry.SellQty3, &entry.SellQty4,
		&entry.Remarks, &entry.Status,
		&entry.CreatedByUserID, &entry.ProcessedByUserID, &entry.ProcessedAt,
		&entry.CreatedAt, &entry.UpdatedAt,
		&entry.SeedProcessed, &entry.SellProcessed,
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
		SELECT g.id, COALESCE(g.token_number, 0) as token_number,
		       g.customer_name, COALESCE(g.so, '') as so, g.village, g.mobile, COALESCE(g.driver_no, '') as driver_no,
		       g.arrival_time, COALESCE(g.seed_quantity, 0) as seed_quantity, COALESCE(g.sell_quantity, 0) as sell_quantity,
		       COALESCE(g.seed_qty_1, 0), COALESCE(g.seed_qty_2, 0), COALESCE(g.seed_qty_3, 0), COALESCE(g.seed_qty_4, 0),
		       COALESCE(g.sell_qty_1, 0), COALESCE(g.sell_qty_2, 0), COALESCE(g.sell_qty_3, 0), COALESCE(g.sell_qty_4, 0),
		       COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at
		FROM guard_entries g
		WHERE g.created_by_user_id = $1
		  AND DATE(g.created_at) = CURRENT_DATE
		ORDER BY g.token_number DESC
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
			&entry.ID, &entry.TokenNumber,
			&entry.CustomerName, &entry.SO, &entry.Village, &entry.Mobile, &entry.DriverNo,
			&entry.ArrivalTime, &entry.SeedQuantity, &entry.SellQuantity,
			&entry.SeedQty1, &entry.SeedQty2, &entry.SeedQty3, &entry.SeedQty4,
			&entry.SellQty1, &entry.SellQty2, &entry.SellQty3, &entry.SellQty4,
			&entry.Remarks, &entry.Status,
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
// Shows entries that have at least one unprocessed portion
func (r *GuardEntryRepository) ListPending(ctx context.Context) ([]*models.GuardEntry, error) {
	query := `
		SELECT g.id, COALESCE(g.token_number, 0) as token_number,
		       g.customer_name, COALESCE(g.so, '') as so, g.village, g.mobile, COALESCE(g.driver_no, '') as driver_no,
		       g.arrival_time, COALESCE(g.seed_quantity, 0) as seed_quantity, COALESCE(g.sell_quantity, 0) as sell_quantity,
		       COALESCE(g.seed_qty_1, 0), COALESCE(g.seed_qty_2, 0), COALESCE(g.seed_qty_3, 0), COALESCE(g.seed_qty_4, 0),
		       COALESCE(g.sell_qty_1, 0), COALESCE(g.sell_qty_2, 0), COALESCE(g.sell_qty_3, 0), COALESCE(g.sell_qty_4, 0),
		       COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at,
		       COALESCE(g.seed_processed, false) as seed_processed,
		       COALESCE(g.sell_processed, false) as sell_processed,
		       u.name as created_by_name
		FROM guard_entries g
		LEFT JOIN users u ON g.created_by_user_id = u.id
		WHERE g.status = 'pending'
		  AND (
		    (COALESCE(g.seed_quantity, 0) > 0 AND COALESCE(g.seed_processed, false) = false)
		    OR
		    (COALESCE(g.sell_quantity, 0) > 0 AND COALESCE(g.sell_processed, false) = false)
		  )
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
			&entry.ID, &entry.TokenNumber,
			&entry.CustomerName, &entry.SO, &entry.Village, &entry.Mobile, &entry.DriverNo,
			&entry.ArrivalTime, &entry.SeedQuantity, &entry.SellQuantity,
			&entry.SeedQty1, &entry.SeedQty2, &entry.SeedQty3, &entry.SeedQty4,
			&entry.SellQty1, &entry.SellQty2, &entry.SellQty3, &entry.SellQty4,
			&entry.Remarks, &entry.Status,
			&entry.CreatedByUserID, &entry.ProcessedByUserID, &entry.ProcessedAt,
			&entry.CreatedAt, &entry.UpdatedAt,
			&entry.SeedProcessed, &entry.SellProcessed,
			&entry.CreatedByUserName,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// MarkAsProcessed marks a guard entry as fully processed
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

// MarkPortionProcessed marks either seed or sell portion as processed
func (r *GuardEntryRepository) MarkPortionProcessed(ctx context.Context, id int, portion string, processedByUserID int) error {
	now := time.Now()
	var query string

	if portion == "seed" {
		query = `
			UPDATE guard_entries
			SET seed_processed = true,
			    seed_processed_by = $2,
			    seed_processed_at = $3,
			    updated_at = $3
			WHERE id = $1
		`
	} else if portion == "sell" {
		query = `
			UPDATE guard_entries
			SET sell_processed = true,
			    sell_processed_by = $2,
			    sell_processed_at = $3,
			    updated_at = $3
			WHERE id = $1
		`
	} else {
		return errors.New("invalid portion: must be 'seed' or 'sell'")
	}

	_, err := r.DB.Exec(ctx, query, id, processedByUserID, now)
	if err != nil {
		return err
	}

	// Check if both portions are now processed, if so mark the whole entry as processed
	checkQuery := `
		SELECT
			COALESCE(seed_quantity, 0) as seed_qty,
			COALESCE(sell_quantity, 0) as sell_qty,
			COALESCE(seed_processed, false) as seed_done,
			COALESCE(sell_processed, false) as sell_done
		FROM guard_entries WHERE id = $1
	`
	var seedQty, sellQty int
	var seedDone, sellDone bool
	err = r.DB.QueryRow(ctx, checkQuery, id).Scan(&seedQty, &sellQty, &seedDone, &sellDone)
	if err != nil {
		return nil // Ignore check error, main update succeeded
	}

	// If all non-zero portions are processed, mark whole entry as processed
	seedComplete := seedQty == 0 || seedDone
	sellComplete := sellQty == 0 || sellDone

	if seedComplete && sellComplete {
		return r.MarkAsProcessed(ctx, id, processedByUserID)
	}

	return nil
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

// Delete deletes a guard entry by ID
func (r *GuardEntryRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM guard_entries WHERE id = $1`
	result, err := r.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("guard entry not found")
	}
	return nil
}

// SearchByPhone searches guard entries by phone number (fuzzy)
func (r *GuardEntryRepository) SearchByPhone(ctx context.Context, phone string) ([]*models.GuardEntry, error) {
	query := `
		SELECT g.id, COALESCE(g.token_number, 0) as token_number,
		       g.customer_name, COALESCE(g.so, '') as so, g.village, g.mobile, COALESCE(g.driver_no, '') as driver_no,
		       g.arrival_time, COALESCE(g.seed_quantity, 0) as seed_quantity, COALESCE(g.sell_quantity, 0) as sell_quantity,
		       COALESCE(g.seed_qty_1, 0), COALESCE(g.seed_qty_2, 0), COALESCE(g.seed_qty_3, 0), COALESCE(g.seed_qty_4, 0),
		       COALESCE(g.sell_qty_1, 0), COALESCE(g.sell_qty_2, 0), COALESCE(g.sell_qty_3, 0), COALESCE(g.sell_qty_4, 0),
		       COALESCE(g.remarks, '') as remarks, g.status,
		       g.created_by_user_id, g.processed_by_user_id, g.processed_at,
		       g.created_at, g.updated_at
		FROM guard_entries g
		WHERE g.mobile LIKE $1
		ORDER BY g.created_at DESC
		LIMIT 10
	`
	rows, err := r.DB.Query(ctx, query, "%"+phone+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.GuardEntry
	for rows.Next() {
		var entry models.GuardEntry
		err := rows.Scan(
			&entry.ID, &entry.TokenNumber,
			&entry.CustomerName, &entry.SO, &entry.Village, &entry.Mobile, &entry.DriverNo,
			&entry.ArrivalTime, &entry.SeedQuantity, &entry.SellQuantity,
			&entry.SeedQty1, &entry.SeedQty2, &entry.SeedQty3, &entry.SeedQty4,
			&entry.SellQty1, &entry.SellQty2, &entry.SellQty3, &entry.SellQty4,
			&entry.Remarks, &entry.Status,
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
