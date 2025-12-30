package models

import "time"

// LedgerEntryType represents the type of ledger entry
type LedgerEntryType string

const (
	LedgerEntryTypeCharge        LedgerEntryType = "CHARGE"         // Rent charged for stored items
	LedgerEntryTypePayment       LedgerEntryType = "PAYMENT"        // Customer payment received (cash)
	LedgerEntryTypeCredit        LedgerEntryType = "CREDIT"         // Discount/adjustment given
	LedgerEntryTypeRefund        LedgerEntryType = "REFUND"         // Money returned to customer
	LedgerEntryTypeDebtApproval  LedgerEntryType = "DEBT_APPROVAL"  // Record of admin approving item out on credit
	LedgerEntryTypeOnlinePayment LedgerEntryType = "ONLINE_PAYMENT" // Online payment via Razorpay (includes UTR)
)

// LedgerEntry represents a single entry in the accounting ledger
type LedgerEntry struct {
	ID              int             `json:"id"`
	CustomerPhone   string          `json:"customer_phone"`
	CustomerName    string          `json:"customer_name"`
	CustomerSO      string          `json:"customer_so"` // S/O (Son Of / Father's Name)
	EntryType       LedgerEntryType `json:"entry_type"`
	Description     string          `json:"description"`
	Debit           float64         `json:"debit"`           // Money owed (increases balance)
	Credit          float64         `json:"credit"`          // Money paid/credited (decreases balance)
	RunningBalance  float64         `json:"running_balance"` // Balance after this entry
	ReferenceID     *int            `json:"reference_id"`    // Links to entry_id, payment_id, gate_pass_id, debt_request_id
	ReferenceType   string          `json:"reference_type"`  // 'entry', 'payment', 'gate_pass', 'debt_request'
	CreatedByUserID int             `json:"created_by_user_id"`
	CreatedByName   string          `json:"created_by_name"`
	CreatedAt       time.Time       `json:"created_at"`
	Notes           string          `json:"notes"`
}

// CreateLedgerEntryRequest is used when creating a new ledger entry
type CreateLedgerEntryRequest struct {
	CustomerPhone   string          `json:"customer_phone" validate:"required"`
	CustomerName    string          `json:"customer_name" validate:"required"`
	CustomerSO      string          `json:"customer_so"`
	EntryType       LedgerEntryType `json:"entry_type" validate:"required"`
	Description     string          `json:"description"`
	Debit           float64         `json:"debit"`
	Credit          float64         `json:"credit"`
	ReferenceID     *int            `json:"reference_id"`
	ReferenceType   string          `json:"reference_type"`
	CreatedByUserID int             `json:"created_by_user_id" validate:"required"`
	Notes           string          `json:"notes"`
}

// LedgerSummary provides summary statistics for a customer
type LedgerSummary struct {
	CustomerPhone  string  `json:"customer_phone"`
	CustomerName   string  `json:"customer_name"`
	CustomerSO     string  `json:"customer_so"`
	TotalDebit     float64 `json:"total_debit"`   // Total charges
	TotalCredit    float64 `json:"total_credit"`  // Total payments + credits
	CurrentBalance float64 `json:"current_balance"` // Debit - Credit
	EntryCount     int     `json:"entry_count"`
}

// LedgerFilter is used for filtering ledger entries
type LedgerFilter struct {
	CustomerPhone string          `json:"customer_phone"`
	EntryType     LedgerEntryType `json:"entry_type"`
	StartDate     *time.Time      `json:"start_date"`
	EndDate       *time.Time      `json:"end_date"`
	Limit         int             `json:"limit"`
	Offset        int             `json:"offset"`
}

// AuditEntry is used for the audit trail display
type AuditEntry struct {
	ID              int             `json:"id"`
	Date            time.Time       `json:"date"`
	CustomerPhone   string          `json:"customer_phone"`
	CustomerName    string          `json:"customer_name"`
	CustomerSO      string          `json:"customer_so"`
	EntryType       LedgerEntryType `json:"entry_type"`
	Description     string          `json:"description"`
	Debit           float64         `json:"debit"`
	Credit          float64         `json:"credit"`
	RunningBalance  float64         `json:"running_balance"`
	CreatedByName   string          `json:"created_by_name"`
	PaymentType     string          `json:"payment_type"`
	Remarks         string          `json:"remarks"`
}
