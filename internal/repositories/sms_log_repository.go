package repositories

import (
	"context"
	"fmt"
	"time"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SMSLogRepository struct {
	DB *pgxpool.Pool
}

func NewSMSLogRepository(db *pgxpool.Pool) *SMSLogRepository {
	return &SMSLogRepository{DB: db}
}

// Create logs a sent SMS
func (r *SMSLogRepository) Create(ctx context.Context, log *models.SMSLog) error {
	query := `
		INSERT INTO sms_logs (customer_id, phone, message_type, message, status, error_message, reference_id, cost, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	return r.DB.QueryRow(ctx, query,
		log.CustomerID,
		log.Phone,
		log.MessageType,
		log.Message,
		log.Status,
		log.ErrorMessage,
		log.ReferenceID,
		log.Cost,
		time.Now(),
	).Scan(&log.ID)
}

// UpdateStatus updates the status of an SMS
func (r *SMSLogRepository) UpdateStatus(ctx context.Context, id int, status string, errorMsg string) error {
	query := `UPDATE sms_logs SET status = $2, error_message = $3 WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id, status, errorMsg)
	return err
}

// MarkDelivered marks an SMS as delivered
func (r *SMSLogRepository) MarkDelivered(ctx context.Context, id int) error {
	query := `UPDATE sms_logs SET status = 'delivered', delivered_at = $2 WHERE id = $1`
	_, err := r.DB.Exec(ctx, query, id, time.Now())
	return err
}

// List returns SMS logs with pagination
func (r *SMSLogRepository) List(ctx context.Context, limit, offset int, messageType string) ([]*models.SMSLog, int, error) {
	var countQuery string
	var args []interface{}

	if messageType != "" {
		countQuery = `SELECT COUNT(*) FROM sms_logs WHERE message_type = $1`
		args = append(args, messageType)
	} else {
		countQuery = `SELECT COUNT(*) FROM sms_logs`
	}

	var total int
	if err := r.DB.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	var query string
	if messageType != "" {
		query = `
			SELECT
				s.id, s.customer_id, COALESCE(c.name, '') as customer_name,
				s.phone, s.message_type, s.message, s.status,
				COALESCE(s.error_message, ''), COALESCE(s.reference_id, ''),
				s.cost, s.created_at, s.delivered_at
			FROM sms_logs s
			LEFT JOIN customers c ON s.customer_id = c.id
			WHERE s.message_type = $1
			ORDER BY s.created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{messageType, limit, offset}
	} else {
		query = `
			SELECT
				s.id, s.customer_id, COALESCE(c.name, '') as customer_name,
				s.phone, s.message_type, s.message, s.status,
				COALESCE(s.error_message, ''), COALESCE(s.reference_id, ''),
				s.cost, s.created_at, s.delivered_at
			FROM sms_logs s
			LEFT JOIN customers c ON s.customer_id = c.id
			ORDER BY s.created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []interface{}{limit, offset}
	}

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*models.SMSLog
	for rows.Next() {
		log := &models.SMSLog{}
		err := rows.Scan(
			&log.ID, &log.CustomerID, &log.CustomerName,
			&log.Phone, &log.MessageType, &log.Message, &log.Status,
			&log.ErrorMessage, &log.ReferenceID,
			&log.Cost, &log.CreatedAt, &log.DeliveredAt,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetStats returns SMS statistics
func (r *SMSLogRepository) GetStats(ctx context.Context) (*models.SMSStats, error) {
	stats := &models.SMSStats{}

	query := `
		SELECT
			COUNT(*) as total_sent,
			COUNT(*) FILTER (WHERE status = 'delivered') as total_delivered,
			COUNT(*) FILTER (WHERE status = 'failed') as total_failed,
			COUNT(*) FILTER (WHERE created_at >= CURRENT_DATE) as today_sent,
			COALESCE(SUM(cost) FILTER (WHERE created_at >= CURRENT_DATE), 0) as today_cost,
			COUNT(*) FILTER (WHERE created_at >= DATE_TRUNC('month', CURRENT_DATE)) as month_sent,
			COALESCE(SUM(cost) FILTER (WHERE created_at >= DATE_TRUNC('month', CURRENT_DATE)), 0) as month_cost
		FROM sms_logs
	`

	err := r.DB.QueryRow(ctx, query).Scan(
		&stats.TotalSent,
		&stats.TotalDelivered,
		&stats.TotalFailed,
		&stats.TodaySent,
		&stats.TodayCost,
		&stats.MonthSent,
		&stats.MonthCost,
	)

	return stats, err
}

// GetByCustomer returns SMS logs for a specific customer
func (r *SMSLogRepository) GetByCustomer(ctx context.Context, customerID int, limit int) ([]*models.SMSLog, error) {
	query := `
		SELECT
			id, customer_id, phone, message_type, message, status,
			COALESCE(error_message, ''), COALESCE(reference_id, ''),
			cost, created_at, delivered_at
		FROM sms_logs
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.DB.Query(ctx, query, customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.SMSLog
	for rows.Next() {
		log := &models.SMSLog{}
		err := rows.Scan(
			&log.ID, &log.CustomerID, &log.Phone, &log.MessageType, &log.Message, &log.Status,
			&log.ErrorMessage, &log.ReferenceID,
			&log.Cost, &log.CreatedAt, &log.DeliveredAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetCustomersWithBalance returns customers with balance for payment reminders
func (r *SMSLogRepository) GetCustomersWithBalance(ctx context.Context, minBalance float64) ([]map[string]interface{}, error) {
	query := `
		WITH customer_balances AS (
			SELECT
				c.id as customer_id,
				c.name,
				c.phone,
				COALESCE(SUM(
					CASE
						WHEN l.transaction_type = 'rent_charge' THEN l.amount
						WHEN l.transaction_type = 'payment' THEN -l.amount
						ELSE 0
					END
				), 0) as balance
			FROM customers c
			LEFT JOIN ledger l ON c.id = l.customer_id
			WHERE c.deleted_at IS NULL
			GROUP BY c.id, c.name, c.phone
		)
		SELECT customer_id, name, phone, balance
		FROM customer_balances
		WHERE balance >= $1
		ORDER BY balance DESC
	`

	rows, err := r.DB.Query(ctx, query, minBalance)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []map[string]interface{}
	for rows.Next() {
		var customerID int
		var name, phone string
		var balance float64
		if err := rows.Scan(&customerID, &name, &phone, &balance); err != nil {
			return nil, err
		}
		customers = append(customers, map[string]interface{}{
			"customer_id": customerID,
			"name":        name,
			"phone":       phone,
			"balance":     balance,
		})
	}

	return customers, nil
}

// GetFilteredCustomers returns customers based on filters
func (r *SMSLogRepository) GetFilteredCustomers(ctx context.Context, filters models.SMSFilter) ([]map[string]interface{}, error) {
	query := `
		WITH customer_data AS (
			SELECT
				c.id as customer_id,
				c.name,
				c.phone,
				COALESCE((
					SELECT SUM(CASE WHEN l.transaction_type = 'rent_charge' THEN l.amount WHEN l.transaction_type = 'payment' THEN -l.amount ELSE 0 END)
					FROM ledger l WHERE l.customer_id = c.id
				), 0) as balance,
				COALESCE((
					SELECT SUM(re.quantity)
					FROM entries e
					JOIN room_entries re ON e.thock_number = re.thock_number
					WHERE e.customer_id = c.id AND e.deleted_at IS NULL
				), 0) as items_stored,
				COALESCE((
					SELECT MAX(e.created_at)
					FROM entries e WHERE e.customer_id = c.id
				), c.created_at) as last_activity,
				EXISTS(SELECT 1 FROM entries e WHERE e.customer_id = c.id AND e.deleted_at IS NULL) as has_active_entry
			FROM customers c
			WHERE c.deleted_at IS NULL AND c.phone IS NOT NULL AND c.phone != ''
		)
		SELECT customer_id, name, phone, balance, items_stored
		FROM customer_data
		WHERE 1=1
	`

	args := []interface{}{}
	argNum := 1

	if filters.MinBalance != nil {
		query += fmt.Sprintf(" AND balance >= $%d", argNum)
		args = append(args, *filters.MinBalance)
		argNum++
	}
	if filters.MaxBalance != nil {
		query += fmt.Sprintf(" AND balance <= $%d", argNum)
		args = append(args, *filters.MaxBalance)
		argNum++
	}
	if filters.MinItemsStored != nil {
		query += fmt.Sprintf(" AND items_stored >= $%d", argNum)
		args = append(args, *filters.MinItemsStored)
		argNum++
	}
	if filters.HasActiveEntry != nil && *filters.HasActiveEntry {
		query += " AND has_active_entry = true"
	}
	if filters.InactiveDays != nil {
		query += fmt.Sprintf(" AND last_activity < NOW() - INTERVAL '1 day' * $%d", argNum)
		args = append(args, *filters.InactiveDays)
		argNum++
	}

	query += " ORDER BY balance DESC LIMIT 5000"

	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []map[string]interface{}
	for rows.Next() {
		var customerID int
		var name, phone string
		var balance float64
		var itemsStored int
		if err := rows.Scan(&customerID, &name, &phone, &balance, &itemsStored); err != nil {
			return nil, err
		}
		customers = append(customers, map[string]interface{}{
			"customer_id":   customerID,
			"name":          name,
			"phone":         phone,
			"balance":       balance,
			"items_stored":  itemsStored,
		})
	}

	return customers, nil
}
