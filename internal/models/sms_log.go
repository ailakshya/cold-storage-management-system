package models

import "time"

// SMSLog represents a sent SMS/WhatsApp message
type SMSLog struct {
	ID           int        `json:"id"`
	CustomerID   int        `json:"customer_id"`
	CustomerName string     `json:"customer_name,omitempty"`
	Phone        string     `json:"phone"`
	MessageType  string     `json:"message_type"`
	Message      string     `json:"message"`
	Channel      string     `json:"channel"` // "sms" or "whatsapp"
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	ReferenceID  string     `json:"reference_id,omitempty"`
	Cost         float64    `json:"cost,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
}

// SMS message types
const (
	SMSTypeOTP              = "otp"
	SMSTypeItemIn           = "item_in"
	SMSTypeItemOut          = "item_out"
	SMSTypePaymentReceived  = "payment_received"
	SMSTypePaymentReminder  = "payment_reminder"
	SMSTypePromotional      = "promotional"
	SMSTypeBulk             = "bulk"
	SMSTypeBoli             = "boli"          // Buyer arrival notification
	SMSTypeBoliRate         = "boli_rate"     // Rate update notification
	SMSTypeBoliComplete     = "boli_complete" // Sale complete notification
)

// SMS status types
const (
	SMSStatusPending   = "pending"
	SMSStatusSent      = "sent"
	SMSStatusDelivered = "delivered"
	SMSStatusFailed    = "failed"
)

// Message channels
const (
	ChannelSMS      = "sms"
	ChannelWhatsApp = "whatsapp"
)

// SMS setting keys for toggles
const (
	SettingSMSItemIn          = "sms_notify_item_in"
	SettingSMSItemOut         = "sms_notify_item_out"
	SettingSMSPaymentReceived = "sms_notify_payment_received"
	SettingSMSPaymentReminder = "sms_notify_payment_reminder"
	SettingSMSPromotional     = "sms_allow_promotional"
)

// WhatsApp setting keys
const (
	SettingWhatsAppEnabled    = "whatsapp_enabled"
	SettingWhatsAppProvider   = "whatsapp_provider"  // "aisensy", "interakt"
	SettingWhatsAppAPIKey     = "whatsapp_api_key"
	SettingWhatsAppCostPerMsg = "whatsapp_cost_per_msg"
)

// Customer portal setting keys
const (
	SettingCustomerLoginMethod = "customer_login_method" // "otp" or "thock"
)

// BulkSMSRequest represents a bulk SMS send request
type BulkSMSRequest struct {
	Message     string   `json:"message"`
	CustomerIDs []int    `json:"customer_ids,omitempty"`
	Filters     SMSFilter `json:"filters,omitempty"`
}

// SMSFilter represents filters for targeting customers
type SMSFilter struct {
	MinBalance      *float64 `json:"min_balance,omitempty"`
	MaxBalance      *float64 `json:"max_balance,omitempty"`
	MinItemsStored  *int     `json:"min_items_stored,omitempty"`
	MaxItemsStored  *int     `json:"max_items_stored,omitempty"`
	InactiveDays    *int     `json:"inactive_days,omitempty"`
	HasActiveEntry  *bool    `json:"has_active_entry,omitempty"`
}

// PaymentReminderRequest for sending payment reminders
type PaymentReminderRequest struct {
	MinBalance float64 `json:"min_balance"`
	Message    string  `json:"message,omitempty"`
}

// SMSStats represents SMS statistics
type SMSStats struct {
	TotalSent      int     `json:"total_sent"`
	TotalDelivered int     `json:"total_delivered"`
	TotalFailed    int     `json:"total_failed"`
	TodaySent      int     `json:"today_sent"`
	TodayCost      float64 `json:"today_cost"`
	MonthSent      int     `json:"month_sent"`
	MonthCost      float64 `json:"month_cost"`
}
