package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerRepository struct {
	DB *pgxpool.Pool
}

func NewLedgerRepository(db *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{DB: db}
}

// Create creates a new ledger entry and calculates running balance
func (r *LedgerRepository) Create(ctx context.Context, entry *models.CreateLedgerEntryRequest) (*models.LedgerEntry, error) {
	// Get current balance for customer
	currentBalance, err := r.GetBalance(ctx, entry.CustomerPhone)
	if err != nil {
		currentBalance = 0 // First entry for this customer
	}

	// Calculate new running balance
	runningBalance := currentBalance + entry.Debit - entry.Credit

	// Get user name (ID 0 = System for automated entries like online payments)
	var createdByName string
	if entry.CreatedByUserID == 0 {
		createdByName = "System"
	} else {
		err = r.DB.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", entry.CreatedByUserID).Scan(&createdByName)
		if err != nil {
			createdByName = "Unknown"
		}
	}

	query := `
		INSERT INTO ledger_entries (
			customer_phone, customer_name, customer_so, entry_type, description,
			debit, credit, running_balance, reference_id, reference_type,
			family_member_id, family_member_name,
			created_by_user_id, created_by_name, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at
	`

	var id int
	var createdAt time.Time
	err = r.DB.QueryRow(ctx, query,
		entry.CustomerPhone,
		entry.CustomerName,
		entry.CustomerSO,
		entry.EntryType,
		entry.Description,
		entry.Debit,
		entry.Credit,
		runningBalance,
		entry.ReferenceID,
		entry.ReferenceType,
		entry.FamilyMemberID,
		entry.FamilyMemberName,
		entry.CreatedByUserID,
		createdByName,
		entry.Notes,
	).Scan(&id, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create ledger entry: %w", err)
	}

	return &models.LedgerEntry{
		ID:               id,
		CustomerPhone:    entry.CustomerPhone,
		CustomerName:     entry.CustomerName,
		CustomerSO:       entry.CustomerSO,
		EntryType:        entry.EntryType,
		Description:      entry.Description,
		Debit:            entry.Debit,
		Credit:           entry.Credit,
		RunningBalance:   runningBalance,
		ReferenceID:      entry.ReferenceID,
		ReferenceType:    entry.ReferenceType,
		FamilyMemberID:   entry.FamilyMemberID,
		FamilyMemberName: entry.FamilyMemberName,
		CreatedByUserID:  entry.CreatedByUserID,
		CreatedByName:    createdByName,
		CreatedAt:        createdAt,
		Notes:            entry.Notes,
	}, nil
}

// GetBalance returns the current balance for a customer
func (r *LedgerRepository) GetBalance(ctx context.Context, customerPhone string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(debit) - SUM(credit), 0) as balance
		FROM ledger_entries
		WHERE customer_phone = $1
	`

	var balance float64
	err := r.DB.QueryRow(ctx, query, customerPhone).Scan(&balance)
	if err != nil {
		return 0, err
	}
	return balance, nil
}

// GetByCustomer returns all ledger entries for a customer
func (r *LedgerRepository) GetByCustomer(ctx context.Context, customerPhone string, limit, offset int) ([]models.LedgerEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			entry_type, COALESCE(description, '') as description, debit, credit, running_balance,
			reference_id, COALESCE(reference_type, '') as reference_type,
			family_member_id, COALESCE(family_member_name, '') as family_member_name,
			created_by_user_id, COALESCE(created_by_name, '') as created_by_name,
			created_at, COALESCE(notes, '') as notes
		FROM ledger_entries
		WHERE customer_phone = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.DB.Query(ctx, query, customerPhone, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LedgerEntry
	for rows.Next() {
		var e models.LedgerEntry
		var refID *int
		err := rows.Scan(
			&e.ID, &e.CustomerPhone, &e.CustomerName, &e.CustomerSO,
			&e.EntryType, &e.Description, &e.Debit, &e.Credit, &e.RunningBalance,
			&refID, &e.ReferenceType,
			&e.FamilyMemberID, &e.FamilyMemberName,
			&e.CreatedByUserID, &e.CreatedByName, &e.CreatedAt, &e.Notes,
		)
		if err != nil {
			return nil, err
		}
		e.ReferenceID = refID
		entries = append(entries, e)
	}

	return entries, nil
}

// GetAll returns all ledger entries with optional filters (for audit)
func (r *LedgerRepository) GetAll(ctx context.Context, filter *models.LedgerFilter) ([]models.LedgerEntry, error) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.CustomerPhone != "" {
		// Use LIKE for fuzzy/partial search on phone OR name
		conditions = append(conditions, fmt.Sprintf("(customer_phone LIKE $%d OR LOWER(customer_name) LIKE LOWER($%d))", argNum, argNum))
		args = append(args, "%"+filter.CustomerPhone+"%")
		argNum++
	}

	if filter.EntryType != "" {
		conditions = append(conditions, fmt.Sprintf("entry_type = $%d", argNum))
		args = append(args, filter.EntryType)
		argNum++
	}

	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argNum))
		args = append(args, filter.StartDate)
		argNum++
	}

	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argNum))
		args = append(args, filter.EndDate)
		argNum++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 500
	}

	query := fmt.Sprintf(`
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			entry_type, COALESCE(description, '') as description, debit, credit, running_balance,
			reference_id, COALESCE(reference_type, '') as reference_type,
			family_member_id, COALESCE(family_member_name, '') as family_member_name,
			created_by_user_id, COALESCE(created_by_name, '') as created_by_name,
			created_at, COALESCE(notes, '') as notes
		FROM ledger_entries
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argNum, argNum+1)

	args = append(args, limit, filter.Offset)

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.LedgerEntry
	for rows.Next() {
		var e models.LedgerEntry
		var refID *int
		err := rows.Scan(
			&e.ID, &e.CustomerPhone, &e.CustomerName, &e.CustomerSO,
			&e.EntryType, &e.Description, &e.Debit, &e.Credit, &e.RunningBalance,
			&refID, &e.ReferenceType,
			&e.FamilyMemberID, &e.FamilyMemberName,
			&e.CreatedByUserID, &e.CreatedByName, &e.CreatedAt, &e.Notes,
		)
		if err != nil {
			return nil, err
		}
		e.ReferenceID = refID
		entries = append(entries, e)
	}

	return entries, nil
}

// GetSummaryByCustomer returns balance summary for a customer
func (r *LedgerRepository) GetSummaryByCustomer(ctx context.Context, customerPhone string) (*models.LedgerSummary, error) {
	query := `
		SELECT
			customer_phone,
			MAX(customer_name) as customer_name,
			COALESCE(MAX(customer_so), '') as customer_so,
			COALESCE(SUM(debit), 0) as total_debit,
			COALESCE(SUM(credit), 0) as total_credit,
			COALESCE(SUM(debit) - SUM(credit), 0) as current_balance,
			COUNT(*) as entry_count
		FROM ledger_entries
		WHERE customer_phone = $1
		GROUP BY customer_phone
	`

	var s models.LedgerSummary
	err := r.DB.QueryRow(ctx, query, customerPhone).Scan(
		&s.CustomerPhone, &s.CustomerName, &s.CustomerSO,
		&s.TotalDebit, &s.TotalCredit, &s.CurrentBalance, &s.EntryCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No entries for this customer
		}
		return nil, err
	}
	return &s, nil
}

// GetAllCustomerBalances returns balance summaries for all customers
func (r *LedgerRepository) GetAllCustomerBalances(ctx context.Context) ([]models.LedgerSummary, error) {
	query := `
		SELECT
			customer_phone,
			MAX(customer_name) as customer_name,
			COALESCE(MAX(customer_so), '') as customer_so,
			COALESCE(SUM(debit), 0) as total_debit,
			COALESCE(SUM(credit), 0) as total_credit,
			COALESCE(SUM(debit) - SUM(credit), 0) as current_balance,
			COUNT(*) as entry_count
		FROM ledger_entries
		GROUP BY customer_phone
		ORDER BY current_balance DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []models.LedgerSummary
	for rows.Next() {
		var s models.LedgerSummary
		err := rows.Scan(
			&s.CustomerPhone, &s.CustomerName, &s.CustomerSO,
			&s.TotalDebit, &s.TotalCredit, &s.CurrentBalance, &s.EntryCount,
		)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

// GetDebtors returns customers with positive balance (they owe money)
func (r *LedgerRepository) GetDebtors(ctx context.Context) ([]models.LedgerSummary, error) {
	query := `
		SELECT
			customer_phone,
			MAX(customer_name) as customer_name,
			COALESCE(MAX(customer_so), '') as customer_so,
			COALESCE(SUM(debit), 0) as total_debit,
			COALESCE(SUM(credit), 0) as total_credit,
			COALESCE(SUM(debit) - SUM(credit), 0) as current_balance,
			COUNT(*) as entry_count
		FROM ledger_entries
		GROUP BY customer_phone
		HAVING SUM(debit) - SUM(credit) > 0
		ORDER BY current_balance DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []models.LedgerSummary
	for rows.Next() {
		var s models.LedgerSummary
		err := rows.Scan(
			&s.CustomerPhone, &s.CustomerName, &s.CustomerSO,
			&s.TotalDebit, &s.TotalCredit, &s.CurrentBalance, &s.EntryCount,
		)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

// CountByType returns count of entries by type
func (r *LedgerRepository) CountByType(ctx context.Context, entryType models.LedgerEntryType) (int, error) {
	var count int
	err := r.DB.QueryRow(ctx,
		"SELECT COUNT(*) FROM ledger_entries WHERE entry_type = $1",
		entryType,
	).Scan(&count)
	return count, err
}

// GetTotalsByType returns sum of debit/credit by entry type
func (r *LedgerRepository) GetTotalsByType(ctx context.Context) (map[models.LedgerEntryType]float64, error) {
	query := `
		SELECT entry_type,
			CASE
				WHEN entry_type IN ('CHARGE', 'REFUND') THEN SUM(debit)
				ELSE SUM(credit)
			END as total
		FROM ledger_entries
		GROUP BY entry_type
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make(map[models.LedgerEntryType]float64)
	for rows.Next() {
		var entryType models.LedgerEntryType
		var total float64
		if err := rows.Scan(&entryType, &total); err != nil {
			return nil, err
		}
		totals[entryType] = total
	}

	return totals, nil
}

// GetTotalCredit returns total payments (credits) for a customer
func (r *LedgerRepository) GetTotalCredit(ctx context.Context, customerPhone string) (float64, error) {
	var total float64
	err := r.DB.QueryRow(ctx,
		"SELECT COALESCE(SUM(credit), 0) FROM ledger_entries WHERE customer_phone = $1",
		customerPhone).Scan(&total)
	return total, err
}

// GetAllTotalCredits returns total credits for all customers (bulk query)
func (r *LedgerRepository) GetAllTotalCredits(ctx context.Context) (map[string]float64, error) {
	query := `
		SELECT customer_phone, COALESCE(SUM(credit), 0) as total_credit
		FROM ledger_entries
		GROUP BY customer_phone
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var phone string
		var total float64
		if err := rows.Scan(&phone, &total); err != nil {
			return nil, err
		}
		result[phone] = total
	}

	return result, nil
}

// GetAllPaymentHistory returns payment history for all customers (bulk query)
func (r *LedgerRepository) GetAllPaymentHistory(ctx context.Context) (map[string][]PaymentHistoryItem, error) {
	query := `
		SELECT customer_phone, id, credit, entry_type, COALESCE(description, ''), COALESCE(notes, ''),
		       family_member_id, COALESCE(family_member_name, ''), created_at
		FROM ledger_entries
		WHERE credit > 0
		ORDER BY created_at DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]PaymentHistoryItem)
	for rows.Next() {
		var phone string
		var p PaymentHistoryItem
		if err := rows.Scan(&phone, &p.ID, &p.Amount, &p.EntryType, &p.Description, &p.Notes,
			&p.FamilyMemberID, &p.FamilyMemberName, &p.CreatedAt); err != nil {
			return nil, err
		}
		result[phone] = append(result[phone], p)
	}

	return result, nil
}

// PaymentHistoryItem represents a payment in the history
type PaymentHistoryItem struct {
	ID               int       `json:"id"`
	Amount           float64   `json:"amount"`
	EntryType        string    `json:"entry_type"`
	Description      string    `json:"description"`
	Notes            string    `json:"notes"`
	FamilyMemberID   *int      `json:"family_member_id,omitempty"`
	FamilyMemberName string    `json:"family_member_name,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// FamilyMemberCredit represents total credit for a family member
type FamilyMemberCredit struct {
	FamilyMemberID   *int    `json:"family_member_id"`
	FamilyMemberName string  `json:"family_member_name"`
	TotalCredit      float64 `json:"total_credit"`
}

// GetCreditsByFamilyMember returns total credits grouped by family member for a customer
func (r *LedgerRepository) GetCreditsByFamilyMember(ctx context.Context, customerPhone string) ([]FamilyMemberCredit, error) {
	query := `
		SELECT family_member_id, COALESCE(family_member_name, '') as family_member_name,
		       COALESCE(SUM(credit), 0) as total_credit
		FROM ledger_entries
		WHERE customer_phone = $1 AND credit > 0
		GROUP BY family_member_id, family_member_name
		ORDER BY total_credit DESC
	`

	rows, err := r.DB.Query(ctx, query, customerPhone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FamilyMemberCredit
	for rows.Next() {
		var fc FamilyMemberCredit
		if err := rows.Scan(&fc.FamilyMemberID, &fc.FamilyMemberName, &fc.TotalCredit); err != nil {
			return nil, err
		}
		results = append(results, fc)
	}

	return results, nil
}

// UpdateCustomerPhone updates the phone number in all ledger entries for a customer
// Used when customer phone changes or when customers are merged
func (r *LedgerRepository) UpdateCustomerPhone(ctx context.Context, oldPhone, newPhone string) (int64, error) {
	result, err := r.DB.Exec(ctx,
		"UPDATE ledger_entries SET customer_phone = $1 WHERE customer_phone = $2",
		newPhone, oldPhone)
	if err != nil {
		return 0, fmt.Errorf("failed to update ledger entries phone: %w", err)
	}
	return result.RowsAffected(), nil
}

// GetPaymentHistory returns recent payments (credits) for a customer
func (r *LedgerRepository) GetPaymentHistory(ctx context.Context, customerPhone string, limit int) ([]PaymentHistoryItem, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, credit, entry_type, COALESCE(description, ''), COALESCE(notes, ''),
		       family_member_id, COALESCE(family_member_name, ''), created_at
		FROM ledger_entries
		WHERE customer_phone = $1 AND credit > 0
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.DB.Query(ctx, query, customerPhone, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []PaymentHistoryItem
	for rows.Next() {
		var p PaymentHistoryItem
		if err := rows.Scan(&p.ID, &p.Amount, &p.EntryType, &p.Description, &p.Notes,
			&p.FamilyMemberID, &p.FamilyMemberName, &p.CreatedAt); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return payments, nil
}
