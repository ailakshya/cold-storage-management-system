package repositories

import (
	"context"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerActivityLogRepository struct {
	DB *pgxpool.Pool
}

func NewCustomerActivityLogRepository(db *pgxpool.Pool) *CustomerActivityLogRepository {
	return &CustomerActivityLogRepository{DB: db}
}

// Create logs a customer activity
func (r *CustomerActivityLogRepository) Create(ctx context.Context, log *models.CustomerActivityLog) error {
	query := `
		INSERT INTO customer_activity_logs (customer_id, phone, action, details, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	return r.DB.QueryRow(ctx, query,
		log.CustomerID,
		log.Phone,
		log.Action,
		log.Details,
		log.IPAddress,
		log.UserAgent,
		time.Now(),
	).Scan(&log.ID)
}

// List returns activity logs with pagination
func (r *CustomerActivityLogRepository) List(ctx context.Context, limit, offset int) ([]*models.CustomerActivityLog, int, error) {
	countQuery := `SELECT COUNT(*) FROM customer_activity_logs`
	var total int
	if err := r.DB.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			cal.id, cal.customer_id, COALESCE(c.name, '') as customer_name,
			cal.phone, cal.action, COALESCE(cal.details, ''),
			COALESCE(cal.ip_address, ''), COALESCE(cal.user_agent, ''), cal.created_at
		FROM customer_activity_logs cal
		LEFT JOIN customers c ON cal.customer_id = c.id
		ORDER BY cal.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.DB.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.CustomerActivityLog
	for rows.Next() {
		log := &models.CustomerActivityLog{}
		err := rows.Scan(
			&log.ID, &log.CustomerID, &log.CustomerName,
			&log.Phone, &log.Action, &log.Details,
			&log.IPAddress, &log.UserAgent, &log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}

	return logs, total, nil
}

// ListByCustomer returns activity logs for a specific customer
func (r *CustomerActivityLogRepository) ListByCustomer(ctx context.Context, customerID int, limit int) ([]*models.CustomerActivityLog, error) {
	query := `
		SELECT
			id, customer_id, phone, action, COALESCE(details, ''),
			COALESCE(ip_address, ''), COALESCE(user_agent, ''), created_at
		FROM customer_activity_logs
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.DB.Query(ctx, query, customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.CustomerActivityLog
	for rows.Next() {
		log := &models.CustomerActivityLog{}
		err := rows.Scan(
			&log.ID, &log.CustomerID, &log.Phone, &log.Action, &log.Details,
			&log.IPAddress, &log.UserAgent, &log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ListByAction returns activity logs filtered by action type
func (r *CustomerActivityLogRepository) ListByAction(ctx context.Context, action string, limit, offset int) ([]*models.CustomerActivityLog, int, error) {
	countQuery := `SELECT COUNT(*) FROM customer_activity_logs WHERE action = $1`
	var total int
	if err := r.DB.QueryRow(ctx, countQuery, action).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			cal.id, cal.customer_id, COALESCE(c.name, '') as customer_name,
			cal.phone, cal.action, COALESCE(cal.details, ''),
			COALESCE(cal.ip_address, ''), COALESCE(cal.user_agent, ''), cal.created_at
		FROM customer_activity_logs cal
		LEFT JOIN customers c ON cal.customer_id = c.id
		WHERE cal.action = $1
		ORDER BY cal.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.DB.Query(ctx, query, action, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.CustomerActivityLog
	for rows.Next() {
		log := &models.CustomerActivityLog{}
		err := rows.Scan(
			&log.ID, &log.CustomerID, &log.CustomerName,
			&log.Phone, &log.Action, &log.Details,
			&log.IPAddress, &log.UserAgent, &log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetStats returns activity statistics
func (r *CustomerActivityLogRepository) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Today's stats
	todayQuery := `
		SELECT
			COUNT(*) FILTER (WHERE action = 'login') as logins_today,
			COUNT(*) FILTER (WHERE action = 'otp_requested') as otp_requests_today,
			COUNT(*) FILTER (WHERE action = 'otp_failed') as otp_failures_today,
			COUNT(*) FILTER (WHERE action = 'gate_pass_request') as gate_pass_requests_today
		FROM customer_activity_logs
		WHERE created_at >= CURRENT_DATE
	`

	var loginsToday, otpRequestsToday, otpFailuresToday, gatePassRequestsToday int
	err := r.DB.QueryRow(ctx, todayQuery).Scan(&loginsToday, &otpRequestsToday, &otpFailuresToday, &gatePassRequestsToday)
	if err != nil {
		return nil, err
	}

	stats["logins_today"] = loginsToday
	stats["otp_requests_today"] = otpRequestsToday
	stats["otp_failures_today"] = otpFailuresToday
	stats["gate_pass_requests_today"] = gatePassRequestsToday

	// Unique customers today
	uniqueQuery := `SELECT COUNT(DISTINCT customer_id) FROM customer_activity_logs WHERE created_at >= CURRENT_DATE AND customer_id > 0`
	var uniqueCustomers int
	r.DB.QueryRow(ctx, uniqueQuery).Scan(&uniqueCustomers)
	stats["unique_customers_today"] = uniqueCustomers

	return stats, nil
}
