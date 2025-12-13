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

type RentPaymentHandler struct {
	Service *services.RentPaymentService
}

func NewRentPaymentHandler(service *services.RentPaymentService) *RentPaymentHandler {
	return &RentPaymentHandler{Service: service}
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

	payment := &models.RentPayment{
		EntryID:           req.EntryID,
		CustomerName:      req.CustomerName,
		CustomerPhone:     req.CustomerPhone,
		TotalRent:         req.TotalRent,
		AmountPaid:        req.AmountPaid,
		Balance:           req.Balance,
		ProcessedByUserID: userID,
		Notes:             req.Notes,
	}

	if err := h.Service.CreatePayment(context.Background(), payment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
