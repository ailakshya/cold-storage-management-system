package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type DetectionHandler struct {
	Service *services.DetectionService
}

func NewDetectionHandler(service *services.DetectionService) *DetectionHandler {
	return &DetectionHandler{Service: service}
}

// CreateSession receives a completed detection session from the Python inference service.
// POST /api/detections
func (h *DetectionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDetectionSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.Service.CreateSession(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// GetSession returns a single detection session by ID.
// GET /api/detections/{id}
func (h *DetectionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	session, err := h.Service.GetSession(context.Background(), id)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// ListSessions returns paginated detection sessions.
// GET /api/detections?gate_id=gate1&status=completed&limit=50&offset=0
func (h *DetectionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	gateID := q.Get("gate_id")
	status := q.Get("status")

	limit := 50
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(q.Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	sessions, total, err := h.Service.ListSessions(context.Background(), gateID, status, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure empty array instead of null
	if sessions == nil {
		sessions = []*models.DetectionSession{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// UpdateSession updates a detection session (link to gate pass, add manual count).
// PUT /api/detections/{id}
func (h *DetectionHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateDetectionSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Service.UpdateSession(context.Background(), id, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// GetRecentByGate returns the latest sessions for a specific gate.
// GET /api/detections/gate/{gate_id}
func (h *DetectionHandler) GetRecentByGate(w http.ResponseWriter, r *http.Request) {
	gateID := mux.Vars(r)["gate_id"]
	if gateID == "" {
		http.Error(w, "gate_id is required", http.StatusBadRequest)
		return
	}

	limit := 10
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}

	sessions, err := h.Service.GetRecentByGate(context.Background(), gateID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if sessions == nil {
		sessions = []*models.DetectionSession{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// GetDailySummary returns aggregated detection stats.
// GET /api/detections/summary?from=2026-01-01&to=2026-01-31
func (h *DetectionHandler) GetDailySummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from, err := time.Parse("2006-01-02", q.Get("from"))
	if err != nil {
		// Default to last 7 days
		from = time.Now().AddDate(0, 0, -7)
	}

	to, err := time.Parse("2006-01-02", q.Get("to"))
	if err != nil {
		to = time.Now().AddDate(0, 0, 1) // tomorrow to include today
	}

	results, err := h.Service.GetDailySummary(context.Background(), from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
