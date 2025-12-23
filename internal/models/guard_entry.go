package models

import "time"

// GuardEntry represents a preliminary vehicle arrival record logged by a guard
type GuardEntry struct {
	ID                int        `json:"id"`
	TokenNumber       int        `json:"token_number"`    // Daily token number for colored token
	CustomerName      string     `json:"customer_name"`
	SO                string     `json:"so"`              // Son Of / Father name (optional)
	Village           string     `json:"village"`
	Mobile            string     `json:"mobile"`
	DriverNo          string     `json:"driver_no"`
	ArrivalTime       time.Time  `json:"arrival_time"`
	SeedQuantity      int        `json:"seed_quantity"`   // Number of seed bags
	SellQuantity      int        `json:"sell_quantity"`   // Number of sell bags
	Remarks           string     `json:"remarks"`
	Status            string     `json:"status"`          // 'pending' or 'processed'
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
	SO           string `json:"so"`            // Son Of / Father name (optional)
	Village      string `json:"village"`
	Mobile       string `json:"mobile"`
	DriverNo     string `json:"driver_no"`
	SeedQuantity int    `json:"seed_quantity"` // Number of seed bags
	SellQuantity int    `json:"sell_quantity"` // Number of sell bags
	Remarks      string `json:"remarks"`
}

// TotalQuantity returns the total bags (seed + sell)
func (g *GuardEntry) TotalQuantity() int {
	return g.SeedQuantity + g.SellQuantity
}

// Category returns the category based on quantities
func (g *GuardEntry) Category() string {
	if g.SeedQuantity > 0 && g.SellQuantity > 0 {
		return "both"
	} else if g.SeedQuantity > 0 {
		return "seed"
	} else if g.SellQuantity > 0 {
		return "sell"
	}
	return ""
}
