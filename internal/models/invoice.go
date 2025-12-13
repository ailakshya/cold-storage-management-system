package models

import "time"

// Invoice represents a generated invoice
type Invoice struct {
	ID            int       `json:"id"`
	InvoiceNumber string    `json:"invoice_number"`
	CustomerID    *int      `json:"customer_id"`
	EmployeeID    *int      `json:"employee_id"`
	TotalAmount   float64   `json:"total_amount"`
	ItemsCount    int       `json:"items_count"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// InvoiceItem represents an item included in an invoice
type InvoiceItem struct {
	ID          int       `json:"id"`
	InvoiceID   int       `json:"invoice_id"`
	EntryID     *int      `json:"entry_id"`
	TruckNumber string    `json:"truck_number"`
	Quantity    int       `json:"quantity"`
	Rate        float64   `json:"rate"`
	Amount      float64   `json:"amount"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateInvoiceRequest represents the request to create an invoice
type CreateInvoiceRequest struct {
	CustomerID  int            `json:"customer_id"`
	EmployeeID  int            `json:"employee_id"`
	TotalAmount float64        `json:"total_amount"`
	Notes       string         `json:"notes"`
	Items       []InvoiceItem  `json:"items"`
}

// InvoiceWithDetails includes customer and employee details
type InvoiceWithDetails struct {
	Invoice
	CustomerName string         `json:"customer_name"`
	EmployeeName string         `json:"employee_name"`
	Items        []InvoiceItem  `json:"items"`
}
