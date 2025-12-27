package models

import "time"

// DebtRequestStatus represents the status of a debt request
type DebtRequestStatus string

const (
	DebtRequestStatusPending  DebtRequestStatus = "pending"  // Awaiting admin approval
	DebtRequestStatusApproved DebtRequestStatus = "approved" // Admin approved, can create gate pass
	DebtRequestStatusRejected DebtRequestStatus = "rejected" // Admin rejected
	DebtRequestStatusExpired  DebtRequestStatus = "expired"  // 24hr timeout
	DebtRequestStatusUsed     DebtRequestStatus = "used"     // Gate pass created using this approval
)

// DebtRequest represents a request for item withdrawal when customer has outstanding balance
type DebtRequest struct {
	ID                  int               `json:"id"`
	CustomerPhone       string            `json:"customer_phone"`
	CustomerName        string            `json:"customer_name"`
	CustomerSO          string            `json:"customer_so"` // S/O (Son Of / Father's Name)
	ThockNumber         string            `json:"thock_number"`
	RequestedQuantity   int               `json:"requested_quantity"`
	CurrentBalance      float64           `json:"current_balance"` // How much customer owes at time of request
	RequestedByUserID   int               `json:"requested_by_user_id"`
	RequestedByName     string            `json:"requested_by_name"`
	Status              DebtRequestStatus `json:"status"`
	ApprovedByUserID    *int              `json:"approved_by_user_id"`
	ApprovedByName      string            `json:"approved_by_name"`
	ApprovedAt          *time.Time        `json:"approved_at"`
	RejectionReason     string            `json:"rejection_reason"`
	GatePassID          *int              `json:"gate_pass_id"` // Linked gate pass after approval is used
	CreatedAt           time.Time         `json:"created_at"`
	ExpiresAt           *time.Time        `json:"expires_at"` // Request expires after 24 hours
}

// CreateDebtRequestRequest is used when creating a new debt request
type CreateDebtRequestRequest struct {
	CustomerPhone     string  `json:"customer_phone" validate:"required"`
	CustomerName      string  `json:"customer_name" validate:"required"`
	CustomerSO        string  `json:"customer_so"`
	ThockNumber       string  `json:"thock_number" validate:"required"`
	RequestedQuantity int     `json:"requested_quantity" validate:"required,gt=0"`
	CurrentBalance    float64 `json:"current_balance" validate:"required,gt=0"`
}

// ApproveDebtRequestRequest is used when approving a debt request
type ApproveDebtRequestRequest struct {
	Notes string `json:"notes"`
}

// RejectDebtRequestRequest is used when rejecting a debt request
type RejectDebtRequestRequest struct {
	RejectionReason string `json:"rejection_reason" validate:"required"`
}

// DebtRequestFilter is used for filtering debt requests
type DebtRequestFilter struct {
	CustomerPhone string            `json:"customer_phone"`
	ThockNumber   string            `json:"thock_number"`
	Status        DebtRequestStatus `json:"status"`
	RequestedBy   int               `json:"requested_by"`
	StartDate     *time.Time        `json:"start_date"`
	EndDate       *time.Time        `json:"end_date"`
	Limit         int               `json:"limit"`
	Offset        int               `json:"offset"`
}

// DebtorSummary provides summary for customers with outstanding balance
type DebtorSummary struct {
	CustomerPhone    string     `json:"customer_phone"`
	CustomerName     string     `json:"customer_name"`
	CustomerSO       string     `json:"customer_so"`
	CurrentBalance   float64    `json:"current_balance"`
	TotalCharged     float64    `json:"total_charged"`
	TotalPaid        float64    `json:"total_paid"`
	LastPaymentDate  *time.Time `json:"last_payment_date"`
	PendingDebtReqs  int        `json:"pending_debt_requests"`
	ApprovedDebtReqs int        `json:"approved_debt_requests"`
}

// PendingDebtRequestSummary for admin dashboard
type PendingDebtRequestSummary struct {
	TotalPending     int     `json:"total_pending"`
	TotalAmount      float64 `json:"total_amount"` // Sum of current_balance for pending requests
	OldestRequestAge int     `json:"oldest_request_age_hours"`
}
