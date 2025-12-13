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

type InvoiceHandler struct {
	Service *services.InvoiceService
}

func NewInvoiceHandler(s *services.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{Service: s}
}

// CreateInvoice creates a new invoice
func (h *InvoiceHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req models.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	invoice, err := h.Service.CreateInvoice(context.Background(), &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// GetInvoice retrieves an invoice by ID
func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid invoice ID", http.StatusBadRequest)
		return
	}

	invoice, err := h.Service.GetInvoice(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// GetInvoiceByNumber retrieves an invoice by invoice number
func (h *InvoiceHandler) GetInvoiceByNumber(w http.ResponseWriter, r *http.Request) {
	invoiceNumber := mux.Vars(r)["number"]

	invoice, err := h.Service.GetInvoiceByNumber(context.Background(), invoiceNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// ListInvoices returns all invoices
func (h *InvoiceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	invoices, err := h.Service.ListInvoices(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}

// GetCustomerInvoices returns invoices for a specific customer
func (h *InvoiceHandler) GetCustomerInvoices(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["customer_id"]
	customerID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	invoices, err := h.Service.GetCustomerInvoices(context.Background(), customerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}
