package repositories

import (
	"context"
	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	DB *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	if u.Role == "" {
		u.Role = "employee" // Default role
	}
	return r.DB.QueryRow(ctx,
		`INSERT INTO users(name, email, password_hash, role, has_accountant_access)
         VALUES($1, $2, $3, $4, $5)
         RETURNING id, created_at, updated_at`,
		u.Name, u.Email, u.PasswordHash, u.Role, u.HasAccountantAccess,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) Get(ctx context.Context, id int) (*models.User, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, email, password_hash, role, has_accountant_access, created_at, updated_at
         FROM users WHERE id=$1`, id)

	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash,
		&user.Role, &user.HasAccountantAccess, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row := r.DB.QueryRow(ctx,
		`SELECT id, name, email, password_hash, role, has_accountant_access, created_at, updated_at
         FROM users WHERE email=$1`, email)

	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash,
		&user.Role, &user.HasAccountantAccess, &user.CreatedAt, &user.UpdatedAt)
	return &user, err
}

// List returns all users
func (r *UserRepository) List(ctx context.Context) ([]*models.User, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, email, role, has_accountant_access, created_at, updated_at
         FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role,
			&user.HasAccountantAccess, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	return users, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, u *models.User) error {
	// If password is empty, don't update it (keep existing password)
	if u.PasswordHash != "" {
		_, err := r.DB.Exec(ctx,
			`UPDATE users SET name=$1, email=$2, password_hash=$3, role=$4, has_accountant_access=$5, updated_at=CURRENT_TIMESTAMP
			 WHERE id=$6`,
			u.Name, u.Email, u.PasswordHash, u.Role, u.HasAccountantAccess, u.ID)
		return err
	}

	// Update without changing password
	_, err := r.DB.Exec(ctx,
		`UPDATE users SET name=$1, email=$2, role=$3, has_accountant_access=$4, updated_at=CURRENT_TIMESTAMP
         WHERE id=$5`,
		u.Name, u.Email, u.Role, u.HasAccountantAccess, u.ID)
	return err
}

// Delete deletes a user
func (r *UserRepository) Delete(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}
