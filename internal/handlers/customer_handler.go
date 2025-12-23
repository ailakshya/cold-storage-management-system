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

type CustomerHandler struct {
	Service *services.CustomerService
}

func NewCustomerHandler(s *services.CustomerService) *CustomerHandler {
	return &CustomerHandler{Service: s}
}

func (h *CustomerHandler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	customer, err := h.Service.CreateCustomer(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (h *CustomerHandler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	customer, err := h.Service.GetCustomer(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (h *CustomerHandler) SearchByPhone(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		http.Error(w, "phone parameter is required", http.StatusBadRequest)
		return
	}

	// IDOR protection - only employees, admins, accountants, and guards can search customers
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized - role not found", http.StatusUnauthorized)
		return
	}

	if role != "admin" && role != "employee" && role != "accountant" && role != "guard" {
		http.Error(w, "Forbidden - access required", http.StatusForbidden)
		return
	}

	// Use fuzzy search to return multiple matching customers
	customers, err := h.Service.FuzzySearchByPhone(context.Background(), phone)
	if err != nil {
		// Return empty array instead of error
		customers = []*models.Customer{}
	}

	if customers == nil {
		customers = []*models.Customer{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func (h *CustomerHandler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := h.Service.ListCustomers(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func (h *CustomerHandler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	// IDOR protection - only admin and employees can update customers
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if role != "admin" && role != "employee" {
		http.Error(w, "Forbidden - admin or employee access required", http.StatusForbidden)
		return
	}

	var req models.UpdateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	customer, err := h.Service.UpdateCustomer(context.Background(), id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (h *CustomerHandler) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	// IDOR protection - only admin can delete customers
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "Forbidden - admin access required to delete customers", http.StatusForbidden)
		return
	}

	if err := h.Service.DeleteCustomer(context.Background(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
