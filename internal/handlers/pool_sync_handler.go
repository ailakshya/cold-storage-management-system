package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/services"
)

// PoolSyncHandler exposes admin API endpoints for pool-to-NAS sync.
type PoolSyncHandler struct {
	SyncService *services.PoolSyncService
}

func NewPoolSyncHandler(syncService *services.PoolSyncService) *PoolSyncHandler {
	return &PoolSyncHandler{SyncService: syncService}
}

// GetOverview returns aggregate sync stats for all pools.
// GET /api/admin/pool-sync/overview
func (h *PoolSyncHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.SyncService.GetOverview(r.Context())
	if err != nil {
		http.Error(w, "Failed to get pool sync overview: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(overview)
}

// GetScanStates returns last scan info per pool.
// GET /api/admin/pool-sync/scan-states
func (h *PoolSyncHandler) GetScanStates(w http.ResponseWriter, r *http.Request) {
	states, err := h.SyncService.GetScanStates(r.Context())
	if err != nil {
		http.Error(w, "Failed to get scan states: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(states)
}

// TriggerScan triggers a filesystem scan for one or all pools.
// POST /api/admin/pool-sync/scan?pool=bulk  (optional pool param)
func (h *PoolSyncHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	poolName := r.URL.Query().Get("pool")

	if poolName != "" {
		if err := h.SyncService.ScanPool(r.Context(), poolName); err != nil {
			http.Error(w, "Scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Scan all pools in background
		go h.SyncService.ScanAllPools(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"pool":    poolName,
		"message": "Scan initiated",
	})
}

// RetryFailed resets all failed records back to pending.
// POST /api/admin/pool-sync/retry-failed?pool=bulk  (optional pool param)
func (h *PoolSyncHandler) RetryFailed(w http.ResponseWriter, r *http.Request) {
	poolName := r.URL.Query().Get("pool")

	count, err := h.SyncService.RetryFailed(r.Context(), poolName)
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

// GetRecentFailed returns recent failed records for admin inspection.
// GET /api/admin/pool-sync/failed?pool=bulk&limit=50
func (h *PoolSyncHandler) GetRecentFailed(w http.ResponseWriter, r *http.Request) {
	poolName := r.URL.Query().Get("pool")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	records, err := h.SyncService.GetRecentFailed(r.Context(), poolName, limit)
	if err != nil {
		http.Error(w, "Failed to get records: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}
