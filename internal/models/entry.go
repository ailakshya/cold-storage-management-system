package models

import "time"

type Entry struct {
	ID                      int        `json:"id"`
	CustomerID              int        `json:"customer_id"`
	FamilyMemberID          *int       `json:"family_member_id,omitempty"`   // FK to family_members
	FamilyMemberName        string     `json:"family_member_name,omitempty"` // Denormalized for display
	Phone                   string     `json:"phone"`
	Name                    string     `json:"name"`
	Village                 string     `json:"village"`
	SO                      string     `json:"so"` // Son Of / Father's Name
	ExpectedQuantity        int        `json:"expected_quantity"`
	ActualQuantity          int        `json:"actual_quantity"` // Sum of room_entries.quantity for this entry
	ThockCategory           string     `json:"thock_category"`  // 'seed' or 'sell'
	ThockNumber             string     `json:"thock_number"`
	Remark                  string     `json:"remark"` // Variety/varieties (comma-separated): Chipsona 1, Chipsona 3, 3797, S4, etc.
	Status                  string     `json:"status"`                     // 'active', 'transferred'
	TransferredToCustomerID *int       `json:"transferred_to_customer_id"` // If transferred, points to new customer
	TransferredAt           *time.Time `json:"transferred_at"`             // When transfer happened
	CreatedByUserID         int        `json:"created_by_user_id"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// CreateEntryRequest represents the request body for creating an entry
type CreateEntryRequest struct {
	CustomerID       int    `json:"customer_id"`
	FamilyMemberID   *int   `json:"family_member_id,omitempty"` // Optional, auto-created if not provided
	Phone            string `json:"phone"`
	Name             string `json:"name"`
	Village          string `json:"village"`
	SO               string `json:"so"`
	ExpectedQuantity int    `json:"expected_quantity"`
	ThockCategory    string `json:"thock_category"`
	Remark           string `json:"remark"` // Variety/varieties (comma-separated)
}

// UpdateEntryRequest represents the request body for updating an entry
type UpdateEntryRequest struct {
	Name             string `json:"name"`
	Phone            string `json:"phone"`
	Village          string `json:"village"`
	SO               string `json:"so"`
	ExpectedQuantity int    `json:"expected_quantity"`
	Remark           string `json:"remark"`
	ThockCategory    string `json:"thock_category"`
}

// ReassignEntryRequest represents the request body for reassigning an entry to a different customer
type ReassignEntryRequest struct {
	NewCustomerID int `json:"new_customer_id"`
}
