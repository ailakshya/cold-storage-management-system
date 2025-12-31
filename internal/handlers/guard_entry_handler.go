package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type GuardEntryHandler struct {
	Service         *services.GuardEntryService
	AdminActionRepo *repositories.AdminActionLogRepository
}

func NewGuardEntryHandler(s *services.GuardEntryService, adminActionRepo *repositories.AdminActionLogRepository) *GuardEntryHandler {
	return &GuardEntryHandler{
		Service:         s,
		AdminActionRepo: adminActionRepo,
	}
}

// CreateGuardEntry handles POST /api/guard/entries
func (h *GuardEntryHandler) CreateGuardEntry(w http.ResponseWriter, r *http.Request) {
	var req models.CreateGuardEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	entry, err := h.Service.CreateGuardEntry(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate guard entries cache
	cache.InvalidateGuardEntryCaches(r.Context())

	// Log guard entry creation
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	description := fmt.Sprintf("Guard recorded vehicle entry for %s - Seed: %d, Sell: %d bags", req.CustomerName, req.SeedQuantity, req.SellQuantity)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "CREATE",
		TargetType:  "guard_entry",
		TargetID:    &entry.ID,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// ListMyEntries handles GET /api/guard/entries - lists today's entries for the logged-in guard
func (h *GuardEntryHandler) ListMyEntries(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	entries, err := h.Service.ListTodayByUser(context.Background(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []*models.GuardEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// ListPendingEntries handles GET /api/guard/entries/pending
func (h *GuardEntryHandler) ListPendingEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.Service.ListPending(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []*models.GuardEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// ProcessGuardEntry handles PUT /api/guard/entries/{id}/process
func (h *GuardEntryHandler) ProcessGuardEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.Service.MarkAsProcessed(context.Background(), id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate guard entries cache
	cache.InvalidateGuardEntryCaches(r.Context())

	// Log guard entry processing
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	description := fmt.Sprintf("Guard entry #%d marked as processed", id)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "UPDATE",
		TargetType:  "guard_entry",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Guard entry marked as processed"})
}

// GetGuardEntry handles GET /api/guard/entries/{id}
func (h *GuardEntryHandler) GetGuardEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	entry, err := h.Service.GetGuardEntry(context.Background(), id)
	if err != nil {
		http.Error(w, "Guard entry not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// GetMyStats handles GET /api/guard/stats - get today's stats for the guard
func (h *GuardEntryHandler) GetMyStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	total, pending, err := h.Service.GetTodayCountByUser(context.Background(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"total":     total,
		"pending":   pending,
		"processed": total - pending,
	})
}

// DeleteGuardEntry handles DELETE /api/guard/entries/{id} - admin only
func (h *GuardEntryHandler) DeleteGuardEntry(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok || role != "admin" {
		http.Error(w, "Only admin can delete guard entries", http.StatusForbidden)
		return
	}

	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	if err := h.Service.DeleteGuardEntry(context.Background(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate guard entries cache
	cache.InvalidateGuardEntryCaches(r.Context())

	// Log guard entry deletion
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	description := fmt.Sprintf("Admin deleted guard entry #%d", id)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "DELETE",
		TargetType:  "guard_entry",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Guard entry deleted"})
}

// SkipToken handles POST /api/guard/skip-token
func (h *GuardEntryHandler) SkipToken(w http.ResponseWriter, r *http.Request) {
	var req models.SkipTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.Service.SkipToken(context.Background(), req.TokenNumber, req.Reason, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get next available token
	nextToken, _ := h.Service.GetNextAvailableToken(context.Background())

	// Log token skip
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	description := fmt.Sprintf("Guard skipped token #%d - Reason: %s", req.TokenNumber, req.Reason)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "SKIP",
		TargetType:  "token",
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Token skipped successfully",
		"skipped":    req.TokenNumber,
		"next_token": nextToken,
	})
}

// GetNextToken handles GET /api/guard/next-token
func (h *GuardEntryHandler) GetNextToken(w http.ResponseWriter, r *http.Request) {
	nextToken, err := h.Service.GetNextAvailableToken(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	skippedTokens, _ := h.Service.GetTodaySkippedTokens(context.Background())
	if skippedTokens == nil {
		skippedTokens = []int{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"next_token":     nextToken,
		"skipped_tokens": skippedTokens,
	})
}

// ProcessPortion handles PUT /api/guard/entries/{id}/process/{portion}
// portion can be "seed" or "sell"
func (h *GuardEntryHandler) ProcessPortion(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	portion := vars["portion"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.Service.MarkPortionProcessed(context.Background(), id, portion, userID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Invalidate guard entries cache
	cache.InvalidateGuardEntryCaches(r.Context())

	// Log portion processing
	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}
	description := fmt.Sprintf("Guard entry #%d: %s portion marked as processed", id, portion)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "UPDATE",
		TargetType:  "guard_entry",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": portion + " portion marked as processed"})
}
