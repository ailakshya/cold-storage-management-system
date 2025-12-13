package models

import "time"

type Entry struct {
	ID               int       `json:"id"`
	CustomerID       int       `json:"customer_id"`
	Phone            string    `json:"phone"`
	Name             string    `json:"name"`
	Village          string    `json:"village"`
	SO               string    `json:"so"` // Son Of / Father's Name
	ExpectedQuantity int       `json:"expected_quantity"`
	TruckCategory    string    `json:"truck_category"` // 'seed' or 'sell'
	TruckNumber      string    `json:"truck_number"`
	CreatedByUserID  int       `json:"created_by_user_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateEntryRequest represents the request body for creating an entry
type CreateEntryRequest struct {
	CustomerID       int    `json:"customer_id"`
	Phone            string `json:"phone"`
	Name             string `json:"name"`
	Village          string `json:"village"`
	SO               string `json:"so"`
	ExpectedQuantity int    `json:"expected_quantity"`
	TruckCategory    string `json:"truck_category"`
}
