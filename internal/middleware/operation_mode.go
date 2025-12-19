package middleware

import (
	"context"
	"net/http"

	"cold-backend/internal/repositories"
)

// OperationModeMiddleware handles operation mode restrictions
type OperationModeMiddleware struct {
	settingsRepo *repositories.SystemSettingRepository
}

// NewOperationModeMiddleware creates a new operation mode middleware
func NewOperationModeMiddleware(settingsRepo *repositories.SystemSettingRepository) *OperationModeMiddleware {
	return &OperationModeMiddleware{settingsRepo: settingsRepo}
}

// getOperationMode fetches the current operation mode from database
func (m *OperationModeMiddleware) getOperationMode(ctx context.Context) string {
	setting, err := m.settingsRepo.Get(ctx, "operation_mode")
	if err != nil {
		return "loading" // Default to loading mode
	}
	return setting.SettingValue
}

// RequireLoadingMode blocks request if system is in unloading mode (for non-admins)
// Use this for: entry creation, room entry operations
func (m *OperationModeMiddleware) RequireLoadingMode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is admin (admins bypass mode restrictions)
		role, ok := GetRoleFromContext(r.Context())
		if ok && role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		// Check current mode
		mode := m.getOperationMode(r.Context())
		if mode == "unloading" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "This operation is not available in unloading mode. The system is currently in unloading mode for gate passes only."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireUnloadingMode blocks request if system is in loading mode (for non-admins)
// Use this for: gate pass creation, unloading tickets
func (m *OperationModeMiddleware) RequireUnloadingMode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is admin (admins bypass mode restrictions)
		role, ok := GetRoleFromContext(r.Context())
		if ok && role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		// Check current mode
		mode := m.getOperationMode(r.Context())
		if mode == "loading" || mode == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "This operation is not available in loading mode. The system is currently in loading mode for entries only."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetOperationModeInfo returns current mode and restrictions for the user
func (m *OperationModeMiddleware) GetOperationModeInfo(ctx context.Context, role string) map[string]interface{} {
	mode := m.getOperationMode(ctx)
	isAdmin := role == "admin"

	var hiddenFeatures, availableFeatures []string

	if mode == "loading" {
		hiddenFeatures = []string{"gate-pass-entry", "unloading-tickets"}
		availableFeatures = []string{"entry-room", "room-config-1", "item-search", "events"}
	} else {
		hiddenFeatures = []string{"entry-room", "room-config-1"}
		availableFeatures = []string{"gate-pass-entry", "unloading-tickets", "item-search", "events"}
	}

	// Admins see everything
	if isAdmin {
		hiddenFeatures = []string{}
		availableFeatures = []string{"entry-room", "room-config-1", "gate-pass-entry", "unloading-tickets", "item-search", "events"}
	}

	return map[string]interface{}{
		"mode":               mode,
		"is_admin":           isAdmin,
		"hidden_features":    hiddenFeatures,
		"available_features": availableFeatures,
	}
}
