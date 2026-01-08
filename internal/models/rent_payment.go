package models

import "time"

type RentPayment struct {
	ID                 int       `json:"id"`
	ReceiptNumber      string    `json:"receipt_number"`
	EntryID            int       `json:"entry_id"`
	FamilyMemberID     *int      `json:"family_member_id,omitempty"`
	FamilyMemberName   string    `json:"family_member_name,omitempty"`
	CustomerName       string    `json:"customer_name"`
	CustomerPhone      string    `json:"customer_phone"`
	TotalRent          float64   `json:"total_rent"`
	AmountPaid         float64   `json:"amount_paid"`
	Balance            float64   `json:"balance"`
	PaymentType        string    `json:"payment_type"` // "cash" or "online"
	PaymentDate        time.Time `json:"payment_date"`
	ProcessedByUserID  int       `json:"processed_by_user_id"`
	ProcessedByName    string    `json:"processed_by_name,omitempty"` // Joined from users table
	Notes              string    `json:"notes"`
	CreatedAt          time.Time `json:"created_at"`
}

type CreateRentPaymentRequest struct {
	EntryID          int     `json:"entry_id"`
	FamilyMemberID   *int    `json:"family_member_id,omitempty"`
	FamilyMemberName string  `json:"family_member_name,omitempty"`
	CustomerName     string  `json:"customer_name"`
	CustomerPhone    string  `json:"customer_phone"`
	TotalRent        float64 `json:"total_rent"`
	AmountPaid       float64 `json:"amount_paid"`
	Balance          float64 `json:"balance"`
	PaymentType      string  `json:"payment_type"` // "cash" or "online"
	Notes            string  `json:"notes"`
}
