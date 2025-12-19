package models

import (
	"encoding/json"
	"time"
)

// SeasonRequest represents a new season request requiring dual admin approval
type SeasonRequest struct {
	ID                 int              `json:"id"`
	Status             string           `json:"status"` // pending, approved, rejected, completed, failed
	InitiatedByUserID  int              `json:"initiated_by_user_id"`
	InitiatedAt        time.Time        `json:"initiated_at"`
	ApprovedByUserID   *int             `json:"approved_by_user_id,omitempty"`
	ApprovedAt         *time.Time       `json:"approved_at,omitempty"`
	ArchiveLocation    string           `json:"archive_location,omitempty"`
	RecordsArchived    *json.RawMessage `json:"records_archived,omitempty"`
	ErrorMessage       string           `json:"error_message,omitempty"`
	SeasonName         string           `json:"season_name,omitempty"`
	Notes              string           `json:"notes,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`

	// Joined fields
	InitiatedByName string `json:"initiated_by_name,omitempty"`
	ApprovedByName  string `json:"approved_by_name,omitempty"`
}

// InitiateSeasonRequest is the request body for initiating a new season
type InitiateSeasonRequest struct {
	SeasonName string `json:"season_name"`
	Notes      string `json:"notes,omitempty"`
	Password   string `json:"password"` // Admin must re-enter password
}

// ApproveSeasonRequest is the request body for approving a season request
type ApproveSeasonRequest struct {
	Password string `json:"password"` // Approving admin must enter password
}

// RejectSeasonRequest is the request body for rejecting a season request
type RejectSeasonRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RecordsArchivedSummary tracks how many records were archived from each table
type RecordsArchivedSummary struct {
	Entries        int `json:"entries"`
	RoomEntries    int `json:"room_entries"`
	EntryEvents    int `json:"entry_events"`
	GatePasses     int `json:"gate_passes"`
	GatePassPickups int `json:"gate_pass_pickups"`
	RentPayments   int `json:"rent_payments"`
	Invoices       int `json:"invoices"`
	CustomerOTPs   int `json:"customer_otps"`
	CustomersReset int `json:"customers_reset"`
	NodeMetrics    int `json:"node_metrics"`
	PostgresMetrics int `json:"postgres_metrics"`
	APIRequestLogs int `json:"api_request_logs"`
}
