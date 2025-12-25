package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

const settingsCacheTTL = 24 * time.Hour

type SystemSettingHandler struct {
	Service *services.SystemSettingService
}

func NewSystemSettingHandler(service *services.SystemSettingService) *SystemSettingHandler {
	return &SystemSettingHandler{Service: service}
}

func (h *SystemSettingHandler) GetSetting(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	setting, err := h.Service.GetSetting(context.Background(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(setting)
}

func (h *SystemSettingHandler) ListSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cacheKey := "settings:list"

	// Try cache first
	if data, ok := cache.GetCached(ctx, cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	settings, err := h.Service.ListSettings(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the response
	data, _ := json.Marshal(settings)
	cache.SetCached(ctx, cacheKey, data, settingsCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

func (h *SystemSettingHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	var req models.UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	if err := h.Service.UpdateSetting(context.Background(), key, req.SettingValue, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalidate settings cache
	cache.InvalidateSettingCaches(r.Context())

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Setting updated successfully"})
}

// GetOperationMode returns the current system operation mode
func (h *SystemSettingHandler) GetOperationMode(w http.ResponseWriter, r *http.Request) {
	// Try to get from database, fallback to default
	setting, err := h.Service.GetSetting(context.Background(), "operation_mode")

	mode := "loading" // Default to loading mode
	message := "System is in loading mode - items being stored"

	if err == nil && setting != nil {
		mode = setting.SettingValue
		switch mode {
		case "loading":
			message = "System is in loading mode - items being stored"
		case "unloading":
			message = "System is in unloading mode - items being dispatched"
		case "maintenance":
			message = "System is in maintenance mode"
		case "readonly":
			message = "System is in read-only mode"
		case "emergency":
			message = "System is in emergency mode"
		default:
			mode = "loading"
			message = "System is in loading mode - items being stored"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mode":    mode,
		"message": message,
	})
}
