package models

import "time"

type GatePass struct {
	ID                    int        `json:"id" db:"id"`
	CustomerID            int        `json:"customer_id" db:"customer_id"`
	ThockNumber           string     `json:"thock_number" db:"thock_number"`
	EntryID               *int       `json:"entry_id,omitempty" db:"entry_id"`
	FamilyMemberID        *int       `json:"family_member_id,omitempty" db:"family_member_id"`
	FamilyMemberName      string     `json:"family_member_name,omitempty" db:"family_member_name"`
	RequestedQuantity     int        `json:"requested_quantity" db:"requested_quantity"`
	ApprovedQuantity      *int       `json:"approved_quantity,omitempty" db:"approved_quantity"`
	GateNo                *string    `json:"gate_no,omitempty" db:"gate_no"`
	Status                string     `json:"status" db:"status"`
	PaymentVerified       bool       `json:"payment_verified" db:"payment_verified"`
	PaymentAmount         *float64   `json:"payment_amount,omitempty" db:"payment_amount"`
	IssuedByUserID        *int       `json:"issued_by_user_id,omitempty" db:"issued_by_user_id"`
	ApprovedByUserID      *int       `json:"approved_by_user_id,omitempty" db:"approved_by_user_id"`
	IssuedAt              time.Time  `json:"issued_at" db:"issued_at"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	Remarks               *string    `json:"remarks,omitempty" db:"remarks"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at" db:"updated_at"`
	TotalPickedUp         int        `json:"total_picked_up" db:"total_picked_up"`
	ApprovalExpiresAt     *time.Time `json:"approval_expires_at,omitempty" db:"approval_expires_at"`
	FinalApprovedQuantity *int       `json:"final_approved_quantity,omitempty" db:"final_approved_quantity"`
	CreatedByCustomerID   *int       `json:"created_by_customer_id,omitempty" db:"created_by_customer_id"`
	RequestSource         string     `json:"request_source" db:"request_source"` // "employee" or "customer_portal"
}

type CreateGatePassRequest struct {
	CustomerID        int     `json:"customer_id"`
	ThockNumber       string  `json:"thock_number"`
	EntryID           *int    `json:"entry_id"`
	FamilyMemberID    *int    `json:"family_member_id"`
	FamilyMemberName  string  `json:"family_member_name"`
	RequestedQuantity int     `json:"requested_quantity"`
	PaymentVerified   bool    `json:"payment_verified"`
	PaymentAmount     float64 `json:"payment_amount"`
	Remarks           string  `json:"remarks"`
}

type UpdateGatePassRequest struct {
	ApprovedQuantity int    `json:"approved_quantity"`
	GateNo           string `json:"gate_no"`
	Status           string `json:"status"`
	RequestSource    string `json:"request_source,omitempty"`
	Remarks          string `json:"remarks"`
}

type RecordPickupRequest struct {
	GatePassID      int              `json:"gate_pass_id"`
	PickupQuantity  int              `json:"pickup_quantity"`
	RoomNo          string           `json:"room_no"`
	Floor           string           `json:"floor"`
	Remarks         string           `json:"remarks"`
	GatarBreakdown  []GatarBreakdown `json:"gatar_breakdown,omitempty"`
}

// CreateCustomerGatePassRequest represents a customer's gate pass request
type CreateCustomerGatePassRequest struct {
	ThockNumber       string `json:"thock_number" binding:"required"`
	FamilyMemberID    *int   `json:"family_member_id"`
	FamilyMemberName  string `json:"family_member_name"`
	RequestedQuantity int    `json:"requested_quantity" binding:"required"`
	Remarks           string `json:"remarks"`
}
