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
		"success":     true,
		"snapshot":    snapshot,
		"target_time": targetTime.Format(time.RFC3339),
	})
}

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
