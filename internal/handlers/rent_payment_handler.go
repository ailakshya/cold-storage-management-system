package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type RentPaymentHandler struct {
	Service             *services.RentPaymentService
	LedgerService       *services.LedgerService
	NotificationService *services.NotificationService
	CustomerService     *services.CustomerService
	AdminActionRepo     *repositories.AdminActionLogRepository
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

func (h *RentPaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRentPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
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

	payment := &models.RentPayment{
		EntryID:           req.EntryID,
		CustomerName:      req.CustomerName,
		CustomerPhone:     req.CustomerPhone,
		TotalRent:         req.TotalRent,
		AmountPaid:        req.AmountPaid,
		Balance:           req.Balance, // Use client-provided cumulative balance
		ProcessedByUserID: userID,
		Notes:             req.Notes,
	}

	if err := h.Service.CreatePayment(context.Background(), payment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create ledger entry for payment
	if h.LedgerService != nil && payment.AmountPaid > 0 {
		// Lookup customer S/O for ledger entry
		customerSO := ""
		if h.CustomerService != nil {
			if customer, err := h.CustomerService.SearchByPhone(r.Context(), req.CustomerPhone); err == nil && customer != nil {
				customerSO = customer.SO
			}
		}

		// Determine ledger entry type based on payment method
		entryType := models.LedgerEntryTypePayment
		description := "Rent payment received (Cash)"
		if req.PaymentType == "online" {
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
			FamilyMemberName: req.FamilyMemberName,
			CreatedByUserID:  userID,
			Notes:            req.Notes,
		}
		// Create ledger entry (don't fail the payment if this fails)
		_, _ = h.LedgerService.CreateEntry(r.Context(), ledgerEntry)
	}

	// Log payment creation
	description := fmt.Sprintf("Payment received: ₹%.2f from %s (%s) - Balance: ₹%.2f",
		payment.AmountPaid, req.CustomerName, req.CustomerPhone, payment.Balance)
	if req.Notes != "" {
		description += " | Notes: " + req.Notes
	}
	h.AdminActionRepo.CreateActionLog(context.Background(), &models.AdminActionLog{
		AdminUserID: userID,
		ActionType:  "PAYMENT",
		TargetType:  "rent_payment",
		TargetID:    &payment.ID,
		Description: description,
	})

	// Invalidate payment caches
	cache.InvalidatePaymentCaches(r.Context())

	// Send payment SMS notification (non-blocking)
	if h.NotificationService != nil && payment.AmountPaid > 0 && req.CustomerPhone != "" {
		go func() {
			customer := &models.Customer{
				Name:  req.CustomerName,
				Phone: req.CustomerPhone,
			}
			// Remaining balance after this payment
			_ = h.NotificationService.NotifyPaymentReceived(context.Background(), customer, payment.AmountPaid, payment.Balance)
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

	payments, err := h.Service.GetPaymentsByEntryID(context.Background(), entryID)
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

	payments, err := h.Service.GetPaymentsByPhone(context.Background(), phone)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func (h *RentPaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	payments, err := h.Service.ListPayments(context.Background())
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

	payment, err := h.Service.GetPaymentByReceiptNumber(context.Background(), receiptNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}
