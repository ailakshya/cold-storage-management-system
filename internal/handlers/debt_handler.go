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

// DebtHandler handles debt request endpoints
type DebtHandler struct {
	DebtService *services.DebtService
}

func NewDebtHandler(debtService *services.DebtService) *DebtHandler {
	return &DebtHandler{
		DebtService: debtService,
	}
}

// CreateDebtRequest creates a new debt request (employee/admin)
// POST /api/debt-requests
func (h *DebtHandler) CreateDebtRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get email from context to use as user name
	userName := "Unknown"
	if email, ok := middleware.GetEmailFromContext(ctx); ok && email != "" {
		userName = email
	}

	var req models.CreateDebtRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.CustomerPhone == "" || req.CustomerName == "" || req.ThockNumber == "" || req.RequestedQuantity <= 0 {
		http.Error(w, "Missing required fields: customer_phone, customer_name, thock_number, requested_quantity", http.StatusBadRequest)
		return
	}

	debtRequest, err := h.DebtService.CreateRequest(ctx, &req, userID, userName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(debtRequest)
}

// GetPendingRequests returns all pending debt requests (admin only)
// GET /api/debt-requests/pending
func (h *DebtHandler) GetPendingRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if role != "admin" {
		http.Error(w, "Forbidden - admin access required", http.StatusForbidden)
		return
	}

	requests, err := h.DebtService.GetPending(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Also get summary
	summary, _ := h.DebtService.GetPendingSummary(ctx)

	response := map[string]interface{}{
		"requests": requests,
		"summary":  summary,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetDebtRequest returns a single debt request by ID
// GET /api/debt-requests/{id}
func (h *DebtHandler) GetDebtRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	request, err := h.DebtService.GetByID(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if request == nil {
		http.Error(w, "Debt request not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// GetCustomerRequests returns all debt requests for a customer
// GET /api/debt-requests/customer/{phone}
func (h *DebtHandler) GetCustomerRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	phone := vars["phone"]
	if phone == "" {
		http.Error(w, "Phone number required", http.StatusBadRequest)
		return
	}

	requests, err := h.DebtService.GetByCustomer(ctx, phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// ApproveDebtRequest approves a debt request (admin only)
// PUT /api/debt-requests/{id}/approve
func (h *DebtHandler) ApproveDebtRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if role != "admin" {
		http.Error(w, "Forbidden - admin access required", http.StatusForbidden)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(ctx)
	userName := "Admin"
	if email, ok := middleware.GetEmailFromContext(ctx); ok && email != "" {
		userName = email
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	err = h.DebtService.Approve(ctx, id, userID, userName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated request
	request, _ := h.DebtService.GetByID(ctx, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// RejectDebtRequest rejects a debt request (admin only)
// PUT /api/debt-requests/{id}/reject
func (h *DebtHandler) RejectDebtRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if role != "admin" {
		http.Error(w, "Forbidden - admin access required", http.StatusForbidden)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(ctx)
	userName := "Admin"
	if email, ok := middleware.GetEmailFromContext(ctx); ok && email != "" {
		userName = email
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	var req models.RejectDebtRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RejectionReason == "" {
		http.Error(w, "Rejection reason is required", http.StatusBadRequest)
		return
	}

	err = h.DebtService.Reject(ctx, id, userID, userName, req.RejectionReason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated request
	request, _ := h.DebtService.GetByID(ctx, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(request)
}

// CheckDebtApproval checks if there's an approved debt request for customer+thock
// GET /api/debt-requests/check?phone={phone}&thock={thock}
func (h *DebtHandler) CheckDebtApproval(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	phone := r.URL.Query().Get("phone")
	thock := r.URL.Query().Get("thock")

	if phone == "" || thock == "" {
		http.Error(w, "Phone and thock are required", http.StatusBadRequest)
		return
	}

	canCreate, debtReq, balance, err := h.DebtService.CanCreateGatePass(ctx, phone, thock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"can_create_gate_pass": canCreate,
		"has_balance":          balance > 0,
		"balance":              balance,
		"debt_request":         debtReq,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAllRequests returns all debt requests with optional filters (admin only)
// GET /api/debt-requests
func (h *DebtHandler) GetAllRequests(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if role != "admin" {
		http.Error(w, "Forbidden - admin access required", http.StatusForbidden)
		return
	}

	filter := &models.DebtRequestFilter{
		CustomerPhone: r.URL.Query().Get("phone"),
		ThockNumber:   r.URL.Query().Get("thock"),
		Status:        models.DebtRequestStatus(r.URL.Query().Get("status")),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		filter.Limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset, _ = strconv.Atoi(offsetStr)
	}

	requests, err := h.DebtService.GetAll(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// GetPendingSummary returns summary of pending requests for dashboard
// GET /api/debt-requests/summary
func (h *DebtHandler) GetPendingSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	summary, err := h.DebtService.GetPendingSummary(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
