package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// TrashService handles the trash bin / undo functionality
type TrashService struct {
	db *sql.DB
}

// NewTrashService creates a new trash service
func NewTrashService(db *sql.DB) *TrashService {
	return &TrashService{db: db}
}

// TrashItem represents an item in the trash bin
type TrashItem struct {
	ID               int             `json:"id"`
	TableName        string          `json:"table_name"`
	RecordID         int             `json:"record_id"`
	RecordData       json.RawMessage `json:"record_data"`
	RelatedData      json.RawMessage `json:"related_data,omitempty"`
	DeletedAt        time.Time       `json:"deleted_at"`
	DeletedByUserID  *int            `json:"deleted_by_user_id,omitempty"`
	DeletedByName    string          `json:"deleted_by_name,omitempty"`
	DeleteReason     string          `json:"delete_reason,omitempty"`
	ExpiresAt        time.Time       `json:"expires_at"`
	RestoredAt       *time.Time      `json:"restored_at,omitempty"`
	RestoredByUserID *int            `json:"restored_by_user_id,omitempty"`
}

// MoveToTrash moves a record to the trash bin
func (s *TrashService) MoveToTrash(ctx context.Context, tableName string, recordID int, deletedByUserID *int, reason string) (int, error) {
	var trashID sql.NullInt32

	err := s.db.QueryRowContext(ctx,
		`SELECT move_to_trash($1, $2, $3, $4)`,
		tableName, recordID, deletedByUserID, reason,
	).Scan(&trashID)

	if err != nil {
		return 0, fmt.Errorf("failed to move record to trash: %w", err)
	}

	if !trashID.Valid {
		return 0, fmt.Errorf("record not found or already in trash")
	}

	log.Printf("[Trash] Moved %s#%d to trash (trash_id: %d)", tableName, recordID, trashID.Int32)
	return int(trashID.Int32), nil
}

// RestoreFromTrash restores a record from the trash bin
func (s *TrashService) RestoreFromTrash(ctx context.Context, trashID int, restoredByUserID *int) error {
	var success bool

	err := s.db.QueryRowContext(ctx,
		`SELECT restore_from_trash($1, $2)`,
		trashID, restoredByUserID,
	).Scan(&success)

	if err != nil {
		return fmt.Errorf("failed to restore from trash: %w", err)
	}

	if !success {
		return fmt.Errorf("restore failed: record not found in trash or already restored")
	}

	log.Printf("[Trash] Restored trash item #%d", trashID)
	return nil
}

// ListTrashItems returns all active (non-expired, non-restored) items in trash
func (s *TrashService) ListTrashItems(ctx context.Context, tableName string, limit int) ([]TrashItem, error) {
	query := `
		SELECT id, table_name, record_id, record_data, related_data,
		       deleted_at, deleted_by_user_id, deleted_by_name, delete_reason,
		       expires_at, restored_at, restored_by_user_id
		FROM v_trash_bin_active
		WHERE ($1 = '' OR table_name = $1)
		ORDER BY deleted_at DESC
		LIMIT $2
	`

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, query, tableName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list trash items: %w", err)
	}
	defer rows.Close()

	var items []TrashItem
	for rows.Next() {
		var item TrashItem
		var relatedData, deleteReason, deletedByName sql.NullString
		var deletedByUserID, restoredByUserID sql.NullInt32
		var restoredAt sql.NullTime

		err := rows.Scan(
			&item.ID, &item.TableName, &item.RecordID, &item.RecordData, &relatedData,
			&item.DeletedAt, &deletedByUserID, &deletedByName, &deleteReason,
			&item.ExpiresAt, &restoredAt, &restoredByUserID,
		)
		if err != nil {
			continue
		}

		if relatedData.Valid {
			item.RelatedData = json.RawMessage(relatedData.String)
		}
		if deleteReason.Valid {
			item.DeleteReason = deleteReason.String
		}
		if deletedByName.Valid {
			item.DeletedByName = deletedByName.String
		}
		if deletedByUserID.Valid {
			id := int(deletedByUserID.Int32)
			item.DeletedByUserID = &id
		}
		if restoredAt.Valid {
			item.RestoredAt = &restoredAt.Time
		}
		if restoredByUserID.Valid {
			id := int(restoredByUserID.Int32)
			item.RestoredByUserID = &id
		}

		items = append(items, item)
	}

	return items, nil
}

// GetTrashItem returns a specific trash item by ID
func (s *TrashService) GetTrashItem(ctx context.Context, trashID int) (*TrashItem, error) {
	var item TrashItem
	var relatedData, deleteReason, deletedByName sql.NullString
	var deletedByUserID, restoredByUserID sql.NullInt32
	var restoredAt sql.NullTime

	err := s.db.QueryRowContext(ctx, `
		SELECT tb.id, tb.table_name, tb.record_id, tb.record_data, tb.related_data,
		       tb.deleted_at, tb.deleted_by_user_id, u.name, tb.delete_reason,
		       tb.expires_at, tb.restored_at, tb.restored_by_user_id
		FROM trash_bin tb
		LEFT JOIN users u ON tb.deleted_by_user_id = u.id
		WHERE tb.id = $1
	`, trashID).Scan(
		&item.ID, &item.TableName, &item.RecordID, &item.RecordData, &relatedData,
		&item.DeletedAt, &deletedByUserID, &deletedByName, &deleteReason,
		&item.ExpiresAt, &restoredAt, &restoredByUserID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get trash item: %w", err)
	}

	if relatedData.Valid {
		item.RelatedData = json.RawMessage(relatedData.String)
	}
	if deleteReason.Valid {
		item.DeleteReason = deleteReason.String
	}
	if deletedByName.Valid {
		item.DeletedByName = deletedByName.String
	}
	if deletedByUserID.Valid {
		id := int(deletedByUserID.Int32)
		item.DeletedByUserID = &id
	}
	if restoredAt.Valid {
		item.RestoredAt = &restoredAt.Time
	}
	if restoredByUserID.Valid {
		id := int(restoredByUserID.Int32)
		item.RestoredByUserID = &id
	}

	return &item, nil
}

// PurgeExpired removes permanently expired trash items
func (s *TrashService) PurgeExpired(ctx context.Context) (int, error) {
	var count int

	err := s.db.QueryRowContext(ctx, `SELECT purge_expired_trash()`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to purge expired trash: %w", err)
	}

	if count > 0 {
		log.Printf("[Trash] Purged %d expired items", count)
	}

	return count, nil
}

// GetTrashStats returns statistics about the trash bin
func (s *TrashService) GetTrashStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count by table
	rows, err := s.db.QueryContext(ctx, `
		SELECT table_name, COUNT(*) as count
		FROM trash_bin
		WHERE restored_at IS NULL AND expires_at > NOW()
		GROUP BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byTable := make(map[string]int)
	total := 0
	for rows.Next() {
		var tableName string
		var count int
		if err := rows.Scan(&tableName, &count); err == nil {
			byTable[tableName] = count
			total += count
		}
	}
	stats["by_table"] = byTable
	stats["total_active"] = total

	// Expiring soon (within 7 days)
	var expiringSoon int
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trash_bin
		WHERE restored_at IS NULL
		AND expires_at > NOW()
		AND expires_at < NOW() + INTERVAL '7 days'
	`).Scan(&expiringSoon)
	stats["expiring_soon"] = expiringSoon

	// Recently deleted (last 24 hours)
	var recentlyDeleted int
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trash_bin
		WHERE deleted_at > NOW() - INTERVAL '24 hours'
	`).Scan(&recentlyDeleted)
	stats["recently_deleted"] = recentlyDeleted

	// Restored count
	var restoredCount int
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trash_bin WHERE restored_at IS NOT NULL
	`).Scan(&restoredCount)
	stats["restored_count"] = restoredCount

	return stats, nil
}

// UndoLastDelete restores the most recently deleted item for a table
func (s *TrashService) UndoLastDelete(ctx context.Context, tableName string, restoredByUserID *int) (*TrashItem, error) {
	// Get most recent trash item for this table
	var trashID int
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM v_trash_bin_active
		WHERE table_name = $1
		ORDER BY deleted_at DESC
		LIMIT 1
	`, tableName).Scan(&trashID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no recent deletes found for %s", tableName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find last delete: %w", err)
	}

	// Restore it
	if err := s.RestoreFromTrash(ctx, trashID, restoredByUserID); err != nil {
		return nil, err
	}

	// Return the restored item
	return s.GetTrashItem(ctx, trashID)
}
