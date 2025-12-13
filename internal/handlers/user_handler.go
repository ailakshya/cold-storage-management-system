package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type UserHandler struct {
	Service *services.UserService
}

func NewUserHandler(s *services.UserService) *UserHandler {
	return &UserHandler{Service: s}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
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
	users, err := h.Service.ListUsers(context.Background())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// UpdateUser updates an existing user
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	if err := h.Service.DeleteUser(context.Background(), id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
