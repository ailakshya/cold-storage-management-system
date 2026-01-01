package handlers

import (
	"context"
	"encoding/json"
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

const customersCacheTTL = 30 * time.Minute
const customersCacheKey = "customers:list"

type CustomerHandler struct {
	Service           *services.CustomerService
	ManagementLogRepo *repositories.EntryManagementLogRepository
	LedgerRepo        *repositories.LedgerRepository
}

func NewCustomerHandler(s *services.CustomerService, managementLogRepo *repositories.EntryManagementLogRepository) *CustomerHandler {
	h := &CustomerHandler{Service: s, ManagementLogRepo: managementLogRepo}

	// Register pre-warm callback for customer list (faster search on cold start)
	cache.RegisterPreWarm(customersCacheKey, func(ctx context.Context) ([]byte, error) {
		customers, err := h.Service.ListCustomers(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(customers)
	})

	return h
}

// SetLedgerRepo sets the ledger repository for cascading phone updates to ledger
func (h *CustomerHandler) SetLedgerRepo(ledgerRepo *repositories.LedgerRepository) {
	h.LedgerRepo = ledgerRepo
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

	// Invalidate customers cache
	cache.InvalidateCustomerCaches(r.Context())

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
	ctx := r.Context()

	// Try cache first
	if data, ok := cache.GetCached(ctx, customersCacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	customers, err := h.Service.ListCustomers(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the response
	data, _ := json.Marshal(customers)
	cache.SetCached(ctx, customersCacheKey, data, customersCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
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

	// Get old phone before update (for cascading to ledger)
	oldCustomer, _ := h.Service.GetCustomer(context.Background(), id)
	oldPhone := ""
	if oldCustomer != nil {
		oldPhone = oldCustomer.Phone
	}

	customer, err := h.Service.UpdateCustomer(context.Background(), id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cascade phone change to ledger entries if phone changed
	if h.LedgerRepo != nil && oldPhone != "" && oldPhone != customer.Phone {
		_, _ = h.LedgerRepo.UpdateCustomerPhone(context.Background(), oldPhone, customer.Phone)
	}

	// Invalidate customers cache
	cache.InvalidateCustomerCaches(r.Context())

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

	// Invalidate customers cache
	cache.InvalidateCustomerCaches(r.Context())

	w.WriteHeader(http.StatusNoContent)
}

// GetCustomerEntryCount returns the number of entries for a customer (for merge preview)
func (h *CustomerHandler) GetCustomerEntryCount(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	count, err := h.Service.GetEntryCount(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"customer_id":  id,
		"entry_count": count,
	})
}

// MergeCustomers merges source customer into target customer
// POST /api/customers/merge
func (h *CustomerHandler) MergeCustomers(w http.ResponseWriter, r *http.Request) {
	// Check permission: admin OR can_manage_entries
	if !middleware.HasManageEntriesAccess(r.Context()) {
		http.Error(w, "Forbidden: Manage entries permission required", http.StatusForbidden)
		return
	}

	// Get user ID for logging
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req models.MergeCustomersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get source customer details before merge (for logging)
	sourceCustomer, err := h.Service.GetCustomer(context.Background(), req.SourceCustomerID)
	if err != nil {
		http.Error(w, "Source customer not found", http.StatusNotFound)
		return
	}

	// Get target customer details (for logging)
	targetCustomer, err := h.Service.GetCustomer(context.Background(), req.TargetCustomerID)
	if err != nil {
		http.Error(w, "Target customer not found", http.StatusNotFound)
		return
	}

	response, err := h.Service.MergeCustomers(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the merge to management log with full details
	if h.ManagementLogRepo != nil {
		managementLog := &models.EntryManagementLog{
			PerformedByID:         userID,
			SourceCustomerID:      &req.SourceCustomerID,
			SourceCustomerName:    &sourceCustomer.Name,
			SourceCustomerPhone:   &sourceCustomer.Phone,
			SourceCustomerVillage: sourceCustomer.Village,
			SourceCustomerSO:      sourceCustomer.SO,
			TargetCustomerID:      &req.TargetCustomerID,
			TargetCustomerName:    &targetCustomer.Name,
			TargetCustomerPhone:   &targetCustomer.Phone,
			TargetCustomerVillage: targetCustomer.Village,
			TargetCustomerSO:      targetCustomer.SO,
			EntriesMoved:          &response.EntriesMoved,
			PaymentsMoved:         response.PaymentsMoved,
			MergeDetails:          response.MergeDetails,
		}
		h.ManagementLogRepo.CreateMergeLog(context.Background(), managementLog)
	}

	// Cascade phone change to ledger entries (source phone -> target phone)
	if h.LedgerRepo != nil && sourceCustomer.Phone != targetCustomer.Phone {
		_, _ = h.LedgerRepo.UpdateCustomerPhone(context.Background(), sourceCustomer.Phone, targetCustomer.Phone)
	}

	// Invalidate caches
	cache.InvalidateCustomerCaches(r.Context())
	cache.InvalidateEntryCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
