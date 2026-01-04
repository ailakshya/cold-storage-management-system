package handlers

import (
	"encoding/json"
	"net/http"

	"cold-backend/internal/services"
)

type PrinterHandler struct {
	PrinterService *services.PrinterService
}

func NewPrinterHandler(ps *services.PrinterService) *PrinterHandler {
	return &PrinterHandler{PrinterService: ps}
}

type PrintRequest struct {
	Line1  string `json:"line1"`
	Line2  string `json:"line2"`
	Copies int    `json:"copies"`
}

type ReceiptPrintRequest struct {
	HTML string `json:"html"`
}

func (h *PrinterHandler) Print2Up(w http.ResponseWriter, r *http.Request) {
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Copies < 1 {
		req.Copies = 1
	}

	err := h.PrinterService.Print2Up(req.Line1, req.Line2, req.Copies)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Printed successfully",
	})
}

func (h *PrinterHandler) PrintReceipt(w http.ResponseWriter, r *http.Request) {
	var req ReceiptPrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.HTML == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "No HTML content provided",
		})
		return
	}

	err := h.PrinterService.PrintReceipt(req.HTML)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Receipt printed to HP printer",
	})
}
