package repositories

import (
	"context"
	"cold-backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SystemSettingRepository struct {
	DB *pgxpool.Pool
}

func NewSystemSettingRepository(db *pgxpool.Pool) *SystemSettingRepository {
	return &SystemSettingRepository{DB: db}
}

func (r *SystemSettingRepository) Get(ctx context.Context, key string) (*models.SystemSetting, error) {
	query := `
		SELECT id, setting_key, setting_value, description, updated_at, COALESCE(updated_by_user_id, 0)
		FROM system_settings
		WHERE setting_key = $1
	`

	setting := &models.SystemSetting{}
	err := r.DB.QueryRow(ctx, query, key).Scan(
		&setting.ID,
		&setting.SettingKey,
		&setting.SettingValue,
		&setting.Description,
		&setting.UpdatedAt,
		&setting.UpdatedByUserID,
	)

	if err != nil {
		return nil, err
	}

	return setting, nil
}

func (r *SystemSettingRepository) List(ctx context.Context) ([]*models.SystemSetting, error) {
	query := `
		SELECT id, setting_key, setting_value, description, updated_at, COALESCE(updated_by_user_id, 0)
		FROM system_settings
		ORDER BY setting_key
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []*models.SystemSetting
	for rows.Next() {
		setting := &models.SystemSetting{}
		err := rows.Scan(
			&setting.ID,
			&setting.SettingKey,
			&setting.SettingValue,
			&setting.Description,
			&setting.UpdatedAt,
			&setting.UpdatedByUserID,
		)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}

	return settings, nil
}

func (r *SystemSettingRepository) Update(ctx context.Context, key string, value string, userID int) error {
	query := `
		UPDATE system_settings
		SET setting_value = $1, updated_at = CURRENT_TIMESTAMP, updated_by_user_id = $2
		WHERE setting_key = $3
	`

	// Pass NULL for user ID if 0 (foreign key constraint)
	var userIDParam interface{}
	if userID == 0 {
		userIDParam = nil
	} else {
		userIDParam = userID
	}

	_, err := r.DB.Exec(ctx, query, value, userIDParam, key)
	return err
}

// Upsert creates a new setting or updates an existing one
func (r *SystemSettingRepository) Upsert(ctx context.Context, key string, value string, description string, userID int) error {
	query := `
		INSERT INTO system_settings (setting_key, setting_value, description, updated_at, updated_by_user_id)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, $4)
		ON CONFLICT (setting_key)
		DO UPDATE SET setting_value = $2, description = $3, updated_at = CURRENT_TIMESTAMP, updated_by_user_id = $4
	`

	// Pass NULL for user ID if 0 (foreign key constraint)
	var userIDParam interface{}
	if userID == 0 {
		userIDParam = nil
	} else {
		userIDParam = userID
	}

	_, err := r.DB.Exec(ctx, query, key, value, description, userIDParam)
	return err
}
