package repositories

import (
	"context"
	"fmt"
	"net/url"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GatePassMediaRepository struct {
	pool *pgxpool.Pool
}

func NewGatePassMediaRepository(pool *pgxpool.Pool) *GatePassMediaRepository {
	return &GatePassMediaRepository{pool: pool}
}

// Create inserts a new gate pass media record
func (r *GatePassMediaRepository) Create(ctx context.Context, media *models.GatePassMedia) error {
	query := `
		INSERT INTO gate_pass_media (
			gate_pass_id, gate_pass_pickup_id, thock_number,
			media_type, file_path, file_name, file_type,
			file_size, uploaded_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		media.GatePassID,
		media.GatePassPickupID,
		media.ThockNumber,
		media.MediaType,
		media.FilePath,
		media.FileName,
		media.FileType,
		media.FileSize,
		media.UploadedByUserID,
	).Scan(&media.ID, &media.CreatedAt)

	return err
}

// ListByThockNumber retrieves all media (entry + pickup) for a given thock number
func (r *GatePassMediaRepository) ListByThockNumber(ctx context.Context, thockNumber string) ([]models.GatePassMedia, error) {
	query := `
		SELECT
			m.id,
			m.gate_pass_id,
			m.gate_pass_pickup_id,
			m.thock_number,
			m.media_type,
			m.file_path,
			m.file_name,
			m.file_type,
			m.file_size,
			m.uploaded_by_user_id,
			m.created_at,
			COALESCE(u.name, u.email, 'Unknown') as uploaded_by_user_name
		FROM gate_pass_media m
		LEFT JOIN users u ON m.uploaded_by_user_id = u.id
		WHERE m.thock_number = $1
		ORDER BY m.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, thockNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []models.GatePassMedia
	for rows.Next() {
		var media models.GatePassMedia
		err := rows.Scan(
			&media.ID,
			&media.GatePassID,
			&media.GatePassPickupID,
			&media.ThockNumber,
			&media.MediaType,
			&media.FilePath,
			&media.FileName,
			&media.FileType,
			&media.FileSize,
			&media.UploadedByUserID,
			&media.CreatedAt,
			&media.UploadedByUserName,
		)
		if err != nil {
			return nil, err
		}

		// Compute download URL
		media.DownloadURL = fmt.Sprintf(
			"/api/files/download?root=bulk&path=%s&mode=inline",
			url.QueryEscape(media.FilePath),
		)

		mediaList = append(mediaList, media)
	}

	return mediaList, rows.Err()
}

// ListByGatePassID retrieves all media for a specific gate pass
func (r *GatePassMediaRepository) ListByGatePassID(ctx context.Context, gatePassID int) ([]models.GatePassMedia, error) {
	query := `
		SELECT
			m.id,
			m.gate_pass_id,
			m.gate_pass_pickup_id,
			m.thock_number,
			m.media_type,
			m.file_path,
			m.file_name,
			m.file_type,
			m.file_size,
			m.uploaded_by_user_id,
			m.created_at,
			COALESCE(u.name, u.email, 'Unknown') as uploaded_by_user_name
		FROM gate_pass_media m
		LEFT JOIN users u ON m.uploaded_by_user_id = u.id
		WHERE m.gate_pass_id = $1
		ORDER BY m.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, gatePassID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []models.GatePassMedia
	for rows.Next() {
		var media models.GatePassMedia
		err := rows.Scan(
			&media.ID,
			&media.GatePassID,
			&media.GatePassPickupID,
			&media.ThockNumber,
			&media.MediaType,
			&media.FilePath,
			&media.FileName,
			&media.FileType,
			&media.FileSize,
			&media.UploadedByUserID,
			&media.CreatedAt,
			&media.UploadedByUserName,
		)
		if err != nil {
			return nil, err
		}

		// Compute download URL
		media.DownloadURL = fmt.Sprintf(
			"/api/files/download?root=bulk&path=%s&mode=inline",
			url.QueryEscape(media.FilePath),
		)

		mediaList = append(mediaList, media)
	}

	return mediaList, rows.Err()
}
