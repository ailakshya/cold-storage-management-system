package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

// SeasonHandler handles season-related HTTP requests
type SeasonHandler struct {
	service *services.SeasonService
}

// NewSeasonHandler creates a new season handler
func NewSeasonHandler(service *services.SeasonService) *SeasonHandler {
	return &SeasonHandler{service: service}
}

// InitiateSeason handles POST /api/season/initiate
func (h *SeasonHandler) InitiateSeason(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.InitiateSeasonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SeasonName == "" {
		http.Error(w, "Season name is required", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password is required for verification", http.StatusBadRequest)
		return
	}

	seasonReq, err := h.service.InitiateNewSeason(r.Context(), userID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(seasonReq)
}

// GetPending handles GET /api/season/pending
func (h *SeasonHandler) GetPending(w http.ResponseWriter, r *http.Request) {
	requests, err := h.service.GetPendingRequests(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if requests == nil {
		requests = []*models.SeasonRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// GetHistory handles GET /api/season/history
func (h *SeasonHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	requests, err := h.service.GetHistory(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if requests == nil {
		requests = []*models.SeasonRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// GetRequest handles GET /api/season/{id}
func (h *SeasonHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	request, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		http.Error(w, "Season request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// ApproveRequest handles POST /api/season/{id}/approve
func (h *SeasonHandler) ApproveRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req models.ApproveSeasonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password is required for verification", http.StatusBadRequest)
		return
	}

	if err := h.service.ApproveRequest(r.Context(), id, userID, req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Season request approved. Archive and reset process started.",
	})
}

// RejectRequest handles POST /api/season/{id}/reject
func (h *SeasonHandler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req models.RejectSeasonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.RejectRequest(r.Context(), id, userID, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Season request rejected",
	})
}
