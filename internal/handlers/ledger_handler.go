package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

// LedgerHandler handles ledger-related endpoints
type LedgerHandler struct {
	LedgerService *services.LedgerService
}

func NewLedgerHandler(ledgerService *services.LedgerService) *LedgerHandler {
	return &LedgerHandler{
		LedgerService: ledgerService,
	}
}

// GetCustomerLedger returns all ledger entries for a specific customer
// GET /api/ledger/customer/{phone}
func (h *LedgerHandler) GetCustomerLedger(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	phone := vars["phone"]
	if phone == "" {
		http.Error(w, "Phone number required", http.StatusBadRequest)
		return
	}

	// Parse pagination
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 100
	}

	entries, err := h.LedgerService.GetCustomerLedger(ctx, phone, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// GetCustomerBalance returns the current balance for a customer
// GET /api/ledger/balance/{phone}
func (h *LedgerHandler) GetCustomerBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	phone := vars["phone"]
	if phone == "" {
		http.Error(w, "Phone number required", http.StatusBadRequest)
		return
	}

	balance, err := h.LedgerService.GetBalance(ctx, phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"phone":   phone,
		"balance": balance,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetCustomerSummary returns balance summary for a customer
// GET /api/ledger/summary/{phone}
func (h *LedgerHandler) GetCustomerSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	phone := vars["phone"]
	if phone == "" {
		http.Error(w, "Phone number required", http.StatusBadRequest)
		return
	}

	summary, err := h.LedgerService.GetCustomerSummary(ctx, phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if summary == nil {
		http.Error(w, "No ledger entries for this customer", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetAuditTrail returns all ledger entries with optional filters (admin only)
// GET /api/ledger/audit
func (h *LedgerHandler) GetAuditTrail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin/accountant access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := ctx.Value(middleware.HasAccountantAccessKey).(bool)
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - admin/accountant access required", http.StatusForbidden)
		return
	}

	// Parse filters
	filter := &models.LedgerFilter{
		CustomerPhone: r.URL.Query().Get("phone"),
		EntryType:     models.LedgerEntryType(r.URL.Query().Get("type")),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		filter.Limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset, _ = strconv.Atoi(offsetStr)
	}

	// Parse date filters
	if startDate := r.URL.Query().Get("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}
	if endDate := r.URL.Query().Get("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// Set to end of day
			t = t.Add(24*time.Hour - time.Second)
			filter.EndDate = &t
		}
	}

	entries, err := h.LedgerService.GetAllEntries(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to audit entries format
	auditEntries := make([]models.AuditEntry, len(entries))
	for i, e := range entries {
		auditEntries[i] = models.AuditEntry{
			ID:              e.ID,
			Date:            e.CreatedAt,
			CustomerPhone:   e.CustomerPhone,
			CustomerName:    e.CustomerName,
			CustomerSO:      e.CustomerSO,
			EntryType:       e.EntryType,
			Description:     e.Description,
			Debit:           e.Debit,
			Credit:          e.Credit,
			RunningBalance:  e.RunningBalance,
			CreatedByName:   e.CreatedByName,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(auditEntries)
}

// GetDebtors returns customers with outstanding balance (admin only)
// GET /api/ledger/debtors
func (h *LedgerHandler) GetDebtors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin/accountant access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := ctx.Value(middleware.HasAccountantAccessKey).(bool)
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - admin/accountant access required", http.StatusForbidden)
		return
	}

	debtors, err := h.LedgerService.GetDebtors(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate total outstanding
	var totalOutstanding float64
	for _, d := range debtors {
		totalOutstanding += d.CurrentBalance
	}

	response := map[string]interface{}{
		"debtors":           debtors,
		"total_debtors":     len(debtors),
		"total_outstanding": totalOutstanding,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetAllBalances returns balance summaries for all customers (admin only)
// GET /api/ledger/balances
func (h *LedgerHandler) GetAllBalances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin/accountant access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := ctx.Value(middleware.HasAccountantAccessKey).(bool)
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - admin/accountant access required", http.StatusForbidden)
		return
	}

	summaries, err := h.LedgerService.GetAllCustomerBalances(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

// CreateEntry creates a new ledger entry (admin only - for manual adjustments)
// POST /api/ledger/entry
func (h *LedgerHandler) CreateEntry(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateLedgerEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	req.CreatedByUserID = userID

	entry, err := h.LedgerService.CreateEntry(ctx, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// GetTotalsByType returns sum of amounts by entry type (admin dashboard)
// GET /api/ledger/totals
func (h *LedgerHandler) GetTotalsByType(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin/accountant access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := ctx.Value(middleware.HasAccountantAccessKey).(bool)
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - admin/accountant access required", http.StatusForbidden)
		return
	}

	totals, err := h.LedgerService.GetTotalsByType(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(totals)
}
