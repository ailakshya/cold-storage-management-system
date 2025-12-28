package repositories

import (
	"context"
	"encoding/json"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EntryManagementLogRepository struct {
	DB *pgxpool.Pool
}

func NewEntryManagementLogRepository(db *pgxpool.Pool) *EntryManagementLogRepository {
	return &EntryManagementLogRepository{DB: db}
}

// CreateReassignLog logs an entry reassignment
func (r *EntryManagementLogRepository) CreateReassignLog(ctx context.Context, log *models.EntryManagementLog) error {
	query := `
		INSERT INTO entry_management_logs (
			action_type, performed_by_id,
			entry_id, thock_number,
			old_customer_id, old_customer_name, old_customer_phone,
			new_customer_id, new_customer_name, new_customer_phone
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		"reassign", log.PerformedByID,
		log.EntryID, log.ThockNumber,
		log.OldCustomerID, log.OldCustomerName, log.OldCustomerPhone,
		log.NewCustomerID, log.NewCustomerName, log.NewCustomerPhone,
	).Scan(&log.ID, &log.CreatedAt)
}

// CreateMergeLog logs a customer merge with full details
func (r *EntryManagementLogRepository) CreateMergeLog(ctx context.Context, log *models.EntryManagementLog) error {
	// Serialize merge details to JSON
	var mergeDetailsJSON []byte
	if log.MergeDetails != nil {
		var err error
		mergeDetailsJSON, err = json.Marshal(log.MergeDetails)
		if err != nil {
			return err
		}
	}

	query := `
		INSERT INTO entry_management_logs (
			action_type, performed_by_id,
			source_customer_id, source_customer_name, source_customer_phone,
			source_customer_village, source_customer_so,
			target_customer_id, target_customer_name, target_customer_phone,
			target_customer_village, target_customer_so,
			entries_moved, payments_moved, merge_details
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		"merge", log.PerformedByID,
		log.SourceCustomerID, log.SourceCustomerName, log.SourceCustomerPhone,
		log.SourceCustomerVillage, log.SourceCustomerSO,
		log.TargetCustomerID, log.TargetCustomerName, log.TargetCustomerPhone,
		log.TargetCustomerVillage, log.TargetCustomerSO,
		log.EntriesMoved, log.PaymentsMoved, mergeDetailsJSON,
	).Scan(&log.ID, &log.CreatedAt)
}

// List returns all entry management logs with full details
func (r *EntryManagementLogRepository) List(ctx context.Context) ([]*models.EntryManagementLog, error) {
	query := `
		SELECT
			eml.id, eml.action_type, eml.performed_by_id, u.name as performed_by_name,
			eml.entry_id, eml.thock_number,
			eml.old_customer_id, eml.old_customer_name, eml.old_customer_phone,
			eml.new_customer_id, eml.new_customer_name, eml.new_customer_phone,
			eml.source_customer_id, eml.source_customer_name, eml.source_customer_phone,
			COALESCE(eml.source_customer_village, ''), COALESCE(eml.source_customer_so, ''),
			eml.target_customer_id, eml.target_customer_name, eml.target_customer_phone,
			COALESCE(eml.target_customer_village, ''), COALESCE(eml.target_customer_so, ''),
			eml.entries_moved, COALESCE(eml.payments_moved, 0), eml.merge_details,
			eml.created_at
		FROM entry_management_logs eml
		JOIN users u ON eml.performed_by_id = u.id
		ORDER BY eml.created_at DESC
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.EntryManagementLog
	for rows.Next() {
		var log models.EntryManagementLog
		var mergeDetailsJSON []byte
		err := rows.Scan(
			&log.ID, &log.ActionType, &log.PerformedByID, &log.PerformedByName,
			&log.EntryID, &log.ThockNumber,
			&log.OldCustomerID, &log.OldCustomerName, &log.OldCustomerPhone,
			&log.NewCustomerID, &log.NewCustomerName, &log.NewCustomerPhone,
			&log.SourceCustomerID, &log.SourceCustomerName, &log.SourceCustomerPhone,
			&log.SourceCustomerVillage, &log.SourceCustomerSO,
			&log.TargetCustomerID, &log.TargetCustomerName, &log.TargetCustomerPhone,
			&log.TargetCustomerVillage, &log.TargetCustomerSO,
			&log.EntriesMoved, &log.PaymentsMoved, &mergeDetailsJSON,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse merge_details JSON if present
		if len(mergeDetailsJSON) > 0 {
			var details models.MergeDetails
			if err := json.Unmarshal(mergeDetailsJSON, &details); err == nil {
				log.MergeDetails = &details
			}
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// ListByType returns entry management logs filtered by action type
func (r *EntryManagementLogRepository) ListByType(ctx context.Context, actionType string) ([]*models.EntryManagementLog, error) {
	query := `
		SELECT
			eml.id, eml.action_type, eml.performed_by_id, u.name as performed_by_name,
			eml.entry_id, eml.thock_number,
			eml.old_customer_id, eml.old_customer_name, eml.old_customer_phone,
			eml.new_customer_id, eml.new_customer_name, eml.new_customer_phone,
			eml.source_customer_id, eml.source_customer_name, eml.source_customer_phone,
			COALESCE(eml.source_customer_village, ''), COALESCE(eml.source_customer_so, ''),
			eml.target_customer_id, eml.target_customer_name, eml.target_customer_phone,
			COALESCE(eml.target_customer_village, ''), COALESCE(eml.target_customer_so, ''),
			eml.entries_moved, COALESCE(eml.payments_moved, 0), eml.merge_details,
			eml.created_at
		FROM entry_management_logs eml
		JOIN users u ON eml.performed_by_id = u.id
		WHERE eml.action_type = $1
		ORDER BY eml.created_at DESC
	`

	rows, err := r.DB.Query(ctx, query, actionType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.EntryManagementLog
	for rows.Next() {
		var log models.EntryManagementLog
		var mergeDetailsJSON []byte
		err := rows.Scan(
			&log.ID, &log.ActionType, &log.PerformedByID, &log.PerformedByName,
			&log.EntryID, &log.ThockNumber,
			&log.OldCustomerID, &log.OldCustomerName, &log.OldCustomerPhone,
			&log.NewCustomerID, &log.NewCustomerName, &log.NewCustomerPhone,
			&log.SourceCustomerID, &log.SourceCustomerName, &log.SourceCustomerPhone,
			&log.SourceCustomerVillage, &log.SourceCustomerSO,
			&log.TargetCustomerID, &log.TargetCustomerName, &log.TargetCustomerPhone,
			&log.TargetCustomerVillage, &log.TargetCustomerSO,
			&log.EntriesMoved, &log.PaymentsMoved, &mergeDetailsJSON,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse merge_details JSON if present
		if len(mergeDetailsJSON) > 0 {
			var details models.MergeDetails
			if err := json.Unmarshal(mergeDetailsJSON, &details); err == nil {
				log.MergeDetails = &details
			}
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// GetBySourceCustomer gets the merge log for a source customer (to extract payment IDs for undo)
func (r *EntryManagementLogRepository) GetBySourceCustomer(ctx context.Context, sourceCustomerID int) (*models.EntryManagementLog, error) {
	var log models.EntryManagementLog
	var mergeDetailsJSON []byte
	err := r.DB.QueryRow(ctx, `
		SELECT id, COALESCE(merge_details, '{}')
		FROM entry_management_logs
		WHERE action_type = 'merge' AND source_customer_id = $1
		ORDER BY created_at DESC LIMIT 1`,
		sourceCustomerID).Scan(&log.ID, &mergeDetailsJSON)
	if err != nil {
		return nil, err
	}

	if len(mergeDetailsJSON) > 0 {
		var details models.MergeDetails
		if err := json.Unmarshal(mergeDetailsJSON, &details); err == nil {
			log.MergeDetails = &details
		}
	}

	return &log, nil
}

// DeleteBySourceCustomer deletes merge log for a source customer (used when undoing merge)
func (r *EntryManagementLogRepository) DeleteBySourceCustomer(ctx context.Context, sourceCustomerID int) error {
	_, err := r.DB.Exec(ctx, `
		DELETE FROM entry_management_logs
		WHERE action_type = 'merge' AND source_customer_id = $1`,
		sourceCustomerID)
	return err
}

// DeleteByEntryID deletes reassign/transfer log for an entry (used when undoing transfer)
func (r *EntryManagementLogRepository) DeleteByEntryID(ctx context.Context, entryID int) error {
	_, err := r.DB.Exec(ctx, `
		DELETE FROM entry_management_logs
		WHERE action_type = 'reassign' AND entry_id = $1`,
		entryID)
	return err
}
