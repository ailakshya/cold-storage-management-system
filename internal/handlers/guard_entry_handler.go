package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type GuardEntryHandler struct {
	Service *services.GuardEntryService
}

func NewGuardEntryHandler(s *services.GuardEntryService) *GuardEntryHandler {
	return &GuardEntryHandler{Service: s}
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
