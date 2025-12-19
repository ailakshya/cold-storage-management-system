package repositories

import (
	"context"
	"fmt"
	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RentPaymentRepository struct {
	DB *pgxpool.Pool
}

func NewRentPaymentRepository(db *pgxpool.Pool) *RentPaymentRepository {
	return &RentPaymentRepository{DB: db}
}

func (r *RentPaymentRepository) GenerateReceiptNumber(ctx context.Context) (string, error) {
	// PERFORMANCE FIX: Use database sequence instead of COUNT for O(1) performance
	var nextNum int
	err := r.DB.QueryRow(ctx, "SELECT nextval('receipt_number_sequence')").Scan(&nextNum)
	if err != nil {
		return "", fmt.Errorf("failed to get next receipt number: %w", err)
	}

	receiptNumber := fmt.Sprintf("RCP-%06d", nextNum)
	return receiptNumber, nil
}

// CheckDuplicatePayment checks if a similar payment was made within the last 10 seconds
// Returns true if a duplicate is found
func (r *RentPaymentRepository) CheckDuplicatePayment(ctx context.Context, customerPhone string, amountPaid float64) (bool, error) {
	query := `
		SELECT COUNT(*) FROM rent_payments
		WHERE customer_phone = $1
		AND amount_paid = $2
		AND created_at > NOW() - INTERVAL '10 seconds'
	`
	var count int
	err := r.DB.QueryRow(ctx, query, customerPhone, amountPaid).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *RentPaymentRepository) Create(ctx context.Context, payment *models.RentPayment) error {
	// Check for duplicate payment (same customer, same amount within 10 seconds)
	isDuplicate, err := r.CheckDuplicatePayment(ctx, payment.CustomerPhone, payment.AmountPaid)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate payment: %w", err)
	}
	if isDuplicate {
		return fmt.Errorf("duplicate payment detected: a payment of ₹%.2f for this customer was already processed within the last 10 seconds", payment.AmountPaid)
	}

	// Generate receipt number
	receiptNumber, err := r.GenerateReceiptNumber(ctx)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO rent_payments (receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance, processed_by_user_id, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, payment_date, created_at
	`

	err = r.DB.QueryRow(ctx, query,
		receiptNumber,
		payment.EntryID,
		payment.CustomerName,
		payment.CustomerPhone,
		payment.TotalRent,
		payment.AmountPaid,
		payment.Balance,
		payment.ProcessedByUserID,
		payment.Notes,
	).Scan(&payment.ID, &payment.PaymentDate, &payment.CreatedAt)

	if err != nil {
		return err
	}

	payment.ReceiptNumber = receiptNumber
	return nil
}

func (r *RentPaymentRepository) GetByEntryID(ctx context.Context, entryID int) ([]*models.RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		WHERE entry_id = $1
		ORDER BY payment_date DESC
	`

	rows, err := r.DB.Query(ctx, query, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*models.RentPayment
	for rows.Next() {
		payment := &models.RentPayment{}
		err := rows.Scan(
			&payment.ID,
			&payment.ReceiptNumber,
			&payment.EntryID,
			&payment.CustomerName,
			&payment.CustomerPhone,
			&payment.TotalRent,
			&payment.AmountPaid,
			&payment.Balance,
			&payment.PaymentDate,
			&payment.ProcessedByUserID,
			&payment.Notes,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *RentPaymentRepository) GetByPhone(ctx context.Context, phone string) ([]*models.RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		WHERE customer_phone = $1
		ORDER BY payment_date DESC
	`

	rows, err := r.DB.Query(ctx, query, phone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*models.RentPayment
	for rows.Next() {
		payment := &models.RentPayment{}
		err := rows.Scan(
			&payment.ID,
			&payment.ReceiptNumber,
			&payment.EntryID,
			&payment.CustomerName,
			&payment.CustomerPhone,
			&payment.TotalRent,
			&payment.AmountPaid,
			&payment.Balance,
			&payment.PaymentDate,
			&payment.ProcessedByUserID,
			&payment.Notes,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *RentPaymentRepository) List(ctx context.Context) ([]*models.RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		ORDER BY payment_date DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*models.RentPayment
	for rows.Next() {
		payment := &models.RentPayment{}
		err := rows.Scan(
			&payment.ID,
			&payment.ReceiptNumber,
			&payment.EntryID,
			&payment.CustomerName,
			&payment.CustomerPhone,
			&payment.TotalRent,
			&payment.AmountPaid,
			&payment.Balance,
			&payment.PaymentDate,
			&payment.ProcessedByUserID,
			&payment.Notes,
			&payment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func (r *RentPaymentRepository) GetByReceiptNumber(ctx context.Context, receiptNumber string) (*models.RentPayment, error) {
	query := `
		SELECT id, receipt_number, entry_id, customer_name, customer_phone, total_rent, amount_paid, balance,
		       payment_date, COALESCE(processed_by_user_id, 0), COALESCE(notes, ''), created_at
		FROM rent_payments
		WHERE receipt_number = $1
	`

	payment := &models.RentPayment{}
	err := r.DB.QueryRow(ctx, query, receiptNumber).Scan(
		&payment.ID,
		&payment.ReceiptNumber,
		&payment.EntryID,
		&payment.CustomerName,
		&payment.CustomerPhone,
		&payment.TotalRent,
		&payment.AmountPaid,
		&payment.Balance,
		&payment.PaymentDate,
		&payment.ProcessedByUserID,
		&payment.Notes,
		&payment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return payment, nil
}
