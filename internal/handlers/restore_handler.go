package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"cold-backend/internal/middleware"
	"cold-backend/internal/services"
)

// RestoreHandler handles point-in-time restore operations
type RestoreHandler struct {
	Service *services.RestoreService
}

// NewRestoreHandler creates a new restore handler
func NewRestoreHandler(service *services.RestoreService) *RestoreHandler {
	return &RestoreHandler{Service: service}
}

// ListRestorePoints returns available restore points grouped by date
// GET /api/admin/restore/snapshots
func (h *RestoreHandler) ListRestorePoints(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for specific date parameter
	date := r.URL.Query().Get("date")

	if date != "" {
		// Return snapshots for specific date
		snapshots, err := h.Service.ListSnapshotsForDate(ctx, date)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"date":      date,
			"snapshots": snapshots,
			"count":     len(snapshots),
		})
		return
	}

	// Return all dates
	dates, totalBackups, err := h.Service.ListAvailableDates(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"dates":         dates,
		"total_backups": totalBackups,
	})
}

// FindClosestSnapshot finds the snapshot closest to a target time
// GET /api/admin/restore/closest?datetime=2026-01-02T15:30:00
func (h *RestoreHandler) FindClosestSnapshot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	datetimeStr := r.URL.Query().Get("datetime")
	if datetimeStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "datetime parameter is required (format: 2006-01-02T15:04:05)",
		})
		return
	}

	targetTime, err := time.Parse("2006-01-02T15:04:05", datetimeStr)
	if err != nil {
		// Try alternative format
		targetTime, err = time.Parse("2006-01-02T15:04", datetimeStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "invalid datetime format, use: 2006-01-02T15:04:05 or 2006-01-02T15:04",
			})
			return
		}
	}

	snapshot, err := h.Service.FindClosestSnapshot(ctx, targetTime)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"snapshot": snapshot,
	})
}

// GetBackupConfiguration returns the current backup settings
// GET /api/admin/restore/config
func (h *RestoreHandler) GetBackupConfiguration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	config, err := h.Service.GetBackupConfiguration(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateBackupConfiguration updates the backup settings
// PUT /api/admin/restore/config
func (h *RestoreHandler) UpdateBackupConfiguration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var config services.BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.Service.UpdateBackupConfiguration(ctx, config, userID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Backup configuration updated successfully",
	})
}

// PreviewRestore initiates a restore operation (creates confirmation token)
// POST /api/admin/restore/preview

// PreviewRestore generates a preview and confirmation token
// POST /api/admin/restore/preview
func (h *RestoreHandler) PreviewRestore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var req struct {
		SnapshotKey string `json:"snapshot_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.SnapshotKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "snapshot_key is required",
		})
		return
	}

	preview, err := h.Service.PreviewRestore(ctx, req.SnapshotKey, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"preview": preview,
	})
}

// ListLocalBackups returns available local backup files
// GET /api/admin/restore/local
func (h *RestoreHandler) ListLocalBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := h.Service.ListLocalBackups()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"backups": backups,
		"count":   len(backups),
	})
}

// PreviewLocalRestore generates a preview and confirmation token for local restore
// POST /api/admin/restore/local/preview
func (h *RestoreHandler) PreviewLocalRestore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var req struct {
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Filename == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "filename is required",
		})
		return
	}

	preview, err := h.Service.PreviewLocalRestore(req.Filename, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"preview": preview,
	})
}

// ExecuteLocalRestore performs the actual restore operation from local file
// POST /api/admin/restore/local/execute
func (h *RestoreHandler) ExecuteLocalRestore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var req struct {
		Filename          string `json:"filename"`
		ConfirmationToken string `json:"confirmation_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.Filename == "" || req.ConfirmationToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "filename and confirmation_token are required",
		})
		return
	}

	result, err := h.Service.ExecuteLocalRestore(ctx, req.Filename, req.ConfirmationToken, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Local restore failed: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  result,
	})
}

// ExecuteRestore performs the actual restore operation
// POST /api/admin/restore/execute
func (h *RestoreHandler) ExecuteRestore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	var req struct {
		SnapshotKey       string `json:"snapshot_key"`
		ConfirmationToken string `json:"confirmation_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.SnapshotKey == "" || req.ConfirmationToken == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "snapshot_key and confirmation_token are required",
		})
		return
	}

	result, err := h.Service.ExecuteRestore(ctx, req.SnapshotKey, req.ConfirmationToken, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"result":  result,
	})
}

// DeleteLocalBackup deletes a local backup file
// DELETE /api/admin/restore/local
func (h *RestoreHandler) DeleteLocalBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract filename from query params or body
	// For DELETE requests, key is often passed in query or path
	filename := r.URL.Query().Get("filename")

	if filename == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "filename parameter is required",
		})
		return
	}

	// Get user ID from context for logging (optional)
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	// Check permissions? Assuming admin route middleware handles this.

	if err := h.Service.DeleteLocalBackup(filename); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"message":    "Backup file deleted successfully",
		"deleted_by": userID,
	})
}

// CreateBackup creates a new local and optionally cloud backup
// POST /api/admin/restore/create
func (h *RestoreHandler) CreateBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "User not authenticated",
		})
		return
	}

	key, err := h.Service.CreateBackup(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Backup creation failed: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"key":        key,
		"created_by": userID,
	})
}
