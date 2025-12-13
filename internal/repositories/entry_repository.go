package repositories

import (
	"context"
	"fmt"

	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EntryRepository struct {
	DB *pgxpool.Pool
}

func NewEntryRepository(db *pgxpool.Pool) *EntryRepository {
	return &EntryRepository{DB: db}
}

func (r *EntryRepository) Create(ctx context.Context, e *models.Entry) error {
	// Count existing entries for this category to get next sequential number
	var count int
	err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM entries WHERE truck_category=$1`,
		e.TruckCategory).Scan(&count)
	if err != nil {
		return err
	}

	// Generate truck number: NO/QUANTITY format
	// SEED: 001-599 range (sequential from 1)
	// SELL: 600-1500 range (sequential from 600)
	var truckNumber string
	if e.TruckCategory == "seed" {
		nextNumber := count + 1
		truckNumber = fmt.Sprintf("%03d/%d", nextNumber, e.ExpectedQuantity)
	} else if e.TruckCategory == "sell" {
		nextNumber := 600 + count
		truckNumber = fmt.Sprintf("%d/%d", nextNumber, e.ExpectedQuantity)
	}
	e.TruckNumber = truckNumber

	return r.DB.QueryRow(ctx,
		`INSERT INTO entries(customer_id, phone, name, village, so, expected_quantity, truck_category, truck_number, created_by_user_id)
         VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
         RETURNING id, created_at, updated_at`,
		e.CustomerID, e.Phone, e.Name, e.Village, e.SO, e.ExpectedQuantity, e.TruckCategory, e.TruckNumber, e.CreatedByUserID,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (r *EntryRepository) Get(ctx context.Context, id int) (*models.Entry, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, customer_id, phone, name, village, so, expected_quantity, truck_category, truck_number, created_by_user_id, created_at, updated_at
         FROM entries WHERE id=$1`, id)

	var entry models.Entry
	err := row.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
		&entry.ExpectedQuantity, &entry.TruckCategory, &entry.TruckNumber, &entry.CreatedByUserID,
		&entry.CreatedAt, &entry.UpdatedAt)
	return &entry, err
}

func (r *EntryRepository) List(ctx context.Context) ([]*models.Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, customer_id, phone, name, village, so, expected_quantity, truck_category, truck_number, created_by_user_id, created_at, updated_at
         FROM entries ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.TruckCategory, &entry.TruckNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *EntryRepository) ListByCustomer(ctx context.Context, customerID int) ([]*models.Entry, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, customer_id, phone, name, village, so, expected_quantity, truck_category, truck_number, created_by_user_id, created_at, updated_at
         FROM entries WHERE customer_id=$1 ORDER BY created_at DESC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.TruckCategory, &entry.TruckNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

func (r *EntryRepository) GetCountByCategory(ctx context.Context, category string) (int, error) {
	var count int
	err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM entries WHERE truck_category=$1`,
		category).Scan(&count)
	return count, err
}

func (r *EntryRepository) ListUnassigned(ctx context.Context) ([]*models.Entry, error) {
	// Get entries that don't have a room entry yet
	rows, err := r.DB.Query(ctx,
		`SELECT e.id, e.customer_id, e.phone, e.name, e.village, e.so, e.expected_quantity,
		        e.truck_category, e.truck_number, e.created_by_user_id, e.created_at, e.updated_at
         FROM entries e
         LEFT JOIN room_entries re ON e.id = re.entry_id
         WHERE re.id IS NULL
         ORDER BY e.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.Entry
	for rows.Next() {
		var entry models.Entry
		err := rows.Scan(&entry.ID, &entry.CustomerID, &entry.Phone, &entry.Name, &entry.Village, &entry.SO,
			&entry.ExpectedQuantity, &entry.TruckCategory, &entry.TruckNumber, &entry.CreatedByUserID,
			&entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}
