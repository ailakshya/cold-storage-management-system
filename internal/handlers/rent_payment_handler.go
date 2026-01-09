package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

// Note: context package is still needed for background goroutine in NotifyPaymentReceived

type RentPaymentHandler struct {
	Service             *services.RentPaymentService
	LedgerService       *services.LedgerService
	NotificationService *services.NotificationService
	CustomerService     *services.CustomerService
	AdminActionRepo     *repositories.AdminActionLogRepository
	EntryRepo           *repositories.EntryRepository
}

func NewRentPaymentHandler(service *services.RentPaymentService, ledgerService *services.LedgerService, adminActionRepo *repositories.AdminActionLogRepository) *RentPaymentHandler {
	return &RentPaymentHandler{
		Service:         service,
		LedgerService:   ledgerService,
		AdminActionRepo: adminActionRepo,
	}
}

// SetNotificationService sets the notification service for payment SMS
func (h *RentPaymentHandler) SetNotificationService(notifService *services.NotificationService) {
	h.NotificationService = notifService
}

// SetCustomerService sets the customer service for S/O lookup
func (h *RentPaymentHandler) SetCustomerService(customerService *services.CustomerService) {
	h.CustomerService = customerService
}

// SetEntryRepo sets the entry repository for family member validation
func (h *RentPaymentHandler) SetEntryRepo(entryRepo *repositories.EntryRepository) {
	h.EntryRepo = entryRepo
}

func (h *RentPaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.CreateRentPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	// Additional validations
	if req.TotalRent < 0 {
		http.Error(w, "Total rent cannot be negative", http.StatusBadRequest)
		return
	}
	if req.AmountPaid < 0 {
		http.Error(w, "Amount paid cannot be negative", http.StatusBadRequest)
		return
	}
	if req.AmountPaid > req.TotalRent {
		http.Error(w, "Amount paid cannot exceed total rent", http.StatusBadRequest)
		return
	}

	// Normalize family member name to match entry's family member name
	// This prevents mismatches like "Aakash" vs "Aakesh"
	familyMemberName := strings.TrimSpace(req.FamilyMemberName)
	if h.EntryRepo != nil && req.EntryID > 0 {
		// Get the entry to find the correct family member name
		if entry, err := h.EntryRepo.Get(ctx, req.EntryID); err == nil && entry != nil {
			if entry.FamilyMemberName != "" {
				// Use the family member name from the entry (source of truth)
				familyMemberName = entry.FamilyMemberName
			}
		}
	}

	// If still empty, use customer name
	if familyMemberName == "" {
		familyMemberName = req.CustomerName
	}

	payment := &models.RentPayment{
		EntryID:           req.EntryID,
		FamilyMemberID:    req.FamilyMemberID,
		FamilyMemberName:  familyMemberName,
		CustomerName:      req.CustomerName,
		CustomerPhone:     req.CustomerPhone,
		TotalRent:         req.TotalRent,
		AmountPaid:        req.AmountPaid,
		Balance:           req.Balance, // Use client-provided cumulative balance
		ProcessedByUserID: userID,
		Notes:             req.Notes,
	}

	if err := h.Service.CreatePayment(ctx, payment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create ledger entry for payment
	if h.LedgerService != nil && payment.AmountPaid > 0 {
		// Lookup customer S/O for ledger entry
		customerSO := ""
		if h.CustomerService != nil {
			if customer, err := h.CustomerService.SearchByPhone(ctx, req.CustomerPhone); err == nil && customer != nil {
				customerSO = customer.SO
			}
		}

		// Determine ledger entry type based on payment method (case-insensitive)
		entryType := models.LedgerEntryTypePayment
		description := "Rent payment received (Cash)"
		if strings.EqualFold(req.PaymentType, "online") {
			entryType = models.LedgerEntryTypeOnlinePayment
			description = "Rent payment received (Online)"
		}

		ledgerEntry := &models.CreateLedgerEntryRequest{
			CustomerPhone:    req.CustomerPhone,
			CustomerName:     req.CustomerName,
			CustomerSO:       customerSO,
			EntryType:        entryType,
			Description:      description,
			Credit:           payment.AmountPaid,
			ReferenceID:      &payment.ID,
			ReferenceType:    "payment",
			FamilyMemberID:   req.FamilyMemberID,
			FamilyMemberName: familyMemberName, // Use corrected family member name
			CreatedByUserID:  userID,
			Notes:            req.Notes,
		}
		// Create ledger entry (don't fail the payment if this fails)
		_, _ = h.LedgerService.CreateEntry(ctx, ledgerEntry)
	}

	// Log payment creation
	logDescription := fmt.Sprintf("Payment received: ₹%.2f from %s (%s) - Balance: ₹%.2f",
		payment.AmountPaid, req.CustomerName, req.CustomerPhone, payment.Balance)
	if req.Notes != "" {
		logDescription += " | Notes: " + req.Notes
	}
	h.AdminActionRepo.CreateActionLog(ctx, &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "PAYMENT",
		TargetType:  "rent_payment",
		TargetID:    &payment.ID,
		Description: logDescription,
	})

	// Invalidate payment caches
	cache.InvalidatePaymentCaches(ctx)

	// Send payment SMS notification (non-blocking)
	if h.NotificationService != nil && payment.AmountPaid > 0 && req.CustomerPhone != "" {
		// Capture values for goroutine
		customerName := req.CustomerName
		customerPhone := req.CustomerPhone
		amountPaid := payment.AmountPaid
		balance := payment.Balance
		go func() {
			customer := &models.Customer{
				Name:  customerName,
				Phone: customerPhone,
			}
			// Remaining balance after this payment
			_ = h.NotificationService.NotifyPaymentReceived(context.Background(), customer, amountPaid, balance)
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func (h *RentPaymentHandler) GetPaymentsByEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	entryID, err := strconv.Atoi(vars["entry_id"])
	if err != nil {
		http.Error(w, "Invalid entry ID", http.StatusBadRequest)
		return
	}

	// IDOR protection - verify accountant access
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := r.Context().Value(middleware.HasAccountantAccessKey).(bool)

	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - accountant access required", http.StatusForbidden)
		return
	}

	payments, err := h.Service.GetPaymentsByEntryID(r.Context(), entryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func (h *RentPaymentHandler) GetPaymentsByPhone(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		http.Error(w, "Phone parameter required", http.StatusBadRequest)
		return
	}

	// CRITICAL FIX: IDOR protection - verify user has permission to view these payments
	role, ok := middleware.GetRoleFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized - role not found", http.StatusUnauthorized)
		return
	}

	hasAccountantAccess, _ := r.Context().Value(middleware.HasAccountantAccessKey).(bool)

	// Only admin, accountant, or employee with accountant access can view payments
	if role != "admin" && role != "accountant" && !hasAccountantAccess {
		http.Error(w, "Forbidden - accountant access required to view payments", http.StatusForbidden)
		return
	}

	payments, err := h.Service.GetPaymentsByPhone(r.Context(), phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func (h *RentPaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	payments, err := h.Service.ListPayments(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func (h *RentPaymentHandler) GetPaymentByReceiptNumber(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	receiptNumber := vars["receipt_number"]
	if receiptNumber == "" {
		http.Error(w, "Receipt number required", http.StatusBadRequest)
		return
	}

	payment, err := h.Service.GetPaymentByReceiptNumber(r.Context(), receiptNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}
