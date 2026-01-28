package repositories

import (
	"context"
	"fmt"
	"net/url"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoomEntryMediaRepository struct {
	pool *pgxpool.Pool
}

func NewRoomEntryMediaRepository(pool *pgxpool.Pool) *RoomEntryMediaRepository {
	return &RoomEntryMediaRepository{pool: pool}
}

func (r *RoomEntryMediaRepository) Create(ctx context.Context, media *models.RoomEntryMedia) error {
	query := `
		INSERT INTO room_entry_media (
			room_entry_id, thock_number, media_type,
			file_path, file_name, file_type,
			file_size, uploaded_by_user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query,
		media.RoomEntryID,
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

func (r *RoomEntryMediaRepository) ListByRoomEntryID(ctx context.Context, roomEntryID int) ([]models.RoomEntryMedia, error) {
	query := `
		SELECT
			m.id, m.room_entry_id, m.thock_number, m.media_type,
			m.file_path, m.file_name, m.file_type, m.file_size,
			m.uploaded_by_user_id, m.created_at,
			COALESCE(u.name, u.email, 'Unknown') as uploaded_by_user_name
		FROM room_entry_media m
		LEFT JOIN users u ON m.uploaded_by_user_id = u.id
		WHERE m.room_entry_id = $1
		ORDER BY m.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, roomEntryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []models.RoomEntryMedia
	for rows.Next() {
		var media models.RoomEntryMedia
		err := rows.Scan(
			&media.ID,
			&media.RoomEntryID,
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

		media.DownloadURL = fmt.Sprintf(
			"/api/files/download?root=bulk&path=%s&mode=inline",
			url.QueryEscape(media.FilePath),
		)

		mediaList = append(mediaList, media)
	}

	return mediaList, rows.Err()
}

func (r *RoomEntryMediaRepository) ListByThockNumber(ctx context.Context, thockNumber string) ([]models.RoomEntryMedia, error) {
	query := `
		SELECT
			m.id, m.room_entry_id, m.thock_number, m.media_type,
			m.file_path, m.file_name, m.file_type, m.file_size,
			m.uploaded_by_user_id, m.created_at,
			COALESCE(u.name, u.email, 'Unknown') as uploaded_by_user_name
		FROM room_entry_media m
		LEFT JOIN users u ON m.uploaded_by_user_id = u.id
		WHERE m.thock_number = $1
		ORDER BY m.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, thockNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []models.RoomEntryMedia
	for rows.Next() {
		var media models.RoomEntryMedia
		err := rows.Scan(
			&media.ID,
			&media.RoomEntryID,
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

		media.DownloadURL = fmt.Sprintf(
			"/api/files/download?root=bulk&path=%s&mode=inline",
			url.QueryEscape(media.FilePath),
		)

		mediaList = append(mediaList, media)
	}

	return mediaList, rows.Err()
}
