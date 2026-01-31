package handlers

import (
	"encoding/json"
	"net/http"

	"cold-backend/internal/services"
)

// MediaSyncHandler exposes admin API endpoints for the 3-2-1 media sync system.
type MediaSyncHandler struct {
	SyncService *services.MediaSyncService
}

func NewMediaSyncHandler(syncService *services.MediaSyncService) *MediaSyncHandler {
	return &MediaSyncHandler{SyncService: syncService}
}

// GetStatus returns aggregate sync status counts.
// GET /api/admin/media-sync/status
func (h *MediaSyncHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats, err := h.SyncService.GetStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to get sync stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// TriggerInitialSync enqueues all existing media files that aren't yet in the sync queue.
// POST /api/admin/media-sync/initial-sync
func (h *MediaSyncHandler) TriggerInitialSync(w http.ResponseWriter, r *http.Request) {
	count, err := h.SyncService.RunInitialSync(r.Context())
	if err != nil {
		http.Error(w, "Initial sync failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"enqueued": count,
	})
}

// RetryFailed resets all failed sync records back to pending.
// POST /api/admin/media-sync/retry-failed
func (h *MediaSyncHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	count, err := h.SyncService.RetryAllFailed(r.Context())
	if err != nil {
		http.Error(w, "Retry failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"reset":   count,
	})
}

// BulkRestore downloads all cloud-synced media files that are missing locally.
// POST /api/admin/media-sync/restore
func (h *MediaSyncHandler) BulkRestore(w http.ResponseWriter, r *http.Request) {
	progress, err := h.SyncService.BulkRestore(r.Context())
	if err != nil {
		http.Error(w, "Restore failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}
