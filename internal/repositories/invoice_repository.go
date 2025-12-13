package repositories

import (
	"context"
	"fmt"
	"time"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InvoiceRepository struct {
	DB *pgxpool.Pool
}

func NewInvoiceRepository(db *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{DB: db}
}

// GenerateInvoiceNumber generates a unique invoice number
func (r *InvoiceRepository) GenerateInvoiceNumber(ctx context.Context) (string, error) {
	now := time.Now()
	// Format: INV-YYYYMMDD-NNNN (e.g., INV-20231213-0001)
	datePrefix := now.Format("20060102")

	// Get count of invoices created today
	var count int
	err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM invoices WHERE invoice_number LIKE $1`,
		fmt.Sprintf("INV-%s-%%", datePrefix),
	).Scan(&count)

	if err != nil {
		return "", err
	}

	// Increment and format with leading zeros
	invoiceNumber := fmt.Sprintf("INV-%s-%04d", datePrefix, count+1)
	return invoiceNumber, nil
}

// Create creates a new invoice with items
func (r *InvoiceRepository) Create(ctx context.Context, invoice *models.Invoice, items []models.InvoiceItem) error {
	// Start transaction
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Generate invoice number if not provided
	if invoice.InvoiceNumber == "" {
		invoiceNumber, err := r.GenerateInvoiceNumber(ctx)
		if err != nil {
			return err
		}
		invoice.InvoiceNumber = invoiceNumber
	}

	// Insert invoice
	err = tx.QueryRow(ctx,
		`INSERT INTO invoices(invoice_number, customer_id, employee_id, total_amount, items_count, notes)
		 VALUES($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at, updated_at`,
		invoice.InvoiceNumber, invoice.CustomerID, invoice.EmployeeID,
		invoice.TotalAmount, len(items), invoice.Notes,
	).Scan(&invoice.ID, &invoice.CreatedAt, &invoice.UpdatedAt)

	if err != nil {
		return err
	}

	// Insert invoice items
	for _, item := range items {
		_, err = tx.Exec(ctx,
			`INSERT INTO invoice_items(invoice_id, entry_id, truck_number, quantity, rate, amount)
			 VALUES($1, $2, $3, $4, $5, $6)`,
			invoice.ID, item.EntryID, item.TruckNumber, item.Quantity, item.Rate, item.Amount,
		)
		if err != nil {
			return err
		}
	}

	// Commit transaction
	return tx.Commit(ctx)
}

// Get retrieves an invoice by ID with items
func (r *InvoiceRepository) Get(ctx context.Context, id int) (*models.InvoiceWithDetails, error) {
	// Get invoice
	var invoice models.InvoiceWithDetails
	err := r.DB.QueryRow(ctx,
		`SELECT i.id, i.invoice_number, i.customer_id, i.employee_id, i.total_amount,
		        i.items_count, i.notes, i.created_at, i.updated_at,
		        COALESCE(c.name, '') as customer_name, COALESCE(u.name, '') as employee_name
		 FROM invoices i
		 LEFT JOIN customers c ON i.customer_id = c.id
		 LEFT JOIN users u ON i.employee_id = u.id
		 WHERE i.id = $1`, id,
	).Scan(&invoice.ID, &invoice.InvoiceNumber, &invoice.CustomerID, &invoice.EmployeeID,
		&invoice.TotalAmount, &invoice.ItemsCount, &invoice.Notes, &invoice.CreatedAt,
		&invoice.UpdatedAt, &invoice.CustomerName, &invoice.EmployeeName)

	if err != nil {
		return nil, err
	}

	// Get invoice items
	rows, err := r.DB.Query(ctx,
		`SELECT id, invoice_id, entry_id, truck_number, quantity, rate, amount, created_at
		 FROM invoice_items WHERE invoice_id = $1`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.InvoiceItem
	for rows.Next() {
		var item models.InvoiceItem
		err := rows.Scan(&item.ID, &item.InvoiceID, &item.EntryID, &item.TruckNumber,
			&item.Quantity, &item.Rate, &item.Amount, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	invoice.Items = items
	return &invoice, nil
}

// GetByInvoiceNumber retrieves an invoice by invoice number
func (r *InvoiceRepository) GetByInvoiceNumber(ctx context.Context, invoiceNumber string) (*models.InvoiceWithDetails, error) {
	var invoice models.InvoiceWithDetails
	err := r.DB.QueryRow(ctx,
		`SELECT i.id, i.invoice_number, i.customer_id, i.employee_id, i.total_amount,
		        i.items_count, i.notes, i.created_at, i.updated_at,
		        COALESCE(c.name, '') as customer_name, COALESCE(u.name, '') as employee_name
		 FROM invoices i
		 LEFT JOIN customers c ON i.customer_id = c.id
		 LEFT JOIN users u ON i.employee_id = u.id
		 WHERE i.invoice_number = $1`, invoiceNumber,
	).Scan(&invoice.ID, &invoice.InvoiceNumber, &invoice.CustomerID, &invoice.EmployeeID,
		&invoice.TotalAmount, &invoice.ItemsCount, &invoice.Notes, &invoice.CreatedAt,
		&invoice.UpdatedAt, &invoice.CustomerName, &invoice.EmployeeName)

	if err != nil {
		return nil, err
	}

	// Get invoice items
	rows, err := r.DB.Query(ctx,
		`SELECT id, invoice_id, entry_id, truck_number, quantity, rate, amount, created_at
		 FROM invoice_items WHERE invoice_id = $1`, invoice.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.InvoiceItem
	for rows.Next() {
		var item models.InvoiceItem
		err := rows.Scan(&item.ID, &item.InvoiceID, &item.EntryID, &item.TruckNumber,
			&item.Quantity, &item.Rate, &item.Amount, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	invoice.Items = items
	return &invoice, nil
}

// List returns all invoices
func (r *InvoiceRepository) List(ctx context.Context) ([]*models.InvoiceWithDetails, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT i.id, i.invoice_number, i.customer_id, i.employee_id, i.total_amount,
		        i.items_count, i.notes, i.created_at, i.updated_at,
		        COALESCE(c.name, '') as customer_name, COALESCE(u.name, '') as employee_name
		 FROM invoices i
		 LEFT JOIN customers c ON i.customer_id = c.id
		 LEFT JOIN users u ON i.employee_id = u.id
		 ORDER BY i.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []*models.InvoiceWithDetails
	for rows.Next() {
		var invoice models.InvoiceWithDetails
		err := rows.Scan(&invoice.ID, &invoice.InvoiceNumber, &invoice.CustomerID,
			&invoice.EmployeeID, &invoice.TotalAmount, &invoice.ItemsCount, &invoice.Notes,
			&invoice.CreatedAt, &invoice.UpdatedAt, &invoice.CustomerName, &invoice.EmployeeName)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, &invoice)
	}

	return invoices, nil
}

// GetByCustomer returns all invoices for a customer
func (r *InvoiceRepository) GetByCustomer(ctx context.Context, customerID int) ([]*models.Invoice, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, invoice_number, customer_id, employee_id, total_amount, items_count,
		        notes, created_at, updated_at
		 FROM invoices WHERE customer_id = $1 ORDER BY created_at DESC`, customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []*models.Invoice
	for rows.Next() {
		var invoice models.Invoice
		err := rows.Scan(&invoice.ID, &invoice.InvoiceNumber, &invoice.CustomerID,
			&invoice.EmployeeID, &invoice.TotalAmount, &invoice.ItemsCount, &invoice.Notes,
			&invoice.CreatedAt, &invoice.UpdatedAt)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, &invoice)
	}

	return invoices, nil
}
