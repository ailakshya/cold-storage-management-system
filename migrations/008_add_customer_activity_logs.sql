-- Migration 008: Add customer activity logs table
-- This table stores customer portal activity for auditing and monitoring

CREATE TABLE IF NOT EXISTS customer_activity_logs (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER DEFAULT 0,
    phone VARCHAR(15) NOT NULL,
    action VARCHAR(50) NOT NULL,
    details TEXT,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_customer_activity_logs_customer_id ON customer_activity_logs(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_activity_logs_phone ON customer_activity_logs(phone);
CREATE INDEX IF NOT EXISTS idx_customer_activity_logs_action ON customer_activity_logs(action);
CREATE INDEX IF NOT EXISTS idx_customer_activity_logs_created_at ON customer_activity_logs(created_at DESC);

-- Insert default SMS rate limiter settings if they don't exist
INSERT INTO system_settings (setting_key, setting_value, description, updated_at)
VALUES
    ('sms_otp_cooldown_minutes', '2', 'Cooldown period between OTP requests (minutes)', CURRENT_TIMESTAMP),
    ('sms_max_otp_per_hour', '3', 'Maximum OTP requests per phone number per hour', CURRENT_TIMESTAMP),
    ('sms_max_otp_per_day', '10', 'Maximum OTP requests per phone number per day', CURRENT_TIMESTAMP),
    ('sms_max_otp_per_ip_hour', '10', 'Maximum OTP requests per IP address per hour', CURRENT_TIMESTAMP),
    ('sms_max_otp_per_ip_day', '50', 'Maximum OTP requests per IP address per day', CURRENT_TIMESTAMP),
    ('sms_max_daily_total', '1000', 'Maximum total SMS messages per day (budget limit)', CURRENT_TIMESTAMP)
ON CONFLICT (setting_key) DO NOTHING;
