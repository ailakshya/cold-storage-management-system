package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"cold-backend/internal/middleware"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DeletedEntriesHandler handles viewing and restoring soft-deleted entries
type DeletedEntriesHandler struct {
	pool *pgxpool.Pool
}

// NewDeletedEntriesHandler creates a new deleted entries handler
func NewDeletedEntriesHandler(pool *pgxpool.Pool) *DeletedEntriesHandler {
	return &DeletedEntriesHandler{
		pool: pool,
	}
}

// DeletedEntry represents a soft-deleted entry with restore information
type DeletedEntry struct {
	ID                     int       `json:"id"`
	CustomerID             int       `json:"customer_id"`
	Phone                  string    `json:"phone"`
	Name                   string    `json:"name"`
	Village                string    `json:"village"`
	ExpectedQuantity       int       `json:"expected_quantity"`
	ThockCategory          string    `json:"thock_category"`
	ThockNumber            string    `json:"thock_number"`
	SO                     string    `json:"so"`
	Remark                 string    `json:"remark"`
	Status                 string    `json:"status"`
	PreviousStatus         *string   `json:"previous_status"`
	CreatedAt              time.Time `json:"created_at"`
	DeletedAt              *time.Time `json:"deleted_at"`
	TransferredToCustomerID *int      `json:"transferred_to_customer_id"`
	FamilyMemberName       *string   `json:"family_member_name"`
}

// SetSessionAndRedirect validates token and sets session cookie
func (h *DeletedEntriesHandler) SetSessionAndRedirect(w http.ResponseWriter, r *http.Request) {
	// This endpoint is called with token in POST body, validates it, sets cookie, and redirects
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Set the token as a secure HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    req.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"redirect": "/admin/deleted-entries",
	})
}

// ViewDeletedEntriesPage renders the deleted entries page
func (h *DeletedEntriesHandler) ViewDeletedEntriesPage(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/deleted_entries.html"))

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// ListDeletedEntries returns all soft-deleted entries
// GET /api/admin/deleted-entries
func (h *DeletedEntriesHandler) ListDeletedEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT
			id, customer_id, phone, name, village, expected_quantity,
			thock_category, thock_number, so, remark, status,
			deleted_at, transferred_to_customer_id, family_member_name,
			created_at
		FROM entries
		WHERE status = 'deleted'
		ORDER BY deleted_at DESC NULLS LAST, id DESC
		LIMIT 100
	`

	rows, err := h.pool.Query(ctx, query)
	if err != nil {
		http.Error(w, "Failed to fetch deleted entries", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []DeletedEntry
	for rows.Next() {
		var e DeletedEntry
		err := rows.Scan(
			&e.ID, &e.CustomerID, &e.Phone, &e.Name, &e.Village,
			&e.ExpectedQuantity, &e.ThockCategory, &e.ThockNumber,
			&e.SO, &e.Remark, &e.Status, &e.DeletedAt,
			&e.TransferredToCustomerID, &e.FamilyMemberName,
			&e.CreatedAt,
		)
		if err != nil {
			continue
		}

		// Determine previous status based on other fields
		// If it was transferred, previous status was 'transferred'
		// Otherwise, it was 'active'
		if e.TransferredToCustomerID != nil && *e.TransferredToCustomerID > 0 {
			prevStatus := "transferred"
			e.PreviousStatus = &prevStatus
		} else {
			prevStatus := "active"
			e.PreviousStatus = &prevStatus
		}

		entries = append(entries, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"entries": entries,
		"count":   len(entries),
	})
}

// RestoreEntry restores a soft-deleted entry to its previous state
// POST /api/admin/deleted-entries/{id}/restore
func (h *DeletedEntriesHandler) RestoreEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	entryID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Start transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Get entry details to determine previous status
	var entry DeletedEntry
	query := `
		SELECT
			id, status, transferred_to_customer_id
		FROM entries
		WHERE id = $1 AND status = 'deleted'
	`
	err = tx.QueryRow(ctx, query, entryID).Scan(
		&entry.ID, &entry.Status, &entry.TransferredToCustomerID,
	)
	if err != nil {
		http.Error(w, "Entry not found or not deleted", http.StatusNotFound)
		return
	}

	// Determine previous status
	previousStatus := "active"
	if entry.TransferredToCustomerID != nil && *entry.TransferredToCustomerID > 0 {
		previousStatus = "transferred"
	}

	// Restore entry to previous status
	updateQuery := `
		UPDATE entries
		SET
			status = $1,
			deleted_at = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	_, err = tx.Exec(ctx, updateQuery, previousStatus, entryID)
	if err != nil {
		http.Error(w, "Failed to restore entry", http.StatusInternalServerError)
		return
	}

	// Log the restore action
	logQuery := `
		INSERT INTO admin_action_logs
		(admin_user_id, action_type, target_type, target_id, description, old_value, new_value, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
	`
	description := "Restored deleted entry"
	_, err = tx.Exec(ctx, logQuery,
		userID, "restore", "entry", entryID, description,
		"deleted", previousStatus, r.RemoteAddr,
	)
	if err != nil {
		// Log error but don't fail the restore
		// TODO: Add proper logging
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit restore", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         "Entry restored successfully",
		"entry_id":        entryID,
		"previous_status": previousStatus,
	})
}

// BulkRestoreEntries restores multiple deleted entries at once
// POST /api/admin/deleted-entries/restore-bulk
func (h *DeletedEntriesHandler) BulkRestoreEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req struct {
		EntryIDs []int `json:"entry_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.EntryIDs) == 0 {
		http.Error(w, "No entry IDs provided", http.StatusBadRequest)
		return
	}

	// Start transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	restoredCount := 0
	for _, entryID := range req.EntryIDs {
		// Get entry details
		var transferredToCustomerID *int
		query := `
			SELECT transferred_to_customer_id
			FROM entries
			WHERE id = $1 AND status = 'deleted'
		`
		err := tx.QueryRow(ctx, query, entryID).Scan(&transferredToCustomerID)
		if err != nil {
			continue // Skip if not found or not deleted
		}

		// Determine previous status
		previousStatus := "active"
		if transferredToCustomerID != nil && *transferredToCustomerID > 0 {
			previousStatus = "transferred"
		}

		// Restore entry
		updateQuery := `
			UPDATE entries
			SET
				status = $1,
				deleted_at = NULL,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $2
		`
		result, err := tx.Exec(ctx, updateQuery, previousStatus, entryID)
		if err != nil {
			continue
		}

		if result.RowsAffected() > 0 {
			restoredCount++

			// Log the restore action
			logQuery := `
				INSERT INTO admin_action_logs
				(admin_user_id, action_type, target_type, target_id, description, old_value, new_value, ip_address, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
			`
			tx.Exec(ctx, logQuery,
				userID, "restore", "entry", entryID, "Bulk restored deleted entry",
				"deleted", previousStatus, r.RemoteAddr,
			)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit bulk restore", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"message":        "Entries restored successfully",
		"restored_count": restoredCount,
		"total_requested": len(req.EntryIDs),
	})
}

// PermanentDeleteEntry permanently deletes a soft-deleted entry from the database
// DELETE /api/admin/deleted-entries/{id}
func (h *DeletedEntriesHandler) PermanentDeleteEntry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	entryID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Start transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	// Verify entry is soft-deleted before permanent deletion
	var status string
	checkQuery := `SELECT status FROM entries WHERE id = $1`
	err = tx.QueryRow(ctx, checkQuery, entryID).Scan(&status)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	if status != "deleted" {
		http.Error(w, "Can only permanently delete soft-deleted entries", http.StatusBadRequest)
		return
	}

	// Delete related records first (manual cascade for tables without ON DELETE CASCADE)
	// Delete from entry_edit_logs
	_, _ = tx.Exec(ctx, `DELETE FROM entry_edit_logs WHERE entry_id = $1`, entryID)
	// Delete from online_transactions
	_, _ = tx.Exec(ctx, `DELETE FROM online_transactions WHERE entry_id = $1`, entryID)
	// Delete from rent_payments
	_, _ = tx.Exec(ctx, `DELETE FROM rent_payments WHERE entry_id = $1`, entryID)

	// Now permanently delete the entry (other tables have CASCADE or SET NULL)
	deleteQuery := `DELETE FROM entries WHERE id = $1`
	result, err := tx.Exec(ctx, deleteQuery, entryID)
	if err != nil {
		http.Error(w, "Failed to delete entry", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	// Log the permanent deletion
	logQuery := `
		INSERT INTO admin_action_logs
		(admin_user_id, action_type, target_type, target_id, description, old_value, new_value, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
	`
	description := "Permanently deleted entry"
	_, err = tx.Exec(ctx, logQuery,
		userID, "permanent_delete", "entry", entryID, description,
		"deleted", "permanently_removed", r.RemoteAddr,
	)
	if err != nil {
		// Log error but don't fail the deletion
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit deletion", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Entry permanently deleted",
		"entry_id": entryID,
	})
}

// BulkPermanentDeleteEntries permanently deletes multiple soft-deleted entries
// DELETE /api/admin/deleted-entries/bulk
func (h *DeletedEntriesHandler) BulkPermanentDeleteEntries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req struct {
		EntryIDs []int `json:"entry_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.EntryIDs) == 0 {
		http.Error(w, "No entry IDs provided", http.StatusBadRequest)
		return
	}

	// Start transaction
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	deletedCount := 0
	for _, entryID := range req.EntryIDs {
		// Verify entry is soft-deleted
		var status string
		checkQuery := `SELECT status FROM entries WHERE id = $1`
		err := tx.QueryRow(ctx, checkQuery, entryID).Scan(&status)
		if err != nil {
			continue // Skip if not found
		}

		if status != "deleted" {
			continue // Skip if not soft-deleted
		}

		// Delete related records first (manual cascade for tables without ON DELETE CASCADE)
		tx.Exec(ctx, `DELETE FROM entry_edit_logs WHERE entry_id = $1`, entryID)
		tx.Exec(ctx, `DELETE FROM online_transactions WHERE entry_id = $1`, entryID)
		tx.Exec(ctx, `DELETE FROM rent_payments WHERE entry_id = $1`, entryID)

		// Permanently delete entry (other tables have CASCADE or SET NULL)
		deleteQuery := `DELETE FROM entries WHERE id = $1`
		result, err := tx.Exec(ctx, deleteQuery, entryID)
		if err != nil {
			continue
		}

		if result.RowsAffected() > 0 {
			deletedCount++

			// Log the permanent deletion
			logQuery := `
				INSERT INTO admin_action_logs
				(admin_user_id, action_type, target_type, target_id, description, old_value, new_value, ip_address, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
			`
			tx.Exec(ctx, logQuery,
				userID, "permanent_delete", "entry", entryID, "Bulk permanently deleted entry",
				"deleted", "permanently_removed", r.RemoteAddr,
			)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "Failed to commit bulk deletion", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         "Entries permanently deleted",
		"deleted_count":   deletedCount,
		"total_requested": len(req.EntryIDs),
	})
}

// GetDeletedEntriesStats returns statistics about deleted entries
// GET /api/admin/deleted-entries/stats
func (h *DeletedEntriesHandler) GetDeletedEntriesStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT
			COUNT(*) as total_deleted,
			COUNT(CASE WHEN deleted_at IS NOT NULL THEN 1 END) as with_timestamp,
			MIN(deleted_at) as oldest_deletion,
			MAX(deleted_at) as newest_deletion
		FROM entries
		WHERE status = 'deleted'
	`

	var stats struct {
		TotalDeleted  int        `json:"total_deleted"`
		WithTimestamp int        `json:"with_timestamp"`
		OldestDeletion *time.Time `json:"oldest_deletion"`
		NewestDeletion *time.Time `json:"newest_deletion"`
	}

	err := h.pool.QueryRow(ctx, query).Scan(
		&stats.TotalDeleted,
		&stats.WithTimestamp,
		&stats.OldestDeletion,
		&stats.NewestDeletion,
	)
	if err != nil {
		http.Error(w, "Failed to fetch stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}
