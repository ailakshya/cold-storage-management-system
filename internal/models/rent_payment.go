package models

import "time"

type RentPayment struct {
	ID                 int       `json:"id"`
	ReceiptNumber      string    `json:"receipt_number"`
	EntryID            int       `json:"entry_id"`
	CustomerName       string    `json:"customer_name"`
	CustomerPhone      string    `json:"customer_phone"`
	TotalRent          float64   `json:"total_rent"`
	AmountPaid         float64   `json:"amount_paid"`
	Balance            float64   `json:"balance"`
	PaymentDate        time.Time `json:"payment_date"`
	ProcessedByUserID  int       `json:"processed_by_user_id"`
	Notes              string    `json:"notes"`
	CreatedAt          time.Time `json:"created_at"`
}

type CreateRentPaymentRequest struct {
	EntryID       int     `json:"entry_id"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	TotalRent     float64 `json:"total_rent"`
	AmountPaid    float64 `json:"amount_paid"`
	Balance       float64 `json:"balance"`
	Notes         string  `json:"notes"`
}
