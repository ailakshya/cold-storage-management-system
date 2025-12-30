package repositories

import (
	"context"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type FamilyMemberRepository struct {
	DB *pgxpool.Pool
}

func NewFamilyMemberRepository(db *pgxpool.Pool) *FamilyMemberRepository {
	return &FamilyMemberRepository{DB: db}
}

// Create creates a new family member
func (r *FamilyMemberRepository) Create(ctx context.Context, fm *models.FamilyMember) error {
	return r.DB.QueryRow(ctx,
		`INSERT INTO family_members (customer_id, name, relation, is_default)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		fm.CustomerID, fm.Name, fm.Relation, fm.IsDefault,
	).Scan(&fm.ID, &fm.CreatedAt, &fm.UpdatedAt)
}

// Get retrieves a family member by ID
func (r *FamilyMemberRepository) Get(ctx context.Context, id int) (*models.FamilyMember, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT fm.id, fm.customer_id, fm.name, fm.relation, fm.is_default,
		 fm.created_at, fm.updated_at,
		 (SELECT COUNT(*) FROM entries e WHERE e.family_member_id = fm.id) as entry_count
		 FROM family_members fm
		 WHERE fm.id = $1`, id)

	var fm models.FamilyMember
	err := row.Scan(&fm.ID, &fm.CustomerID, &fm.Name, &fm.Relation, &fm.IsDefault,
		&fm.CreatedAt, &fm.UpdatedAt, &fm.EntryCount)
	return &fm, err
}

// ListByCustomer returns all family members for a customer
func (r *FamilyMemberRepository) ListByCustomer(ctx context.Context, customerID int) ([]models.FamilyMember, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT fm.id, fm.customer_id, fm.name, fm.relation, fm.is_default,
		 fm.created_at, fm.updated_at,
		 (SELECT COUNT(*) FROM entries e WHERE e.family_member_id = fm.id) as entry_count
		 FROM family_members fm
		 WHERE fm.customer_id = $1
		 ORDER BY fm.is_default DESC, fm.name ASC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.FamilyMember
	for rows.Next() {
		var fm models.FamilyMember
		err := rows.Scan(&fm.ID, &fm.CustomerID, &fm.Name, &fm.Relation, &fm.IsDefault,
			&fm.CreatedAt, &fm.UpdatedAt, &fm.EntryCount)
		if err != nil {
			return nil, err
		}
		members = append(members, fm)
	}
	return members, rows.Err()
}

// Update updates a family member
func (r *FamilyMemberRepository) Update(ctx context.Context, fm *models.FamilyMember) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE family_members
		 SET name = $1, relation = $2, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $3`,
		fm.Name, fm.Relation, fm.ID)
	return err
}

// Delete deletes a family member (entries will have family_member_id set to NULL)
func (r *FamilyMemberRepository) Delete(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM family_members WHERE id = $1`, id)
	return err
}

// GetByCustomerAndName finds a family member by customer ID and name
func (r *FamilyMemberRepository) GetByCustomerAndName(ctx context.Context, customerID int, name string) (*models.FamilyMember, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT fm.id, fm.customer_id, fm.name, fm.relation, fm.is_default,
		 fm.created_at, fm.updated_at,
		 (SELECT COUNT(*) FROM entries e WHERE e.family_member_id = fm.id) as entry_count
		 FROM family_members fm
		 WHERE fm.customer_id = $1 AND fm.name = $2`, customerID, name)

	var fm models.FamilyMember
	err := row.Scan(&fm.ID, &fm.CustomerID, &fm.Name, &fm.Relation, &fm.IsDefault,
		&fm.CreatedAt, &fm.UpdatedAt, &fm.EntryCount)
	return &fm, err
}

// GetOrCreateDefault gets or creates the default "Self" family member for a customer
func (r *FamilyMemberRepository) GetOrCreateDefault(ctx context.Context, customerID int, customerName string) (*models.FamilyMember, error) {
	// First try to find existing default member
	row := r.DB.QueryRow(ctx,
		`SELECT fm.id, fm.customer_id, fm.name, fm.relation, fm.is_default,
		 fm.created_at, fm.updated_at, 0 as entry_count
		 FROM family_members fm
		 WHERE fm.customer_id = $1 AND fm.is_default = true`, customerID)

	var fm models.FamilyMember
	err := row.Scan(&fm.ID, &fm.CustomerID, &fm.Name, &fm.Relation, &fm.IsDefault,
		&fm.CreatedAt, &fm.UpdatedAt, &fm.EntryCount)
	if err == nil {
		return &fm, nil
	}

	// Try to find by customer name
	row = r.DB.QueryRow(ctx,
		`SELECT fm.id, fm.customer_id, fm.name, fm.relation, fm.is_default,
		 fm.created_at, fm.updated_at, 0 as entry_count
		 FROM family_members fm
		 WHERE fm.customer_id = $1 AND fm.name = $2`, customerID, customerName)

	err = row.Scan(&fm.ID, &fm.CustomerID, &fm.Name, &fm.Relation, &fm.IsDefault,
		&fm.CreatedAt, &fm.UpdatedAt, &fm.EntryCount)
	if err == nil {
		return &fm, nil
	}

	// Create new default family member
	fm = models.FamilyMember{
		CustomerID: customerID,
		Name:       customerName,
		Relation:   "Self",
		IsDefault:  true,
	}
	err = r.Create(ctx, &fm)
	if err != nil {
		return nil, err
	}
	return &fm, nil
}

// GetOrCreateByName gets or creates a family member by name for a customer
func (r *FamilyMemberRepository) GetOrCreateByName(ctx context.Context, customerID int, name string, customerName string) (*models.FamilyMember, error) {
	// First try to find existing member with this name
	fm, err := r.GetByCustomerAndName(ctx, customerID, name)
	if err == nil {
		return fm, nil
	}

	// Determine relation
	relation := "Other"
	isDefault := false
	if name == customerName {
		relation = "Self"
		isDefault = true
	}

	// Create new family member
	newFM := &models.FamilyMember{
		CustomerID: customerID,
		Name:       name,
		Relation:   relation,
		IsDefault:  isDefault,
	}
	err = r.Create(ctx, newFM)
	if err != nil {
		return nil, err
	}
	return newFM, nil
}

// CountByCustomer returns the count of family members for a customer
func (r *FamilyMemberRepository) CountByCustomer(ctx context.Context, customerID int) (int, error) {
	var count int
	err := r.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM family_members WHERE customer_id = $1`, customerID).Scan(&count)
	return count, err
}
