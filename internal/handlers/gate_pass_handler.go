package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type GatePassHandler struct {
	Service         *services.GatePassService
	AdminActionRepo *repositories.AdminActionLogRepository
}

func NewGatePassHandler(service *services.GatePassService, adminActionRepo *repositories.AdminActionLogRepository) *GatePassHandler {
	return &GatePassHandler{
		Service:         service,
		AdminActionRepo: adminActionRepo,
	}
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

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Issued gate pass for truck %s - %d items requested", req.TruckNumber, req.RequestedQuantity)
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

	err := h.Service.RecordPickup(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	json.NewEncoder(w).Encode(map[string]string{"message": "Pickup recorded successfully"})
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

// GetExpiredGatePasses retrieves expired gate passes for admin logs
func (h *GatePassHandler) GetExpiredGatePasses(w http.ResponseWriter, r *http.Request) {
	expiredPasses, err := h.Service.GetExpiredGatePassLogs(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
