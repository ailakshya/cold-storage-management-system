-- Migration 009: Add SMS logs table and notification settings
-- This table tracks all SMS messages sent (OTP, transaction, promotional)

CREATE TABLE IF NOT EXISTS sms_logs (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER DEFAULT 0,
    phone VARCHAR(15) NOT NULL,
    message_type VARCHAR(30) NOT NULL,
    message TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    reference_id VARCHAR(100),
    cost DECIMAL(10, 4) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_sms_logs_customer_id ON sms_logs(customer_id);
CREATE INDEX IF NOT EXISTS idx_sms_logs_phone ON sms_logs(phone);
CREATE INDEX IF NOT EXISTS idx_sms_logs_message_type ON sms_logs(message_type);
CREATE INDEX IF NOT EXISTS idx_sms_logs_status ON sms_logs(status);
CREATE INDEX IF NOT EXISTS idx_sms_logs_created_at ON sms_logs(created_at DESC);

-- Insert SMS notification toggle settings
INSERT INTO system_settings (setting_key, setting_value, description, updated_at)
VALUES
    ('sms_notify_item_in', 'false', 'Send SMS when items are stored (Item In)', CURRENT_TIMESTAMP),
    ('sms_notify_item_out', 'false', 'Send SMS when items are picked up (Item Out)', CURRENT_TIMESTAMP),
    ('sms_notify_payment_received', 'false', 'Send SMS when payment is received', CURRENT_TIMESTAMP),
    ('sms_notify_payment_reminder', 'false', 'Allow sending payment reminder SMS', CURRENT_TIMESTAMP),
    ('sms_allow_promotional', 'false', 'Allow sending promotional/bulk SMS', CURRENT_TIMESTAMP),
    ('sms_route', 'q', 'SMS route: q (quick/₹5), dlt (cheap/₹0.17), v3 (promo)', CURRENT_TIMESTAMP),
    ('sms_sender_id', '', 'Sender ID for DLT route (e.g., COLDST)', CURRENT_TIMESTAMP),
    ('sms_dlt_entity_id', '', 'DLT Entity ID (PEID)', CURRENT_TIMESTAMP),
    ('sms_cost_per_sms', '5.0', 'Cost per SMS for tracking', CURRENT_TIMESTAMP)
ON CONFLICT (setting_key) DO NOTHING;
