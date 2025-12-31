package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type EntryHandler struct {
	Service           *services.EntryService
	EditLogRepo       *repositories.EntryEditLogRepository
	ManagementLogRepo *repositories.EntryManagementLogRepository
	AdminActionRepo   *repositories.AdminActionLogRepository
	SettingService    *services.SystemSettingService
}

func NewEntryHandler(s *services.EntryService, editLogRepo *repositories.EntryEditLogRepository, managementLogRepo *repositories.EntryManagementLogRepository, adminActionRepo *repositories.AdminActionLogRepository) *EntryHandler {
	return &EntryHandler{
		Service:           s,
		EditLogRepo:       editLogRepo,
		ManagementLogRepo: managementLogRepo,
		AdminActionRepo:   adminActionRepo,
	}
}

// SetSettingService sets the SystemSettingService for skip range calculation
func (h *EntryHandler) SetSettingService(ss *services.SystemSettingService) {
	h.SettingService = ss
}

func (h *EntryHandler) CreateEntry(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	entry, err := h.Service.CreateEntry(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log entry creation
	description := "Created unloading ticket " + entry.ThockNumber + " for " + req.Name + " (" + req.Village + ") - Qty: " + strconv.Itoa(req.ExpectedQuantity)
	if req.Remark != "" {
		description += " | Remark: " + req.Remark
	}
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "CREATE",
		TargetType:  "entry",
		TargetID:    &entry.ID,
		Description: description,
	})

	// Invalidate entries cache
	cache.InvalidateEntryCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (h *EntryHandler) GetEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	entry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (h *EntryHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.Service.ListEntries(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if entries == nil {
		entries = []*models.Entry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *EntryHandler) ListEntriesByCustomer(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["customer_id"]
	customerID, _ := strconv.Atoi(idStr)

	entries, err := h.Service.ListEntriesByCustomer(context.Background(), customerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array instead of null
	if entries == nil {
		entries = []*models.Entry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *EntryHandler) GetCountByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category == "" {
		http.Error(w, "category query parameter is required", http.StatusBadRequest)
		return
	}

	count, err := h.Service.GetCountByCategory(context.Background(), category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Calculate next number based on category
	// SEED: 1-1500 range (next = count + 1)
	// SELL: 1501-3000 range (next = 1501 + count)
	var next int
	if category == "seed" {
		next = count + 1
	} else if category == "sell" {
		next = 1501 + count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"category": category,
		"count":    count,
		"next":     next,
	})
}

func (h *EntryHandler) UpdateEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context for logging
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req models.UpdateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get old entry before update for logging
	oldEntry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	// Update the entry
	if err := h.Service.UpdateEntry(context.Background(), id, &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the edit if EditLogRepo is available
	if h.EditLogRepo != nil {
		editLog := &models.EntryEditLog{
			EntryID:        id,
			EditedByUserID: userID,
		}

		// Only log fields that changed
		if oldEntry.Name != req.Name {
			editLog.OldName = &oldEntry.Name
			editLog.NewName = &req.Name
		}
		if oldEntry.Phone != req.Phone {
			editLog.OldPhone = &oldEntry.Phone
			editLog.NewPhone = &req.Phone
		}
		if oldEntry.Village != req.Village {
			editLog.OldVillage = &oldEntry.Village
			editLog.NewVillage = &req.Village
		}
		if oldEntry.SO != req.SO {
			editLog.OldSO = &oldEntry.SO
			editLog.NewSO = &req.SO
		}
		if oldEntry.ExpectedQuantity != req.ExpectedQuantity {
			editLog.OldExpectedQuantity = &oldEntry.ExpectedQuantity
			editLog.NewExpectedQuantity = &req.ExpectedQuantity
		}
		if oldEntry.ThockCategory != req.ThockCategory {
			editLog.OldThockCategory = &oldEntry.ThockCategory
			editLog.NewThockCategory = &req.ThockCategory
		}
		if oldEntry.Remark != req.Remark {
			editLog.OldRemark = &oldEntry.Remark
			editLog.NewRemark = &req.Remark
		}

		// Only create log if something changed
		if editLog.OldName != nil || editLog.OldPhone != nil || editLog.OldVillage != nil ||
			editLog.OldSO != nil || editLog.OldExpectedQuantity != nil ||
			editLog.OldThockCategory != nil || editLog.OldRemark != nil {
			h.EditLogRepo.CreateEditLog(context.Background(), editLog)
		}
	}

	// Invalidate entries cache
	cache.InvalidateEntryCaches(r.Context())

	// Fetch updated entry to return the new thock number
	entry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// ReassignEntry reassigns an entry to a different customer
// PUT /api/entries/{id}/reassign
func (h *EntryHandler) ReassignEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

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

	var req models.ReassignEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewCustomerID <= 0 {
		http.Error(w, "new_customer_id is required", http.StatusBadRequest)
		return
	}

	// Get old entry for logging
	oldEntry, err := h.Service.GetEntry(context.Background(), id)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	// Check if already assigned to this customer
	if oldEntry.CustomerID == req.NewCustomerID && req.FamilyMemberID == nil {
		http.Error(w, "Entry is already assigned to this customer", http.StatusBadRequest)
		return
	}

	// Reassign the entry (with optional family member)
	entry, newCustomer, err := h.Service.ReassignEntry(context.Background(), id, req.NewCustomerID, req.FamilyMemberID, req.FamilyMemberName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the reassignment to entry edit log
	if h.EditLogRepo != nil {
		editLog := &models.EntryEditLog{
			EntryID:        id,
			EditedByUserID: userID,
		}

		// Log all changed fields (customer reassignment changes all denormalized fields)
		if oldEntry.Name != newCustomer.Name {
			editLog.OldName = &oldEntry.Name
			editLog.NewName = &newCustomer.Name
		}
		if oldEntry.Phone != newCustomer.Phone {
			editLog.OldPhone = &oldEntry.Phone
			editLog.NewPhone = &newCustomer.Phone
		}
		if oldEntry.Village != newCustomer.Village {
			editLog.OldVillage = &oldEntry.Village
			editLog.NewVillage = &newCustomer.Village
		}
		if oldEntry.SO != newCustomer.SO {
			editLog.OldSO = &oldEntry.SO
			editLog.NewSO = &newCustomer.SO
		}

		// Create log if something changed
		if editLog.OldName != nil || editLog.OldPhone != nil || editLog.OldVillage != nil || editLog.OldSO != nil {
			h.EditLogRepo.CreateEditLog(context.Background(), editLog)
		}
	}

	// Log the reassignment to management log (separate section)
	if h.ManagementLogRepo != nil {
		oldCustomerID := oldEntry.CustomerID
		managementLog := &models.EntryManagementLog{
			PerformedByID:    userID,
			EntryID:          &id,
			ThockNumber:      &oldEntry.ThockNumber,
			OldCustomerID:    &oldCustomerID,
			OldCustomerName:  &oldEntry.Name,
			OldCustomerPhone: &oldEntry.Phone,
			NewCustomerID:    &req.NewCustomerID,
			NewCustomerName:  &newCustomer.Name,
			NewCustomerPhone: &newCustomer.Phone,
		}
		h.ManagementLogRepo.CreateReassignLog(context.Background(), managementLog)
	}

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())
	cache.InvalidateCustomerCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// SkipRange for JSON parsing
type SkipRangeEntry struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// GetNextThockPreview returns the next thock numbers for both categories considering skip ranges
func (h *EntryHandler) GetNextThockPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current counts for display
	seedCount, _ := h.Service.GetCountByCategory(ctx, "seed")
	sellCount, _ := h.Service.GetCountByCategory(ctx, "sell")

	// Get max thock numbers (more accurate when skip ranges exist)
	maxSeed, _ := h.Service.GetMaxThockNumber(ctx, "seed")
	maxSell, _ := h.Service.GetMaxThockNumber(ctx, "sell")

	// Calculate base next numbers from MAX
	nextSeed := maxSeed + 1
	nextSell := maxSell + 1

	// Get skip ranges from settings if SettingService is available
	if h.SettingService != nil {
		// Get seed skip ranges
		seedSetting, _ := h.SettingService.GetSetting(ctx, "skip_thock_ranges_seed")
		if seedSetting != nil && seedSetting.SettingValue != "" {
			var seedRanges []SkipRangeEntry
			if json.Unmarshal([]byte(seedSetting.SettingValue), &seedRanges) == nil {
				nextSeed = calculateNextWithSkips(nextSeed, seedRanges)
			}
		}

		// Get sell skip ranges
		sellSetting, _ := h.SettingService.GetSetting(ctx, "skip_thock_ranges_sell")
		if sellSetting != nil && sellSetting.SettingValue != "" {
			var sellRanges []SkipRangeEntry
			if json.Unmarshal([]byte(sellSetting.SettingValue), &sellRanges) == nil {
				nextSell = calculateNextWithSkips(nextSell, sellRanges)
			}
		}
	}

	// Format the thock numbers
	nextSeedStr := padThockNumber(nextSeed, "seed")
	nextSellStr := strconv.Itoa(nextSell)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"next_seed":     nextSeedStr,
		"next_sell":     nextSellStr,
		"next_seed_num": nextSeed,
		"next_sell_num": nextSell,
		"seed_count":    seedCount,
		"sell_count":    sellCount,
	})
}

// calculateNextWithSkips finds the next valid number by skipping ranges
func calculateNextWithSkips(num int, ranges []SkipRangeEntry) int {
	// Keep incrementing if the number falls within any skip range
	for {
		inSkipRange := false
		for _, r := range ranges {
			if num >= r.From && num <= r.To {
				// Jump to after this range
				num = r.To + 1
				inSkipRange = true
				break
			}
		}
		if !inSkipRange {
			break
		}
	}
	return num
}

// padThockNumber formats the thock number based on category
func padThockNumber(num int, category string) string {
	if category == "seed" {
		return strconv.Itoa(num) // Will be padded to 4 digits when combined with quantity
	}
	return strconv.Itoa(num)
}

// SoftDeleteEntry soft-deletes an entry (admin only)
// DELETE /api/entries/{id}/soft-delete
func (h *EntryHandler) SoftDeleteEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Check admin role
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(r.Context())

	// Get entry before deletion for logging
	entry, err := h.Service.EntryRepo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	// Soft delete
	err = h.Service.EntryRepo.SoftDelete(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "Failed to delete entry: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the deletion to management log
	if h.ManagementLogRepo != nil {
		managementLog := &models.EntryManagementLog{
			ActionType:       "delete",
			PerformedByID:    userID,
			EntryID:          &id,
			ThockNumber:      &entry.ThockNumber,
			OldCustomerID:    &entry.CustomerID,
			OldCustomerName:  &entry.Name,
			OldCustomerPhone: &entry.Phone,
		}
		h.ManagementLogRepo.CreateReassignLog(context.Background(), managementLog)
	}

	// Log entry deletion to admin action log
	description := "Deleted entry " + entry.ThockNumber + " - " + entry.Name + " (" + entry.Village + ") - Qty: " + strconv.Itoa(entry.ExpectedQuantity)
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "DELETE",
		TargetType:  "entry",
		TargetID:    &id,
		Description: description,
	})

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())
	cache.InvalidateCustomerCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Entry deleted successfully",
	})
}

// RestoreEntry restores a soft-deleted entry (admin only)
// PUT /api/entries/{id}/restore
func (h *EntryHandler) RestoreEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Check admin role
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(r.Context())

	// Get entry before restoration for logging
	entry, err := h.Service.EntryRepo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	if entry.Status != "deleted" {
		http.Error(w, "Entry is not deleted", http.StatusBadRequest)
		return
	}

	// Restore
	err = h.Service.EntryRepo.RestoreDeleted(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to restore entry: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Log the restoration to management log
	if h.ManagementLogRepo != nil {
		managementLog := &models.EntryManagementLog{
			ActionType:       "restore",
			PerformedByID:    userID,
			EntryID:          &id,
			ThockNumber:      &entry.ThockNumber,
			NewCustomerID:    &entry.CustomerID,
			NewCustomerName:  &entry.Name,
			NewCustomerPhone: &entry.Phone,
		}
		h.ManagementLogRepo.CreateReassignLog(context.Background(), managementLog)
	}

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())
	cache.InvalidateCustomerCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Entry restored successfully",
	})
}

// GetDeletedEntries returns all soft-deleted entries (admin only)
// GET /api/entries/deleted
func (h *EntryHandler) GetDeletedEntries(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	entries, err := h.Service.EntryRepo.GetDeletedEntries(r.Context())
	if err != nil {
		http.Error(w, "Failed to get deleted entries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// UpdateFamilyMember updates the family member assignment for an entry
// PUT /api/entries/{id}/family-member
func (h *EntryHandler) UpdateFamilyMember(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// Check permission: admin OR can_manage_entries
	if !middleware.HasManageEntriesAccess(r.Context()) {
		http.Error(w, "Forbidden: Manage entries permission required", http.StatusForbidden)
		return
	}

	var req struct {
		FamilyMemberID   int    `json:"family_member_id"`
		FamilyMemberName string `json:"family_member_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FamilyMemberID <= 0 {
		http.Error(w, "family_member_id is required", http.StatusBadRequest)
		return
	}

	// Update family member in database
	err = h.Service.EntryRepo.UpdateFamilyMember(r.Context(), id, req.FamilyMemberID, req.FamilyMemberName)
	if err != nil {
		http.Error(w, "Failed to update family member: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Family member updated successfully",
	})
}

// BulkSoftDeleteEntries soft-deletes multiple entries at once (admin only)
// POST /api/entries/bulk-delete
func (h *EntryHandler) BulkSoftDeleteEntries(w http.ResponseWriter, r *http.Request) {
	// Check admin role
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok || role != "admin" {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(r.Context())

	var req struct {
		EntryIDs []int `json:"entry_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.EntryIDs) == 0 {
		http.Error(w, "entry_ids is required", http.StatusBadRequest)
		return
	}

	// Process each entry
	var successCount int
	var errors []string

	for _, entryID := range req.EntryIDs {
		// Get entry before deletion for logging
		entry, err := h.Service.EntryRepo.Get(r.Context(), entryID)
		if err != nil {
			errors = append(errors, "Entry "+strconv.Itoa(entryID)+": not found")
			continue
		}

		// Soft delete
		err = h.Service.EntryRepo.SoftDelete(r.Context(), entryID, userID)
		if err != nil {
			errors = append(errors, "Entry "+strconv.Itoa(entryID)+": "+err.Error())
			continue
		}

		// Log the deletion
		if h.ManagementLogRepo != nil {
			managementLog := &models.EntryManagementLog{
				ActionType:       "delete",
				PerformedByID:    userID,
				EntryID:          &entryID,
				ThockNumber:      &entry.ThockNumber,
				OldCustomerID:    &entry.CustomerID,
				OldCustomerName:  &entry.Name,
				OldCustomerPhone: &entry.Phone,
			}
			h.ManagementLogRepo.CreateReassignLog(context.Background(), managementLog)
		}

		successCount++
	}

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())
	cache.InvalidateCustomerCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       len(errors) == 0,
		"success_count": successCount,
		"total_count":   len(req.EntryIDs),
		"errors":        errors,
	})
}

// BulkReassignEntries reassigns multiple entries to a different customer at once
// POST /api/entries/bulk-reassign
func (h *EntryHandler) BulkReassignEntries(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		EntryIDs         []int  `json:"entry_ids"`
		NewCustomerID    int    `json:"new_customer_id"`
		FamilyMemberID   *int   `json:"family_member_id,omitempty"`
		FamilyMemberName string `json:"family_member_name,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.EntryIDs) == 0 {
		http.Error(w, "entry_ids is required", http.StatusBadRequest)
		return
	}

	if req.NewCustomerID <= 0 {
		http.Error(w, "new_customer_id is required", http.StatusBadRequest)
		return
	}

	// Process each entry
	var successCount int
	var errors []string

	for _, entryID := range req.EntryIDs {
		// Get old entry for logging
		oldEntry, err := h.Service.GetEntry(context.Background(), entryID)
		if err != nil {
			errors = append(errors, "Entry "+strconv.Itoa(entryID)+": not found")
			continue
		}

		// Skip if already assigned to this customer (unless changing family member)
		if oldEntry.CustomerID == req.NewCustomerID && req.FamilyMemberID == nil {
			errors = append(errors, "Entry "+strconv.Itoa(entryID)+": already assigned to this customer")
			continue
		}

		// Reassign the entry
		entry, newCustomer, err := h.Service.ReassignEntry(context.Background(), entryID, req.NewCustomerID, req.FamilyMemberID, req.FamilyMemberName)
		if err != nil {
			errors = append(errors, "Entry "+strconv.Itoa(entryID)+": "+err.Error())
			continue
		}

		// Log the reassignment
		if h.ManagementLogRepo != nil {
			oldCustomerID := oldEntry.CustomerID
			managementLog := &models.EntryManagementLog{
				PerformedByID:    userID,
				EntryID:          &entryID,
				ThockNumber:      &entry.ThockNumber,
				OldCustomerID:    &oldCustomerID,
				OldCustomerName:  &oldEntry.Name,
				OldCustomerPhone: &oldEntry.Phone,
				NewCustomerID:    &req.NewCustomerID,
				NewCustomerName:  &newCustomer.Name,
				NewCustomerPhone: &newCustomer.Phone,
			}
			h.ManagementLogRepo.CreateReassignLog(context.Background(), managementLog)
		}

		successCount++
	}

	// Invalidate caches
	cache.InvalidateEntryCaches(r.Context())
	cache.InvalidateCustomerCaches(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       len(errors) == 0,
		"success_count": successCount,
		"total_count":   len(req.EntryIDs),
		"errors":        errors,
	})
}
