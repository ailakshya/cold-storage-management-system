package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"cold-backend/internal/services"
)

// DeploymentHandler handles deployment API endpoints
type DeploymentHandler struct {
	service *services.DeploymentService
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(service *services.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{service: service}
}

// ListDeployments returns all deployment configurations
func (h *DeploymentHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	configs, err := h.service.ListDeploymentConfigs(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deployments": configs,
	})
}

// GetDeployment returns a deployment configuration with history
func (h *DeploymentHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	config, err := h.service.GetDeploymentConfig(r.Context(), id)
	if err != nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	history, _ := h.service.GetDeploymentHistory(r.Context(), id, 10)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"deployment": config,
		"history":    history,
	})
}

// GetDeploymentHistory returns deployment history
func (h *DeploymentHandler) GetDeploymentHistory(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	history, err := h.service.GetDeploymentHistory(r.Context(), id, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"history": history,
	})
}

// DeployRequest represents a deployment request
type DeployRequest struct {
	Version       string   `json:"version"`
	SkipBuild     bool     `json:"skip_build"`
	DeployTargets []string `json:"deploy_targets"`
}

// Deploy triggers a new deployment (returns SSE stream)
func (h *DeploymentHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	// Parse request
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body for default deployment
		req = DeployRequest{}
	}

	// Get user ID from context (set by auth middleware)
	userID := 0
	if uid, ok := r.Context().Value("user_id").(int); ok {
		userID = uid
	}

	// Build options
	opts := services.DeployOptions{
		Version:       req.Version,
		SkipBuild:     req.SkipBuild,
		DeployTargets: req.DeployTargets,
		UserID:        userID,
	}

	// Start deployment
	historyID, progressChan, err := h.service.Deploy(r.Context(), id, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback to non-streaming response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"history_id": historyID,
			"message":    "Deployment started",
		})
		return
	}

	// Send initial event
	fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
		"type":       "started",
		"history_id": historyID,
	}))
	flusher.Flush()

	// Stream progress updates
	for progress := range progressChan {
		data := mustJSON(map[string]interface{}{
			"type":     "progress",
			"step":     progress.Step,
			"progress": progress.Progress,
			"message":  progress.Message,
			"error":    progress.Error,
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	// Send completion event
	fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
		"type": "done",
	}))
	flusher.Flush()
}

// DeploySync triggers a deployment and returns JSON result (non-streaming)
func (h *DeploymentHandler) DeploySync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = DeployRequest{}
	}

	userID := 0
	if uid, ok := r.Context().Value("user_id").(int); ok {
		userID = uid
	}

	opts := services.DeployOptions{
		Version:       req.Version,
		SkipBuild:     req.SkipBuild,
		DeployTargets: req.DeployTargets,
		UserID:        userID,
	}

	historyID, progressChan, err := h.service.Deploy(r.Context(), id, opts)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Collect all progress messages
	var lastProgress services.DeployProgress
	for progress := range progressChan {
		lastProgress = progress
	}

	success := lastProgress.Step == "complete"
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    success,
		"history_id": historyID,
		"step":       lastProgress.Step,
		"progress":   lastProgress.Progress,
		"message":    lastProgress.Message,
		"error":      lastProgress.Error,
	})
}

// Rollback initiates a rollback
func (h *DeploymentHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
		return
	}

	userID := 0
	if uid, ok := r.Context().Value("user_id").(int); ok {
		userID = uid
	}

	if err := h.service.Rollback(r.Context(), id, userID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Rollback initiated",
	})
}

// GetDeploymentStatus returns current deployment job status (SSE)
func (h *DeploymentHandler) GetDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	historyIDStr := r.PathValue("historyId")
	historyID, err := strconv.Atoi(historyIDStr)
	if err != nil {
		http.Error(w, "Invalid history ID", http.StatusBadRequest)
		return
	}

	job, ok := h.service.GetJobStatus(historyID)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active": false,
		})
		return
	}

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream from job's status channel
	for progress := range job.Status {
		data := mustJSON(map[string]interface{}{
			"type":     "progress",
			"step":     progress.Step,
			"progress": progress.Progress,
			"message":  progress.Message,
			"error":    progress.Error,
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: %s\n\n", mustJSON(map[string]interface{}{
		"type": "done",
	}))
	flusher.Flush()
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
