package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"cold-backend/internal/auth"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/services"
)

type CustomerPortalHandler struct {
	OTPService            *services.OTPService
	CustomerPortalService *services.CustomerPortalService
	JWTManager            *auth.JWTManager
}

func NewCustomerPortalHandler(
	otpService *services.OTPService,
	customerPortalService *services.CustomerPortalService,
	jwtManager *auth.JWTManager,
) *CustomerPortalHandler {
	return &CustomerPortalHandler{
		OTPService:            otpService,
		CustomerPortalService: customerPortalService,
		JWTManager:            jwtManager,
	}
}

// SimpleLogin handles phone + truck number authentication (temporary until SMS OTP is ready)
func (h *CustomerPortalHandler) SimpleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone        string `json:"phone"`
		TruckNumber  string `json:"truck_number"`
		RememberMe   bool   `json:"remember_me"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Get customer by phone
	customer, err := h.CustomerPortalService.CustomerRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		http.Error(w, "Customer not found with this phone number", http.StatusUnauthorized)
		return
	}

	// Verify truck belongs to customer
	entry, err := h.CustomerPortalService.EntryRepo.GetByTruckNumber(ctx, req.TruckNumber)
	if err != nil {
		http.Error(w, "Truck number not found", http.StatusUnauthorized)
		return
	}

	if entry.CustomerID != customer.ID {
		http.Error(w, "Truck number does not belong to this customer", http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := h.JWTManager.GenerateCustomerToken(customer, req.RememberMe)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Get truck list
	trucks, err := h.CustomerPortalService.GetTrucksByCustomerID(ctx, customer.ID)
	if err != nil {
		trucks = []string{}
	}

	// Set cookie
	if req.RememberMe {
		http.SetCookie(w, &http.Cookie{
			Name:     "customer_session",
			Value:    token,
			Path:     "/",
			MaxAge:   30 * 24 * 60 * 60, // 30 days
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteStrictMode,
		})
	} else {
		http.SetCookie(w, &http.Cookie{
			Name:     "customer_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteStrictMode,
		})
	}

	// Return auth response
	response := models.CustomerAuthResponse{
		Success:  true,
		Token:    token,
		Customer: customer,
		Trucks:   trucks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SendOTP handles OTP generation and SMS sending
func (h *CustomerPortalHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req models.SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get IP address for rate limiting
	ipAddress := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ipAddress = forwarded
	}

	// Send OTP
	ctx := context.Background()
	err := h.OTPService.SendOTP(ctx, req.Phone, ipAddress)
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "OTP sent successfully to your phone",
	})
}

// VerifyOTP handles OTP verification and login
func (h *CustomerPortalHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req models.VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify OTP
	ctx := context.Background()
	customer, err := h.OTPService.VerifyOTP(ctx, req.Phone, req.OTP)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Generate JWT token
	token, err := h.JWTManager.GenerateCustomerToken(customer, req.RememberMe)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Get truck list
	trucks, err := h.CustomerPortalService.GetTrucksByCustomerID(ctx, customer.ID)
	if err != nil {
		trucks = []string{}
	}

	// Set cookie if remember me is enabled
	if req.RememberMe {
		http.SetCookie(w, &http.Cookie{
			Name:     "customer_session",
			Value:    token,
			Path:     "/",
			MaxAge:   30 * 24 * 60 * 60, // 30 days
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
			SameSite: http.SameSiteStrictMode,
		})
	} else {
		// Session cookie (expires when browser closes)
		http.SetCookie(w, &http.Cookie{
			Name:     "customer_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			SameSite: http.SameSiteStrictMode,
		})
	}

	// Return auth response
	response := models.CustomerAuthResponse{
		Success:  true,
		Token:    token,
		Customer: customer,
		Trucks:   trucks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ValidateSession validates customer session from cookie
func (h *CustomerPortalHandler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	// Get cookie
	cookie, err := r.Cookie("customer_session")
	if err != nil {
		http.Error(w, "No session found", http.StatusUnauthorized)
		return
	}

	// Validate JWT
	claims, err := h.JWTManager.ValidateCustomerToken(cookie.Value)
	if err != nil {
		// Cookie expired or invalid, clear it
		http.SetCookie(w, &http.Cookie{
			Name:   "customer_session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	// Get customer info
	ctx := context.Background()
	customer, err := h.CustomerPortalService.CustomerRepo.Get(ctx, claims.CustomerID)
	if err != nil {
		http.Error(w, "Customer not found", http.StatusUnauthorized)
		return
	}

	// Get trucks
	trucks, err := h.CustomerPortalService.GetTrucksByCustomerID(ctx, customer.ID)
	if err != nil {
		trucks = []string{}
	}

	// Return session data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":    true,
		"token":    cookie.Value,
		"customer": customer,
		"trucks":   trucks,
	})
}

// GetDashboard returns dashboard data for authenticated customer
func (h *CustomerPortalHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	// Get customer ID from context (set by middleware)
	customerID, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get dashboard data
	ctx := context.Background()
	dashboardData, err := h.CustomerPortalService.GetDashboardData(ctx, customerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboardData)
}

// CreateGatePassRequest handles customer gate pass request creation
func (h *CustomerPortalHandler) CreateGatePassRequest(w http.ResponseWriter, r *http.Request) {
	// Get customer ID from context (set by middleware)
	customerID, ok := middleware.GetCustomerIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.CreateCustomerGatePassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.TruckNumber == "" || req.RequestedQuantity <= 0 {
		http.Error(w, "Invalid truck number or quantity", http.StatusBadRequest)
		return
	}

	// Create gate pass request
	ctx := context.Background()
	gatePass, err := h.CustomerPortalService.CreateGatePassRequest(ctx, customerID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Gate pass request submitted successfully",
		"gate_pass": gatePass,
	})
}

// Logout clears the customer session
func (h *CustomerPortalHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "customer_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}
