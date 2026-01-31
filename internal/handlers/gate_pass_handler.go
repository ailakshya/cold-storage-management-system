package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type GatePassHandler struct {
	Service         *services.GatePassService
	AdminActionRepo *repositories.AdminActionLogRepository
	SyncService     *services.MediaSyncService
}

func NewGatePassHandler(service *services.GatePassService, adminActionRepo *repositories.AdminActionLogRepository) *GatePassHandler {
	return &GatePassHandler{
		Service:         service,
		AdminActionRepo: adminActionRepo,
	}
}

func (h *GatePassHandler) SetSyncService(s *services.MediaSyncService) {
	h.SyncService = s
}

// CreateGatePass issues a new gate pass
func (h *GatePassHandler) CreateGatePass(w http.ResponseWriter, r *http.Request) {
	var req models.CreateGatePassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	gatePass, err := h.Service.CreateGatePass(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate gate pass cache
	cache.InvalidateGatePassCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Issued gate pass for thock %s - %d items requested", req.ThockNumber, req.RequestedQuantity)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "CREATE",
		TargetType:  "gate_pass",
		TargetID:    &gatePass.ID,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(gatePass)
}

// ListAllGatePasses returns all gate passes
func (h *GatePassHandler) ListAllGatePasses(w http.ResponseWriter, r *http.Request) {
	gatePasses, err := h.Service.ListAllGatePasses(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if gatePasses == nil {
		gatePasses = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gatePasses)
}

// ListPendingGatePasses returns pending gate passes (for unloading tickets)
func (h *GatePassHandler) ListPendingGatePasses(w http.ResponseWriter, r *http.Request) {
	gatePasses, err := h.Service.ListPendingGatePasses(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if gatePasses == nil {
		gatePasses = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(gatePasses)
}

// ApproveGatePass approves a gate pass (from unloading tickets)
func (h *GatePassHandler) ApproveGatePass(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid gate pass ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateGatePassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.Service.ApproveGatePass(context.Background(), id, &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate gate pass cache
	cache.InvalidateGatePassCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Approved gate pass #%d - %d items at gate %s", id, req.ApprovedQuantity, req.GateNo)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "UPDATE",
		TargetType:  "gate_pass",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Gate pass approved successfully"})
}

// CompleteGatePass marks gate pass as completed (items physically taken)
func (h *GatePassHandler) CompleteGatePass(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid gate pass ID", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.Service.CompleteGatePass(context.Background(), id, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate gate pass and room caches (items leaving affects room occupancy)
	cache.InvalidateGatePassCaches(r.Context())
	cache.InvalidateRoomEntryCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Completed gate pass #%d - items physically taken out by customer", id)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "COMPLETE",
		TargetType:  "gate_pass",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Gate pass completed - items out"})
}

// RecordPickup records a partial pickup for an approved gate pass
func (h *GatePassHandler) RecordPickup(w http.ResponseWriter, r *http.Request) {
	var req models.RecordPickupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pickupID, err := h.Service.RecordPickup(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate gate pass and room caches (pickup affects room occupancy)
	cache.InvalidateGatePassCaches(r.Context())
	cache.InvalidateRoomEntryCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Recorded pickup for gate pass #%d - %d items picked up from Room %s, Floor %s",
		req.GatePassID, req.PickupQuantity, req.RoomNo, req.Floor)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "PICKUP",
		TargetType:  "gate_pass",
		TargetID:    &req.GatePassID,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Pickup recorded successfully",
		"pickup_id": pickupID,
	})
}

// GetPickupHistory retrieves pickup history for a gate pass
func (h *GatePassHandler) GetPickupHistory(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid gate pass ID", http.StatusBadRequest)
		return
	}

	pickups, err := h.Service.GetPickupHistory(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pickups)
}

// ListAllPickups retrieves all pickups with customer info for activity log
func (h *GatePassHandler) ListAllPickups(w http.ResponseWriter, r *http.Request) {
	pickups, err := h.Service.GetAllPickups(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if pickups == nil {
		pickups = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pickups)
}

// GetPickupHistoryByThock retrieves pickup history for a thock number (across all gate passes)
func (h *GatePassHandler) GetPickupHistoryByThock(w http.ResponseWriter, r *http.Request) {
	thockNumber := r.URL.Query().Get("thock_number")
	if thockNumber == "" {
		http.Error(w, "thock_number query parameter is required", http.StatusBadRequest)
		return
	}

	pickups, err := h.Service.GetPickupHistoryByThockNumber(context.Background(), thockNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if pickups == nil {
		pickups = []models.GatePassPickup{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pickups)
}

// GetExpiredGatePasses retrieves expired gate passes for admin logs
func (h *GatePassHandler) GetExpiredGatePasses(w http.ResponseWriter, r *http.Request) {
	expiredPasses, err := h.Service.GetExpiredGatePassLogs(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if expiredPasses == nil {
		expiredPasses = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expiredPasses)
}

// ListApprovedGatePasses returns approved/partially_completed gate passes for pickup UI
func (h *GatePassHandler) ListApprovedGatePasses(w http.ResponseWriter, r *http.Request) {
	// Run expiration check first
	h.Service.CheckAndExpireGatePasses(context.Background())

	// This would need a new repository method, but for now we can filter from all
	// TODO: Add optimized query for approved/partially_completed only
	allPasses, err := h.Service.ListAllGatePasses(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter for approved and partially_completed statuses
	var approvedPasses []map[string]interface{}
	for _, gp := range allPasses {
		status, ok := gp["status"].(string)
		if ok && (status == "approved" || status == "partially_completed") {
			approvedPasses = append(approvedPasses, gp)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(approvedPasses)
}

// ListMediaByThock retrieves all media (entry + pickup) for a specific thock number
// GET /api/gate-passes/media/by-thock?thock_number={thockNumber}
func (h *GatePassHandler) ListMediaByThock(w http.ResponseWriter, r *http.Request) {
	thockNumber := r.URL.Query().Get("thock_number")
	if thockNumber == "" {
		http.Error(w, "thock_number parameter is required", http.StatusBadRequest)
		return
	}

	media, err := h.Service.GetMediaByThockNumber(r.Context(), thockNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

// SaveMediaMetadata saves media metadata after file upload
// POST /api/gate-passes/media
func (h *GatePassHandler) SaveMediaMetadata(w http.ResponseWriter, r *http.Request) {
	var media models.GatePassMedia
	if err := json.NewDecoder(r.Body).Decode(&media); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	media.UploadedByUserID = &userID

	// Validate media type
	if media.MediaType != "entry" && media.MediaType != "pickup" {
		http.Error(w, "Invalid media_type. Must be 'entry' or 'pickup'", http.StatusBadRequest)
		return
	}

	// Validate file path doesn't contain directory traversal
	if strings.Contains(media.FilePath, "..") {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	if err := h.Service.SaveMediaMetadata(r.Context(), &media); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enqueue async cloud sync (non-blocking, best-effort)
	if h.SyncService != nil {
		var fileSize int64
		if media.FileSize != nil {
			fileSize = *media.FileSize
		}
		go h.SyncService.EnqueueMedia(context.Background(), "gate_pass", media.ID, media.FilePath, media.FileName, fileSize, media.ThockNumber, media.MediaType)
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}
