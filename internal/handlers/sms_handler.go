package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/sms"
)

type SMSHandler struct {
	SMSLogRepo    *repositories.SMSLogRepository
	SettingRepo   *repositories.SystemSettingRepository
	SMSService    sms.SMSProvider
}

func NewSMSHandler(
	smsLogRepo *repositories.SMSLogRepository,
	settingRepo *repositories.SystemSettingRepository,
	smsService sms.SMSProvider,
) *SMSHandler {
	return &SMSHandler{
		SMSLogRepo:  smsLogRepo,
		SettingRepo: settingRepo,
		SMSService:  smsService,
	}
}

// ListLogs returns paginated SMS logs
func (h *SMSHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	messageType := r.URL.Query().Get("type")

	logs, total, err := h.SMSLogRepo.List(ctx, limit, offset, messageType)
	if err != nil {
		http.Error(w, "Failed to fetch SMS logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetStats returns SMS statistics
func (h *SMSHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.SMSLogRepo.GetStats(ctx)
	if err != nil {
		http.Error(w, "Failed to fetch stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetCustomersForBulkSMS returns customers based on filters
func (h *SMSHandler) GetCustomersForBulkSMS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var filters models.SMSFilter

	if minBalance := r.URL.Query().Get("min_balance"); minBalance != "" {
		if val, err := strconv.ParseFloat(minBalance, 64); err == nil {
			filters.MinBalance = &val
		}
	}

	if maxBalance := r.URL.Query().Get("max_balance"); maxBalance != "" {
		if val, err := strconv.ParseFloat(maxBalance, 64); err == nil {
			filters.MaxBalance = &val
		}
	}

	if minItems := r.URL.Query().Get("min_items"); minItems != "" {
		if val, err := strconv.Atoi(minItems); err == nil {
			filters.MinItemsStored = &val
		}
	}

	if inactiveDays := r.URL.Query().Get("inactive_days"); inactiveDays != "" {
		if val, err := strconv.Atoi(inactiveDays); err == nil {
			filters.InactiveDays = &val
		}
	}

	if hasActive := r.URL.Query().Get("has_active_entry"); hasActive == "true" {
		val := true
		filters.HasActiveEntry = &val
	}

	customers, err := h.SMSLogRepo.GetFilteredCustomers(ctx, filters)
	if err != nil {
		http.Error(w, "Failed to fetch customers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"customers": customers,
		"count":     len(customers),
	})
}

// SendBulkSMS sends SMS to multiple customers
func (h *SMSHandler) SendBulkSMS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if promotional SMS is allowed
	allowed, _ := h.SettingRepo.Get(ctx, models.SettingSMSPromotional)
	if allowed == nil || allowed.SettingValue != "true" {
		http.Error(w, "Promotional SMS is disabled in settings", http.StatusForbidden)
		return
	}

	var req models.BulkSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	var customers []map[string]interface{}
	var err error

	if len(req.CustomerIDs) > 0 {
		// Specific customers selected
		// TODO: Fetch customer details by IDs
		http.Error(w, "Customer ID selection not yet implemented", http.StatusNotImplemented)
		return
	} else {
		// Use filters
		customers, err = h.SMSLogRepo.GetFilteredCustomers(ctx, req.Filters)
		if err != nil {
			http.Error(w, "Failed to fetch customers: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if len(customers) == 0 {
		http.Error(w, "No customers match the criteria", http.StatusBadRequest)
		return
	}

	// Extract phones and customer IDs
	var phones []string
	var customerIDs []int
	for _, c := range customers {
		if phone, ok := c["phone"].(string); ok && phone != "" {
			phones = append(phones, phone)
			if id, ok := c["customer_id"].(int); ok {
				customerIDs = append(customerIDs, id)
			}
		}
	}

	// Send bulk SMS
	success, failed, err := h.SMSService.SendBulkSMS(phones, req.Message, customerIDs)
	if err != nil {
		http.Error(w, "Failed to send SMS: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"total":       len(phones),
		"sent":        success,
		"failed":      failed,
		"message":     fmt.Sprintf("Sent %d SMS, %d failed", success, failed),
	})
}

// SendPaymentReminders sends payment reminder to customers with balance
func (h *SMSHandler) SendPaymentReminders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if payment reminder is allowed
	allowed, _ := h.SettingRepo.Get(ctx, models.SettingSMSPaymentReminder)
	if allowed == nil || allowed.SettingValue != "true" {
		http.Error(w, "Payment reminder SMS is disabled in settings", http.StatusForbidden)
		return
	}

	var req models.PaymentReminderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MinBalance <= 0 {
		req.MinBalance = 1000 // Default minimum balance
	}

	customers, err := h.SMSLogRepo.GetCustomersWithBalance(ctx, req.MinBalance)
	if err != nil {
		http.Error(w, "Failed to fetch customers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(customers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"total":   0,
			"message": "No customers with balance above threshold",
		})
		return
	}

	success := 0
	failed := 0

	for _, c := range customers {
		phone := c["phone"].(string)
		name := c["name"].(string)
		balance := c["balance"].(float64)
		customerID := c["customer_id"].(int)

		// Personalized message
		message := req.Message
		if message == "" {
			message = fmt.Sprintf("Dear %s, your pending balance at Cold Storage is Rs.%.2f. Please clear the dues at your earliest. Thank you!", name, balance)
		}

		err := h.SMSService.SendSMS(phone, message, models.SMSTypePaymentReminder, customerID)
		if err != nil {
			failed++
		} else {
			success++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   len(customers),
		"sent":    success,
		"failed":  failed,
		"message": fmt.Sprintf("Sent %d reminders, %d failed", success, failed),
	})
}

// GetNotificationSettings returns SMS notification settings
func (h *SMSHandler) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	settings := make(map[string]interface{})

	keys := []string{
		models.SettingSMSItemIn,
		models.SettingSMSItemOut,
		models.SettingSMSPaymentReceived,
		models.SettingSMSPaymentReminder,
		models.SettingSMSPromotional,
		"sms_route",
		"sms_sender_id",
		"sms_dlt_entity_id",
		"sms_cost_per_sms",
		// WhatsApp settings
		models.SettingWhatsAppEnabled,
		models.SettingWhatsAppProvider,
		models.SettingWhatsAppAPIKey,
		models.SettingWhatsAppCostPerMsg,
	}

	for _, key := range keys {
		setting, err := h.SettingRepo.Get(ctx, key)
		if err == nil && setting != nil {
			settings[key] = setting.SettingValue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// UpdateNotificationSettings updates SMS notification settings
func (h *SMSHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for key, value := range req {
		err := h.SettingRepo.Upsert(ctx, key, value, "SMS notification setting", 0)
		if err != nil {
			http.Error(w, "Failed to update setting: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Settings updated successfully",
	})
}

// TestSMS sends a test SMS
func (h *SMSHandler) TestSMS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone   string `json:"phone"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Phone == "" {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		req.Message = "This is a test SMS from Cold Storage Management System."
	}

	err := h.SMSService.SendSMS(req.Phone, req.Message, "test", 0)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Test SMS sent successfully",
	})
}

// BoliNotificationRequest represents a boli notification request
type BoliNotificationRequest struct {
	ItemType  string  `json:"item_type"`
	Rate      float64 `json:"rate"`
	BuyerName string  `json:"buyer_name"`
	Language  string  `json:"language"` // "hindi" or "english"
}

// SendBoliNotification sends boli (buyer arrival) notification to customers with active entries
func (h *SMSHandler) SendBoliNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req BoliNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ItemType == "" {
		http.Error(w, "Item type is required", http.StatusBadRequest)
		return
	}

	if req.Rate <= 0 {
		http.Error(w, "Rate must be greater than 0", http.StatusBadRequest)
		return
	}

	if req.Language == "" {
		req.Language = "hindi" // Default to Hindi
	}

	// Get customers with active entries
	hasActiveEntry := true
	filters := models.SMSFilter{
		HasActiveEntry: &hasActiveEntry,
	}

	customers, err := h.SMSLogRepo.GetFilteredCustomers(ctx, filters)
	if err != nil {
		http.Error(w, "Failed to fetch customers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(customers) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"total":   0,
			"sent":    0,
			"failed":  0,
			"message": "No customers with active entries found",
		})
		return
	}

	success := 0
	failed := 0

	for _, c := range customers {
		phone, _ := c["phone"].(string)
		name, _ := c["name"].(string)
		customerID, _ := c["customer_id"].(int)

		if phone == "" {
			failed++
			continue
		}

		// Generate personalized message
		var message string
		if req.Language == "hindi" {
			message = fmt.Sprintf("%s जी, आज कोल्ड स्टोरेज में %s की बोली लगने वाली है।", name, req.ItemType)
			if req.BuyerName != "" {
				message += fmt.Sprintf(" खरीददार: %s।", req.BuyerName)
			}
			message += fmt.Sprintf(" अनुमानित भाव: Rs.%.0f/क्विंटल। कृपया अपना माल बेचने हेतु संपर्क करें। धन्यवाद!", req.Rate)
		} else {
			message = fmt.Sprintf("Dear %s, buyers available today at Cold Storage for %s.", name, req.ItemType)
			if req.BuyerName != "" {
				message += fmt.Sprintf(" Buyer: %s.", req.BuyerName)
			}
			message += fmt.Sprintf(" Expected Rate: Rs.%.0f/quintal. Contact us to sell at best rates. Thank you!", req.Rate)
		}

		err := h.SMSService.SendSMS(phone, message, models.SMSTypeBoli, customerID)
		if err != nil {
			failed++
		} else {
			success++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   len(customers),
		"sent":    success,
		"failed":  failed,
		"message": fmt.Sprintf("Sent %d बोली notifications, %d failed", success, failed),
	})
}
