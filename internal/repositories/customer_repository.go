package repositories

import (
	"context"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository struct {
	DB *pgxpool.Pool
}

func NewCustomerRepository(db *pgxpool.Pool) *CustomerRepository {
	return &CustomerRepository{DB: db}
}

func (r *CustomerRepository) Create(ctx context.Context, c *models.Customer) error {
	return r.DB.QueryRow(ctx,
		`INSERT INTO customers(name, phone, so, village, address)
         VALUES($1, $2, $3, $4, $5)
         RETURNING id, created_at, updated_at`,
		c.Name, c.Phone, c.SO, c.Village, c.Address,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *CustomerRepository) Get(ctx context.Context, id int) (*models.Customer, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, COALESCE(village, '') as village, COALESCE(address, '') as address,
		 COALESCE(status, 'active') as status, merged_into_customer_id, merged_at,
		 created_at, updated_at
         FROM customers WHERE id=$1`, id)

	var customer models.Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.Status, &customer.MergedIntoCustomerID, &customer.MergedAt,
		&customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *CustomerRepository) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, COALESCE(village, '') as village, COALESCE(address, '') as address,
		 COALESCE(status, 'active') as status, merged_into_customer_id, merged_at,
		 created_at, updated_at
         FROM customers WHERE phone=$1`, phone)

	var customer models.Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.Status, &customer.MergedIntoCustomerID, &customer.MergedAt,
		&customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *CustomerRepository) List(ctx context.Context) ([]*models.Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, COALESCE(village, '') as village, COALESCE(address, '') as address,
		 COALESCE(status, 'active') as status, merged_into_customer_id, merged_at,
		 created_at, updated_at
         FROM customers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*models.Customer
	for rows.Next() {
		var customer models.Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.Status, &customer.MergedIntoCustomerID, &customer.MergedAt,
			&customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}

func (r *CustomerRepository) Update(ctx context.Context, c *models.Customer) error {
	// Update customer
	_, err := r.DB.Exec(ctx,
		`UPDATE customers SET name=$1, phone=$2, so=$3, village=$4, address=$5, updated_at=CURRENT_TIMESTAMP
         WHERE id=$6`,
		c.Name, c.Phone, c.SO, c.Village, c.Address, c.ID)
	if err != nil {
		return err
	}

	// Also update all entries for this customer to sync the denormalized data
	_, err = r.DB.Exec(ctx,
		`UPDATE entries SET name=$1, phone=$2, so=$3, village=$4, updated_at=CURRENT_TIMESTAMP
         WHERE customer_id=$5`,
		c.Name, c.Phone, c.SO, c.Village, c.ID)
	return err
}

func (r *CustomerRepository) Delete(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM customers WHERE id=$1`, id)
	return err
}

// GetEntryCount returns the number of entries for a customer
func (r *CustomerRepository) GetEntryCount(ctx context.Context, customerID int) (int, error) {
	var count int
	err := r.DB.QueryRow(ctx, `SELECT COUNT(*) FROM entries WHERE customer_id=$1`, customerID).Scan(&count)
	return count, err
}

// MergeCustomers moves all entries and payments from source customer to target customer
// Instead of deleting, marks source customer as 'merged' for audit trail
// Returns the number of entries moved, payments moved, and detailed info
func (r *CustomerRepository) MergeCustomers(ctx context.Context, sourceID, targetID int, targetName, targetPhone, targetVillage, targetSO string, sourcePhone string) (int, int, *models.MergeDetails, error) {
	// Start transaction
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback(ctx)

	// Collect entry details BEFORE moving (for audit trail)
	entryRows, err := tx.Query(ctx, `
		SELECT id, thock_number, expected_quantity, thock_category
		FROM entries WHERE customer_id=$1
		UNION ALL
		SELECT id, thock_number, expected_quantity, thock_category
		FROM entries WHERE phone=$2 AND customer_id NOT IN (SELECT id FROM customers)`,
		sourceID, sourcePhone)
	if err != nil {
		return 0, 0, nil, err
	}

	var entryDetails []models.MergeEntryDetail
	for entryRows.Next() {
		var e models.MergeEntryDetail
		if err := entryRows.Scan(&e.ID, &e.ThockNumber, &e.ExpectedQuantity, &e.ThockCategory); err != nil {
			entryRows.Close()
			return 0, 0, nil, err
		}
		entryDetails = append(entryDetails, e)
	}
	entryRows.Close()

	// Collect payment details BEFORE moving (for audit trail)
	paymentRows, err := tx.Query(ctx, `
		SELECT id, amount_paid, COALESCE(receipt_number, ''), TO_CHAR(payment_date, 'DD/MM/YYYY')
		FROM rent_payments WHERE customer_phone=$1`,
		sourcePhone)
	if err != nil {
		return 0, 0, nil, err
	}

	var paymentDetails []models.MergePaymentDetail
	for paymentRows.Next() {
		var p models.MergePaymentDetail
		if err := paymentRows.Scan(&p.ID, &p.Amount, &p.ReceiptNumber, &p.PaymentDate); err != nil {
			paymentRows.Close()
			return 0, 0, nil, err
		}
		paymentDetails = append(paymentDetails, p)
	}
	paymentRows.Close()

	entriesMoved := len(entryDetails)
	paymentsMoved := len(paymentDetails)

	// Save original_customer_id before moving entries (for undo functionality)
	_, err = tx.Exec(ctx, `
		UPDATE entries
		SET original_customer_id = customer_id
		WHERE customer_id=$1 AND original_customer_id IS NULL`,
		sourceID)
	if err != nil {
		return 0, 0, nil, err
	}

	// Move all entries from source to target (update customer_id and denormalized fields)
	_, err = tx.Exec(ctx, `
		UPDATE entries
		SET customer_id=$1, name=$2, phone=$3, village=$4, so=$5, updated_at=NOW()
		WHERE customer_id=$6`,
		targetID, targetName, targetPhone, targetVillage, targetSO, sourceID)
	if err != nil {
		return 0, 0, nil, err
	}

	// Also move orphaned entries with source phone
	_, err = tx.Exec(ctx, `
		UPDATE entries
		SET customer_id=$1, name=$2, phone=$3, village=$4, so=$5, updated_at=NOW()
		WHERE phone=$6 AND customer_id NOT IN (SELECT id FROM customers)`,
		targetID, targetName, targetPhone, targetVillage, targetSO, sourcePhone)
	if err != nil {
		return 0, 0, nil, err
	}

	// Transfer all rent payments from source customer to target customer
	_, err = tx.Exec(ctx, `
		UPDATE rent_payments
		SET customer_name=$1, customer_phone=$2
		WHERE customer_phone=$3`,
		targetName, targetPhone, sourcePhone)
	if err != nil {
		return 0, 0, nil, err
	}

	// SOFT DELETE: Mark source customer as merged (don't delete)
	_, err = tx.Exec(ctx, `
		UPDATE customers
		SET status='merged', merged_into_customer_id=$1, merged_at=NOW(), updated_at=NOW()
		WHERE id=$2`,
		targetID, sourceID)
	if err != nil {
		return 0, 0, nil, err
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, nil, err
	}

	mergeDetails := &models.MergeDetails{
		Entries:  entryDetails,
		Payments: paymentDetails,
	}

	return entriesMoved, paymentsMoved, mergeDetails, nil
}

// GetMergedCustomers returns all customers that have been merged
func (r *CustomerRepository) GetMergedCustomers(ctx context.Context) ([]*models.Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address,
		 COALESCE(status, 'active') as status, merged_into_customer_id, merged_at,
		 created_at, updated_at
         FROM customers
         WHERE status = 'merged'
         ORDER BY merged_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*models.Customer
	for rows.Next() {
		var customer models.Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.Status, &customer.MergedIntoCustomerID, &customer.MergedAt,
			&customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}

// UndoMerge fully reverses a merge:
// 1. Moves all entries back to source customer
// 2. Moves specified payments back to source customer (using IDs from merge log)
// 3. Sets source customer status back to active
func (r *CustomerRepository) UndoMerge(ctx context.Context, sourceCustomerID int, paymentIDs []int) error {
	// Start transaction
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get source customer details (for restoring entry denormalized fields)
	var sourceName, sourcePhone, sourceVillage, sourceSO string
	err = tx.QueryRow(ctx, `
		SELECT name, phone, village, COALESCE(so, '')
		FROM customers WHERE id = $1`,
		sourceCustomerID).Scan(&sourceName, &sourcePhone, &sourceVillage, &sourceSO)
	if err != nil {
		return err
	}

	// Move entries back to source customer (using original_customer_id)
	_, err = tx.Exec(ctx, `
		UPDATE entries
		SET customer_id = original_customer_id,
		    name = $1, phone = $2, village = $3, so = $4,
		    updated_at = NOW()
		WHERE original_customer_id = $5`,
		sourceName, sourcePhone, sourceVillage, sourceSO, sourceCustomerID)
	if err != nil {
		return err
	}

	// Move payments back to source customer (only those that were moved during merge)
	if len(paymentIDs) > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE rent_payments
			SET customer_name = $1, customer_phone = $2
			WHERE id = ANY($3)`,
			sourceName, sourcePhone, paymentIDs)
		if err != nil {
			return err
		}
	}

	// Set source customer status back to active
	_, err = tx.Exec(ctx, `
		UPDATE customers
		SET status = 'active', merged_into_customer_id = NULL, merged_at = NULL, updated_at = NOW()
		WHERE id = $1 AND status = 'merged'`,
		sourceCustomerID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// FuzzySearchByPhone searches customers by phone number (fuzzy match)
// Only returns active customers (not merged ones)
func (r *CustomerRepository) FuzzySearchByPhone(ctx context.Context, phone string) ([]*models.Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address,
		 COALESCE(status, 'active') as status, merged_into_customer_id, merged_at,
		 created_at, updated_at
         FROM customers
         WHERE phone LIKE $1 AND (status IS NULL OR status = 'active')
         ORDER BY created_at DESC
         LIMIT 10`,
		"%"+phone+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*models.Customer
	for rows.Next() {
		var customer models.Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.Status, &customer.MergedIntoCustomerID, &customer.MergedAt,
			&customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}
