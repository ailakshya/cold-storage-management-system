package models

import "time"

// GuardEntry represents a preliminary vehicle arrival record logged by a guard
type GuardEntry struct {
	ID                int        `json:"id"`
	TokenNumber       int        `json:"token_number"`    // Daily token number for colored token
	CustomerID        *int       `json:"customer_id,omitempty"` // Linked customer ID (optional)
	FamilyMemberID    *int       `json:"family_member_id,omitempty"` // Linked family member ID (optional)
	CustomerName      string     `json:"customer_name"`
	SO                string     `json:"so"`              // Son Of / Father name (optional)
	Village           string     `json:"village"`
	Mobile            string     `json:"mobile"`
	DriverNo          string     `json:"driver_no"`
	ArrivalTime       time.Time  `json:"arrival_time"`
	SeedQuantity      int        `json:"seed_quantity"`   // Number of seed bags
	SellQuantity      int        `json:"sell_quantity"`   // Number of sell bags
	SeedQty1          int        `json:"seed_qty_1"`      // Individual seed quantity 1
	SeedQty2          int        `json:"seed_qty_2"`      // Individual seed quantity 2
	SeedQty3          int        `json:"seed_qty_3"`      // Individual seed quantity 3
	SeedQty4          int        `json:"seed_qty_4"`      // Individual seed quantity 4
	SellQty1          int        `json:"sell_qty_1"`      // Individual sell quantity 1
	SellQty2          int        `json:"sell_qty_2"`      // Individual sell quantity 2
	SellQty3          int        `json:"sell_qty_3"`      // Individual sell quantity 3
	SellQty4          int        `json:"sell_qty_4"`      // Individual sell quantity 4
	Remarks           string     `json:"remarks"`
	Status            string     `json:"status"`          // 'pending' or 'processed'
	CreatedByUserID   int        `json:"created_by_user_id"`
	ProcessedByUserID *int       `json:"processed_by_user_id,omitempty"`
	ProcessedAt       *time.Time `json:"processed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Partial processing fields
	SeedProcessed   bool       `json:"seed_processed"`
	SellProcessed   bool       `json:"sell_processed"`
	SeedProcessedBy *int       `json:"seed_processed_by,omitempty"`
	SellProcessedBy *int       `json:"sell_processed_by,omitempty"`
	SeedProcessedAt *time.Time `json:"seed_processed_at,omitempty"`
	SellProcessedAt *time.Time `json:"sell_processed_at,omitempty"`

	// Joined fields - populated by certain queries
	CreatedByUserName   string `json:"created_by_user_name,omitempty"`
	ProcessedByUserName string `json:"processed_by_user_name,omitempty"`
}

// CreateGuardEntryRequest represents the request body for creating a guard entry
type CreateGuardEntryRequest struct {
	CustomerID     *int   `json:"customer_id"`     // Linked customer ID (optional - for existing customers)
	FamilyMemberID *int   `json:"family_member_id"` // Linked family member ID (optional)
	CustomerName   string `json:"customer_name"`
	SO             string `json:"so"`            // Son Of / Father name (optional)
	Village        string `json:"village"`
	Mobile         string `json:"mobile"`
	DriverNo       string `json:"driver_no"`
	SeedQuantity   int    `json:"seed_quantity"` // Number of seed bags
	SellQuantity   int    `json:"sell_quantity"` // Number of sell bags
	SeedQty1       int    `json:"seed_qty_1"`    // Individual seed quantity 1
	SeedQty2       int    `json:"seed_qty_2"`    // Individual seed quantity 2
	SeedQty3       int    `json:"seed_qty_3"`    // Individual seed quantity 3
	SeedQty4       int    `json:"seed_qty_4"`    // Individual seed quantity 4
	SellQty1       int    `json:"sell_qty_1"`    // Individual sell quantity 1
	SellQty2       int    `json:"sell_qty_2"`    // Individual sell quantity 2
	SellQty3       int    `json:"sell_qty_3"`    // Individual sell quantity 3
	SellQty4       int    `json:"sell_qty_4"`    // Individual sell quantity 4
	Remarks        string `json:"remarks"`
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
