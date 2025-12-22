package models

import "time"

// GuardEntry represents a preliminary vehicle arrival record logged by a guard
type GuardEntry struct {
	ID                int        `json:"id"`
	CustomerName      string     `json:"customer_name"`
	Village           string     `json:"village"`
	Mobile            string     `json:"mobile"`
	DriverNo          string     `json:"driver_no"`
	ArrivalTime       time.Time  `json:"arrival_time"`
	Category          string     `json:"category"` // 'seed', 'sell', or 'both'
	Remarks           string     `json:"remarks"`
	Status            string     `json:"status"` // 'pending' or 'processed'
	CreatedByUserID   int        `json:"created_by_user_id"`
	ProcessedByUserID *int       `json:"processed_by_user_id,omitempty"`
	ProcessedAt       *time.Time `json:"processed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Joined fields - populated by certain queries
	CreatedByUserName   string `json:"created_by_user_name,omitempty"`
	ProcessedByUserName string `json:"processed_by_user_name,omitempty"`
}

// CreateGuardEntryRequest represents the request body for creating a guard entry
type CreateGuardEntryRequest struct {
	CustomerName string `json:"customer_name"`
	Village      string `json:"village"`
	Mobile       string `json:"mobile"`
	DriverNo     string `json:"driver_no"`
	Category     string `json:"category"` // 'seed', 'sell', or 'both'
	Remarks      string `json:"remarks"`
}
