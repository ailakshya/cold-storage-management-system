package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

const usersCacheTTL = 1 * time.Hour

type UserHandler struct {
	Service         *services.UserService
	AdminActionRepo *repositories.AdminActionLogRepository
}

func NewUserHandler(s *services.UserService, adminActionRepo *repositories.AdminActionLogRepository) *UserHandler {
	return &UserHandler{
		Service:         s,
		AdminActionRepo: adminActionRepo,
	}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// CRITICAL FIX: Verify admin role before allowing user creation
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized - role not found", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "Forbidden - admin access required to create users", http.StatusForbidden)
		return
	}

	// Get admin user ID from context
	adminUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := &models.User{
		Name:                req.Name,
		Email:               req.Email,
		PasswordHash:        req.Password, // Will be hashed in service layer
		Role:                req.Role,
		HasAccountantAccess: req.HasAccountantAccess,
	}

	if err := h.Service.CreateUser(context.Background(), user); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Invalidate users cache
	cache.InvalidateUserCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Created user: %s (%s) with role: %s", user.Name, user.Email, user.Role)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: adminUserID,
		ActionType:  "CREATE",
		TargetType:  "user",
		TargetID:    &user.ID,
		Description: description,
		IPAddress:   &ipAddress,
	})

	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	user, err := h.Service.GetUser(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// ListUsers returns all users
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// CRITICAL FIX: Verify admin role before listing all users
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized - role not found", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "Forbidden - admin access required to list users", http.StatusForbidden)
		return
	}

	cacheKey := "users:list"

	// Try cache first
	if data, ok := cache.GetCached(ctx, cacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	users, err := h.Service.ListUsers(context.Background())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Cache the response
	data, _ := json.Marshal(users)
	cache.SetCached(ctx, cacheKey, data, usersCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// UpdateUser updates an existing user
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	// CRITICAL FIX: Verify admin role before updating users
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized - role not found", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "Forbidden - admin access required to update users", http.StatusForbidden)
		return
	}

	// Get admin user ID from context
	adminUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get old user data for logging
	oldUser, _ := h.Service.GetUser(context.Background(), id)

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user := &models.User{
		ID:                  id,
		Name:                req.Name,
		Email:               req.Email,
		PasswordHash:        req.Password, // Will be hashed in service layer if provided
		Role:                req.Role,
		HasAccountantAccess: req.HasAccountantAccess,
	}

	if err := h.Service.UpdateUser(context.Background(), user); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Invalidate users cache
	cache.InvalidateUserCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := fmt.Sprintf("Updated user: %s (%s)", user.Name, user.Email)

	// Track specific changes
	var changes []string
	if oldUser != nil {
		if oldUser.Role != user.Role {
			changes = append(changes, fmt.Sprintf("role: %s → %s", oldUser.Role, user.Role))
		}
		if oldUser.HasAccountantAccess != user.HasAccountantAccess {
			changes = append(changes, fmt.Sprintf("accountant access: %v → %v", oldUser.HasAccountantAccess, user.HasAccountantAccess))
		}
	}

	if len(changes) > 0 {
		description += fmt.Sprintf(" [Changes: %v]", changes)
	}

	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: adminUserID,
		ActionType:  "UPDATE",
		TargetType:  "user",
		TargetID:    &user.ID,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	// Get admin user ID from context
	adminUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user data before deletion for logging
	user, _ := h.Service.GetUser(context.Background(), id)

	if err := h.Service.DeleteUser(context.Background(), id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Invalidate users cache
	cache.InvalidateUserCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	description := "Deleted user"
	if user != nil {
		description = fmt.Sprintf("Deleted user: %s (%s)", user.Name, user.Email)
	}

	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: adminUserID,
		ActionType:  "DELETE",
		TargetType:  "user",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.WriteHeader(http.StatusNoContent)
}

// ToggleActiveStatus toggles the is_active status of a user
func (h *UserHandler) ToggleActiveStatus(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	// Get admin user ID from context
	adminUserID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user data for logging
	user, _ := h.Service.GetUser(context.Background(), id)

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Service.ToggleActiveStatus(context.Background(), id, req.IsActive); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Invalidate users cache
	cache.InvalidateUserCaches(r.Context())

	// Log admin action
	ipAddress := getIPAddress(r)
	status := "activated"
	if !req.IsActive {
		status = "paused"
	}

	description := fmt.Sprintf("User status changed to %s", status)
	if user != nil {
		description = fmt.Sprintf("%s user: %s (%s)", status, user.Name, user.Email)
	}

	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: adminUserID,
		ActionType:  "TOGGLE_STATUS",
		TargetType:  "user",
		TargetID:    &id,
		Description: description,
		IPAddress:   &ipAddress,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"is_active": req.IsActive,
		"message":   "User status updated successfully",
	})
}
