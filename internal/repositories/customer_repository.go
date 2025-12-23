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
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers WHERE id=$1`, id)

	var customer models.Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *CustomerRepository) GetByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers WHERE phone=$1`, phone)

	var customer models.Customer
	err := row.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
		&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
	return &customer, err
}

func (r *CustomerRepository) List(ctx context.Context) ([]*models.Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*models.Customer
	for rows.Next() {
		var customer models.Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Phone, &customer.SO, &customer.Village,
			&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}

func (r *CustomerRepository) Update(ctx context.Context, c *models.Customer) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE customers SET name=$1, phone=$2, so=$3, village=$4, address=$5, updated_at=CURRENT_TIMESTAMP
         WHERE id=$6`,
		c.Name, c.Phone, c.SO, c.Village, c.Address, c.ID)
	return err
}

func (r *CustomerRepository) Delete(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM customers WHERE id=$1`, id)
	return err
}

// FuzzySearchByPhone searches customers by phone number (fuzzy match)
func (r *CustomerRepository) FuzzySearchByPhone(ctx context.Context, phone string) ([]*models.Customer, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, phone, COALESCE(so, '') as so, village, address, created_at, updated_at
         FROM customers
         WHERE phone LIKE $1
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
			&customer.Address, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, &customer)
	}
	return customers, nil
}
