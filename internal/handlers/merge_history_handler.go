package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// MergeHistoryHandler handles merge and transfer history endpoints
type MergeHistoryHandler struct {
	CustomerRepo      *repositories.CustomerRepository
	EntryRepo         *repositories.EntryRepository
	ManagementLogRepo *repositories.EntryManagementLogRepository
}

// NewMergeHistoryHandler creates a new MergeHistoryHandler
func NewMergeHistoryHandler(customerRepo *repositories.CustomerRepository, entryRepo *repositories.EntryRepository, managementLogRepo *repositories.EntryManagementLogRepository) *MergeHistoryHandler {
	return &MergeHistoryHandler{
		CustomerRepo:      customerRepo,
		EntryRepo:         entryRepo,
		ManagementLogRepo: managementLogRepo,
	}
}

// MergeHistoryResponse contains all merge and transfer history data
type MergeHistoryResponse struct {
	MergeLogs          []*models.EntryManagementLog `json:"merge_logs"`
	MergedCustomers    []*MergedCustomerInfo        `json:"merged_customers"`
	TransferredEntries []*TransferredEntryInfo      `json:"transferred_entries"`
}

// MergedCustomerInfo contains merged customer details with target info
type MergedCustomerInfo struct {
	ID                   int                `json:"id"`
	Name                 string             `json:"name"`
	Phone                string             `json:"phone"`
	SO                   string             `json:"so"`
	Village              string             `json:"village"`
	MergedIntoCustomerID *int               `json:"merged_into_customer_id"`
	MergedAt             string             `json:"merged_at"`
	TargetCustomer       *models.Customer   `json:"target_customer,omitempty"`
}

// TransferredEntryInfo contains transferred entry details with customer info
type TransferredEntryInfo struct {
	ID                      int              `json:"id"`
	ThockNumber             string           `json:"thock_number"`
	Name                    string           `json:"name"`
	Phone                   string           `json:"phone"`
	Village                 string           `json:"village"`
	ExpectedQuantity        int              `json:"expected_quantity"`
	TransferredToCustomerID *int             `json:"transferred_to_customer_id"`
	TransferredAt           string           `json:"transferred_at"`
	CurrentCustomer         *models.Customer `json:"current_customer,omitempty"`
}

// GetMergeHistory returns all merged customers and transferred entries
func (h *MergeHistoryHandler) GetMergeHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Get merge logs from entry_management_logs (with full details)
	var mergeLogs []*models.EntryManagementLog
	if h.ManagementLogRepo != nil {
		logs, err := h.ManagementLogRepo.ListByType(ctx, "merge")
		if err == nil {
			mergeLogs = logs
		}
	}

	// Get merged customers
	mergedCustomers, err := h.CustomerRepo.GetMergedCustomers(ctx)
	if err != nil {
		http.Error(w, "Failed to get merged customers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get transferred entries
	transferredEntries, err := h.EntryRepo.GetTransferredEntries(ctx)
	if err != nil {
		http.Error(w, "Failed to get transferred entries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build response with enriched data
	mergedInfos := make([]*MergedCustomerInfo, 0, len(mergedCustomers))
	for _, c := range mergedCustomers {
		info := &MergedCustomerInfo{
			ID:                   c.ID,
			Name:                 c.Name,
			Phone:                c.Phone,
			SO:                   c.SO,
			Village:              c.Village,
			MergedIntoCustomerID: c.MergedIntoCustomerID,
		}
		if c.MergedAt != nil {
			info.MergedAt = c.MergedAt.Format("02/01/2006 15:04")
		}
		// Get target customer info
		if c.MergedIntoCustomerID != nil {
			target, err := h.CustomerRepo.Get(ctx, *c.MergedIntoCustomerID)
			if err == nil {
				info.TargetCustomer = target
			}
		}
		mergedInfos = append(mergedInfos, info)
	}

	transferredInfos := make([]*TransferredEntryInfo, 0, len(transferredEntries))
	for _, e := range transferredEntries {
		info := &TransferredEntryInfo{
			ID:                      e.ID,
			ThockNumber:             e.ThockNumber,
			Name:                    e.Name,
			Phone:                   e.Phone,
			Village:                 e.Village,
			ExpectedQuantity:        e.ExpectedQuantity,
			TransferredToCustomerID: e.TransferredToCustomerID,
		}
		if e.TransferredAt != nil {
			info.TransferredAt = e.TransferredAt.Format("02/01/2006 15:04")
		}
		// Get current customer info
		if e.TransferredToCustomerID != nil {
			current, err := h.CustomerRepo.Get(ctx, *e.TransferredToCustomerID)
			if err == nil {
				info.CurrentCustomer = current
			}
		}
		transferredInfos = append(transferredInfos, info)
	}

	response := MergeHistoryResponse{
		MergeLogs:          mergeLogs,
		MergedCustomers:    mergedInfos,
		TransferredEntries: transferredInfos,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UndoMerge reverses a customer merge
func (h *MergeHistoryHandler) UndoMerge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Parse request
	var req struct {
		CustomerID int `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.CustomerID == 0 {
		// Try URL parameter
		idStr := r.URL.Query().Get("id")
		if idStr != "" {
			id, err := strconv.Atoi(idStr)
			if err == nil {
				req.CustomerID = id
			}
		}
	}

	if req.CustomerID == 0 {
		http.Error(w, "customer_id is required", http.StatusBadRequest)
		return
	}

	// Get payment IDs from merge log (to know which payments to move back)
	var paymentIDs []int
	if h.ManagementLogRepo != nil {
		mergeLog, err := h.ManagementLogRepo.GetBySourceCustomer(ctx, req.CustomerID)
		if err == nil && mergeLog.MergeDetails != nil {
			for _, p := range mergeLog.MergeDetails.Payments {
				paymentIDs = append(paymentIDs, p.ID)
			}
		}
	}

	// Undo the merge (moves entries and payments back)
	err := h.CustomerRepo.UndoMerge(ctx, req.CustomerID, paymentIDs)
	if err != nil {
		http.Error(w, "Failed to undo merge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete the merge log entry
	if h.ManagementLogRepo != nil {
		_ = h.ManagementLogRepo.DeleteBySourceCustomer(ctx, req.CustomerID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Merge undone successfully - entries and payments restored",
	})
}

// UndoTransfer reverses an entry transfer
func (h *MergeHistoryHandler) UndoTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify admin access
	role, ok := middleware.GetRoleFromContext(ctx)
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Parse request
	var req struct {
		EntryID            int    `json:"entry_id"`
		OriginalCustomerID int    `json:"original_customer_id"`
		Name               string `json:"name"`
		Phone              string `json:"phone"`
		Village            string `json:"village"`
		SO                 string `json:"so"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.EntryID == 0 || req.OriginalCustomerID == 0 {
		http.Error(w, "entry_id and original_customer_id are required", http.StatusBadRequest)
		return
	}

	// Undo the transfer
	err := h.EntryRepo.UndoTransfer(ctx, req.EntryID, req.OriginalCustomerID, req.Name, req.Phone, req.Village, req.SO)
	if err != nil {
		http.Error(w, "Failed to undo transfer: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete the transfer log entry
	if h.ManagementLogRepo != nil {
		_ = h.ManagementLogRepo.DeleteByEntryID(ctx, req.EntryID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Transfer undone successfully - entry restored",
	})
}
