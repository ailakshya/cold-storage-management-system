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

type DebtRequestRepository struct {
	DB *pgxpool.Pool
}

func NewDebtRequestRepository(db *pgxpool.Pool) *DebtRequestRepository {
	return &DebtRequestRepository{DB: db}
}

// Create creates a new debt request
func (r *DebtRequestRepository) Create(ctx context.Context, req *models.CreateDebtRequestRequest, requestedByUserID int, requestedByName string) (*models.DebtRequest, error) {
	// Set expiry to 24 hours from now
	expiresAt := time.Now().Add(24 * time.Hour)

	query := `
		INSERT INTO debt_requests (
			customer_phone, customer_name, customer_so, thock_number,
			requested_quantity, current_balance, requested_by_user_id,
			requested_by_name, status, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9)
		RETURNING id, created_at
	`

	var id int
	var createdAt time.Time
	err := r.DB.QueryRow(ctx, query,
		req.CustomerPhone,
		req.CustomerName,
		req.CustomerSO,
		req.ThockNumber,
		req.RequestedQuantity,
		req.CurrentBalance,
		requestedByUserID,
		requestedByName,
		expiresAt,
	).Scan(&id, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create debt request: %w", err)
	}

	return &models.DebtRequest{
		ID:                id,
		CustomerPhone:     req.CustomerPhone,
		CustomerName:      req.CustomerName,
		CustomerSO:        req.CustomerSO,
		ThockNumber:       req.ThockNumber,
		RequestedQuantity: req.RequestedQuantity,
		CurrentBalance:    req.CurrentBalance,
		RequestedByUserID: requestedByUserID,
		RequestedByName:   requestedByName,
		Status:            models.DebtRequestStatusPending,
		CreatedAt:         createdAt,
		ExpiresAt:         &expiresAt,
	}, nil
}

// GetByID returns a debt request by ID
func (r *DebtRequestRepository) GetByID(ctx context.Context, id int) (*models.DebtRequest, error) {
	query := `
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			thock_number, requested_quantity, current_balance,
			requested_by_user_id, COALESCE(requested_by_name, '') as requested_by_name,
			status, approved_by_user_id, COALESCE(approved_by_name, '') as approved_by_name,
			approved_at, COALESCE(rejection_reason, '') as rejection_reason,
			gate_pass_id, created_at, expires_at
		FROM debt_requests
		WHERE id = $1
	`

	var d models.DebtRequest
	var approvedByUserID *int
	var approvedAt *time.Time
	var gatePassID *int
	var expiresAt *time.Time

	err := r.DB.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.CustomerPhone, &d.CustomerName, &d.CustomerSO,
		&d.ThockNumber, &d.RequestedQuantity, &d.CurrentBalance,
		&d.RequestedByUserID, &d.RequestedByName,
		&d.Status, &approvedByUserID, &d.ApprovedByName,
		&approvedAt, &d.RejectionReason,
		&gatePassID, &d.CreatedAt, &expiresAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	d.ApprovedByUserID = approvedByUserID
	d.ApprovedAt = approvedAt
	d.GatePassID = gatePassID
	d.ExpiresAt = expiresAt

	return &d, nil
}

// GetPending returns all pending debt requests
func (r *DebtRequestRepository) GetPending(ctx context.Context) ([]models.DebtRequest, error) {
	// Also expire old requests
	r.ExpireOldRequests(ctx)

	query := `
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			thock_number, requested_quantity, current_balance,
			requested_by_user_id, COALESCE(requested_by_name, '') as requested_by_name,
			status, approved_by_user_id, COALESCE(approved_by_name, '') as approved_by_name,
			approved_at, COALESCE(rejection_reason, '') as rejection_reason,
			gate_pass_id, created_at, expires_at
		FROM debt_requests
		WHERE status = 'pending'
		ORDER BY created_at DESC
	`

	return r.queryDebtRequests(ctx, query)
}

// GetByCustomer returns debt requests for a customer
func (r *DebtRequestRepository) GetByCustomer(ctx context.Context, customerPhone string) ([]models.DebtRequest, error) {
	query := `
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			thock_number, requested_quantity, current_balance,
			requested_by_user_id, COALESCE(requested_by_name, '') as requested_by_name,
			status, approved_by_user_id, COALESCE(approved_by_name, '') as approved_by_name,
			approved_at, COALESCE(rejection_reason, '') as rejection_reason,
			gate_pass_id, created_at, expires_at
		FROM debt_requests
		WHERE customer_phone = $1
		ORDER BY created_at DESC
	`

	return r.queryDebtRequestsWithArgs(ctx, query, customerPhone)
}

// GetApprovedForCustomerAndThock returns an approved (not used) debt request for a specific customer and thock
func (r *DebtRequestRepository) GetApprovedForCustomerAndThock(ctx context.Context, customerPhone, thockNumber string) (*models.DebtRequest, error) {
	query := `
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			thock_number, requested_quantity, current_balance,
			requested_by_user_id, COALESCE(requested_by_name, '') as requested_by_name,
			status, approved_by_user_id, COALESCE(approved_by_name, '') as approved_by_name,
			approved_at, COALESCE(rejection_reason, '') as rejection_reason,
			gate_pass_id, created_at, expires_at
		FROM debt_requests
		WHERE customer_phone = $1 AND thock_number = $2 AND status = 'approved'
		ORDER BY created_at DESC
		LIMIT 1
	`

	requests, err := r.queryDebtRequestsWithArgs(ctx, query, customerPhone, thockNumber)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, nil
	}
	return &requests[0], nil
}

// GetAll returns all debt requests with optional filters
func (r *DebtRequestRepository) GetAll(ctx context.Context, filter *models.DebtRequestFilter) ([]models.DebtRequest, error) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.CustomerPhone != "" {
		conditions = append(conditions, fmt.Sprintf("customer_phone = $%d", argNum))
		args = append(args, filter.CustomerPhone)
		argNum++
	}

	if filter.ThockNumber != "" {
		conditions = append(conditions, fmt.Sprintf("thock_number = $%d", argNum))
		args = append(args, filter.ThockNumber)
		argNum++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, filter.Status)
		argNum++
	}

	if filter.RequestedBy > 0 {
		conditions = append(conditions, fmt.Sprintf("requested_by_user_id = $%d", argNum))
		args = append(args, filter.RequestedBy)
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
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, customer_phone, customer_name, COALESCE(customer_so, '') as customer_so,
			thock_number, requested_quantity, current_balance,
			requested_by_user_id, COALESCE(requested_by_name, '') as requested_by_name,
			status, approved_by_user_id, COALESCE(approved_by_name, '') as approved_by_name,
			approved_at, COALESCE(rejection_reason, '') as rejection_reason,
			gate_pass_id, created_at, expires_at
		FROM debt_requests
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argNum, argNum+1)

	args = append(args, limit, filter.Offset)

	return r.queryDebtRequestsWithArgs(ctx, query, args...)
}

// Approve approves a debt request
func (r *DebtRequestRepository) Approve(ctx context.Context, id int, approvedByUserID int, approvedByName string) error {
	query := `
		UPDATE debt_requests
		SET status = 'approved',
			approved_by_user_id = $2,
			approved_by_name = $3,
			approved_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.DB.Exec(ctx, query, id, approvedByUserID, approvedByName)
	if err != nil {
		return fmt.Errorf("failed to approve debt request: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("debt request not found or not in pending status")
	}

	return nil
}

// Reject rejects a debt request
func (r *DebtRequestRepository) Reject(ctx context.Context, id int, approvedByUserID int, approvedByName, reason string) error {
	query := `
		UPDATE debt_requests
		SET status = 'rejected',
			approved_by_user_id = $2,
			approved_by_name = $3,
			approved_at = CURRENT_TIMESTAMP,
			rejection_reason = $4
		WHERE id = $1 AND status = 'pending'
	`

	result, err := r.DB.Exec(ctx, query, id, approvedByUserID, approvedByName, reason)
	if err != nil {
		return fmt.Errorf("failed to reject debt request: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("debt request not found or not in pending status")
	}

	return nil
}

// MarkAsUsed marks an approved debt request as used and links the gate pass
func (r *DebtRequestRepository) MarkAsUsed(ctx context.Context, id int, gatePassID int) error {
	query := `
		UPDATE debt_requests
		SET status = 'used',
			gate_pass_id = $2
		WHERE id = $1 AND status = 'approved'
	`

	result, err := r.DB.Exec(ctx, query, id, gatePassID)
	if err != nil {
		return fmt.Errorf("failed to mark debt request as used: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("debt request not found or not in approved status")
	}

	return nil
}

// ExpireOldRequests expires pending requests older than 24 hours
func (r *DebtRequestRepository) ExpireOldRequests(ctx context.Context) error {
	query := `
		UPDATE debt_requests
		SET status = 'expired'
		WHERE status = 'pending' AND expires_at < CURRENT_TIMESTAMP
	`

	_, err := r.DB.Exec(ctx, query)
	return err
}

// GetPendingSummary returns summary of pending requests for dashboard
func (r *DebtRequestRepository) GetPendingSummary(ctx context.Context) (*models.PendingDebtRequestSummary, error) {
	query := `
		SELECT
			COUNT(*) as total_pending,
			COALESCE(SUM(current_balance), 0) as total_amount,
			COALESCE(EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - MIN(created_at))) / 3600, 0) as oldest_hours
		FROM debt_requests
		WHERE status = 'pending'
	`

	var summary models.PendingDebtRequestSummary
	var oldestHours float64
	err := r.DB.QueryRow(ctx, query).Scan(
		&summary.TotalPending,
		&summary.TotalAmount,
		&oldestHours,
	)
	if err != nil {
		return nil, err
	}

	summary.OldestRequestAge = int(oldestHours)
	return &summary, nil
}

// CountByStatus returns count of requests by status
func (r *DebtRequestRepository) CountByStatus(ctx context.Context, status models.DebtRequestStatus) (int, error) {
	var count int
	err := r.DB.QueryRow(ctx,
		"SELECT COUNT(*) FROM debt_requests WHERE status = $1",
		status,
	).Scan(&count)
	return count, err
}

// Helper function to query debt requests
func (r *DebtRequestRepository) queryDebtRequests(ctx context.Context, query string) ([]models.DebtRequest, error) {
	return r.queryDebtRequestsWithArgs(ctx, query)
}

func (r *DebtRequestRepository) queryDebtRequestsWithArgs(ctx context.Context, query string, args ...interface{}) ([]models.DebtRequest, error) {
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.DebtRequest
	for rows.Next() {
		var d models.DebtRequest
		var approvedByUserID *int
		var approvedAt *time.Time
		var gatePassID *int
		var expiresAt *time.Time

		err := rows.Scan(
			&d.ID, &d.CustomerPhone, &d.CustomerName, &d.CustomerSO,
			&d.ThockNumber, &d.RequestedQuantity, &d.CurrentBalance,
			&d.RequestedByUserID, &d.RequestedByName,
			&d.Status, &approvedByUserID, &d.ApprovedByName,
			&approvedAt, &d.RejectionReason,
			&gatePassID, &d.CreatedAt, &expiresAt,
		)
		if err != nil {
			return nil, err
		}

		d.ApprovedByUserID = approvedByUserID
		d.ApprovedAt = approvedAt
		d.GatePassID = gatePassID
		d.ExpiresAt = expiresAt

		requests = append(requests, d)
	}

	return requests, nil
}
