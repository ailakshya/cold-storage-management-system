package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"cold-backend/internal/auth"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"
)

type TOTPHandler struct {
	TOTPService *services.TOTPService
	UserRepo    *repositories.UserRepository
	JWTManager  *auth.JWTManager
}

func NewTOTPHandler(totpService *services.TOTPService, userRepo *repositories.UserRepository, jwtManager *auth.JWTManager) *TOTPHandler {
	return &TOTPHandler{
		TOTPService: totpService,
		UserRepo:    userRepo,
		JWTManager:  jwtManager,
	}
}

// SetupTOTP initiates 2FA setup - returns secret and QR code
func (h *TOTPHandler) SetupTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	user, err := h.UserRepo.Get(context.Background(), userID)
	if err != nil {
		log.Printf("[2FA] Failed to get user %d: %v", userID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not found: " + err.Error()})
		return
	}

	// Check if already enabled
	if user.TOTPEnabled {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "2FA is already enabled"})
		return
	}

	response, err := h.TOTPService.GenerateSetup(context.Background(), user)
	if err != nil {
		log.Printf("[2FA] Failed to generate setup for user %d: %v", user.ID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate 2FA setup: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// EnableTOTP verifies the code and enables 2FA - returns backup codes
func (h *TOTPHandler) EnableTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.TOTPEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, "Verification code is required", http.StatusBadRequest)
		return
	}

	ipAddress := getIPAddress(r)
	response, err := h.TOTPService.VerifyAndEnable(context.Background(), userID, req.Code, ipAddress)
	if err != nil {
		if _, ok := err.(*services.TOTPError); ok {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to enable 2FA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DisableTOTP turns off 2FA after verifying password and code
func (h *TOTPHandler) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.TOTPDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password == "" || req.Code == "" {
		http.Error(w, "Password and verification code are required", http.StatusBadRequest)
		return
	}

	err := h.TOTPService.Disable(context.Background(), userID, req.Password, req.Code)
	if err != nil {
		if _, ok := err.(*services.TOTPError); ok {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to disable 2FA", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "2FA disabled successfully"})
}

// GetStatus returns the 2FA status for the current user
func (h *TOTPHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	status, err := h.TOTPService.GetStatus(context.Background(), userID)
	if err != nil {
		log.Printf("[2FA] Failed to get status for user %d: %v", userID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get 2FA status: " + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// RegenerateBackupCodes creates new backup codes (requires password)
func (h *TOTPHandler) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.RegenerateBackupCodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}

	response, err := h.TOTPService.RegenerateBackupCodes(context.Background(), userID, req.Password)
	if err != nil {
		if _, ok := err.(*services.TOTPError); ok {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to regenerate backup codes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// VerifyTOTP handles 2FA verification during login (step 2)
func (h *TOTPHandler) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
	var req models.TOTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TempToken == "" || req.Code == "" {
		http.Error(w, "Temp token and verification code are required", http.StatusBadRequest)
		return
	}

	// Validate temp token
	tempClaims, err := h.JWTManager.ValidateTempToken(req.TempToken)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Get user
	user, err := h.UserRepo.Get(context.Background(), tempClaims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Verify TOTP code
	ipAddress := getIPAddress(r)
	valid, err := h.TOTPService.Verify(context.Background(), user.ID, req.Code, ipAddress)
	if err != nil {
		if _, ok := err.(*services.TOTPError); ok {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Verification failed", http.StatusInternalServerError)
		return
	}

	if !valid {
		http.Error(w, "Invalid verification code", http.StatusUnauthorized)
		return
	}

	// Generate full JWT token
	token, err := h.JWTManager.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	response := &models.AuthResponse{
		Token: token,
		User:  user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
