package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/repositories"
)

type CustomerActivityLogHandler struct {
	Repo *repositories.CustomerActivityLogRepository
}

func NewCustomerActivityLogHandler(repo *repositories.CustomerActivityLogRepository) *CustomerActivityLogHandler {
	return &CustomerActivityLogHandler{Repo: repo}
}

// List returns paginated activity logs
func (h *CustomerActivityLogHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// Filter by action type if specified
	action := r.URL.Query().Get("action")

	var logs interface{}
	var total int
	var err error

	if action != "" {
		logs, total, err = h.Repo.ListByAction(ctx, action, limit, offset)
	} else {
		logs, total, err = h.Repo.List(ctx, limit, offset)
	}

	if err != nil {
		http.Error(w, "Failed to fetch activity logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetStats returns activity statistics
func (h *CustomerActivityLogHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.Repo.GetStats(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ListByCustomer returns activity logs for a specific customer
func (h *CustomerActivityLogHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	customerID, err := strconv.Atoi(r.URL.Query().Get("customer_id"))
	if err != nil || customerID <= 0 {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	logs, err := h.Repo.ListByCustomer(ctx, customerID, limit)
	if err != nil {
		http.Error(w, "Failed to fetch customer logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
