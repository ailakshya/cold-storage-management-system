package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type EntryHandler struct {
	Service     *services.EntryService
	EditLogRepo *repositories.EntryEditLogRepository
}

func NewEntryHandler(s *services.EntryService, editLogRepo *repositories.EntryEditLogRepository) *EntryHandler {
	return &EntryHandler{
		Service:     s,
		EditLogRepo: editLogRepo,
	}
}

func (h *EntryHandler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	entry, err := h.Service.CreateEntry(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalidate entries cache
	cache.InvalidateEntryCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (h *EntryHandler) GetEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	entry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (h *EntryHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.Service.ListEntries(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if entries == nil {
		entries = []*models.Entry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *EntryHandler) ListEntriesByCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["customer_id"]
	customerID, _ := strconv.Atoi(idStr)

	entries, err := h.Service.ListEntriesByCustomer(context.Background(), customerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if entries == nil {
		entries = []*models.Entry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *EntryHandler) GetCountByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "category query parameter is required", http.StatusBadRequest)
		return
	}

	count, err := h.Service.GetCountByCategory(context.Background(), category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Calculate next number based on category
	// SEED: 1-1500 range (next = count + 1)
	// SELL: 1501-3000 range (next = 1501 + count)
	var next int
	if category == "seed" {
		next = count + 1
	} else if category == "sell" {
		next = 1501 + count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"category": category,
		"count":    count,
		"next":     next,
	})
}

func (h *EntryHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context for logging
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req models.UpdateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get old entry before update for logging
	oldEntry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	// Update the entry
	if err := h.Service.UpdateEntry(context.Background(), id, &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the edit if EditLogRepo is available
	if h.EditLogRepo != nil {
		editLog := &models.EntryEditLog{
			EntryID:        id,
			EditedByUserID: userID,
		}

		// Only log fields that changed
		if oldEntry.Name != req.Name {
			editLog.OldName = &oldEntry.Name
			editLog.NewName = &req.Name
		}
		if oldEntry.Phone != req.Phone {
			editLog.OldPhone = &oldEntry.Phone
			editLog.NewPhone = &req.Phone
		}
		if oldEntry.Village != req.Village {
			editLog.OldVillage = &oldEntry.Village
			editLog.NewVillage = &req.Village
		}
		if oldEntry.SO != req.SO {
			editLog.OldSO = &oldEntry.SO
			editLog.NewSO = &req.SO
		}
		if oldEntry.ExpectedQuantity != req.ExpectedQuantity {
			editLog.OldExpectedQuantity = &oldEntry.ExpectedQuantity
			editLog.NewExpectedQuantity = &req.ExpectedQuantity
		}
		if oldEntry.ThockCategory != req.ThockCategory {
			editLog.OldThockCategory = &oldEntry.ThockCategory
			editLog.NewThockCategory = &req.ThockCategory
		}
		if oldEntry.Remark != req.Remark {
			editLog.OldRemark = &oldEntry.Remark
			editLog.NewRemark = &req.Remark
		}

		// Only create log if something changed
		if editLog.OldName != nil || editLog.OldPhone != nil || editLog.OldVillage != nil ||
			editLog.OldSO != nil || editLog.OldExpectedQuantity != nil ||
			editLog.OldThockCategory != nil || editLog.OldRemark != nil {
			h.EditLogRepo.CreateEditLog(context.Background(), editLog)
		}
	}

	// Invalidate entries cache
	cache.InvalidateEntryCaches(r.Context())

	// Fetch updated entry to return the new thock number
	entry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}
