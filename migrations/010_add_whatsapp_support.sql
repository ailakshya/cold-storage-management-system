-- Add WhatsApp support to messaging system

-- Add channel column to sms_logs (defaults to 'sms' for existing records)
ALTER TABLE sms_logs ADD COLUMN IF NOT EXISTS channel VARCHAR(20) DEFAULT 'sms';

-- Create index on channel for filtering
CREATE INDEX IF NOT EXISTS idx_sms_logs_channel ON sms_logs(channel);

-- Insert WhatsApp settings
INSERT INTO system_settings (setting_key, setting_value, description, updated_by_user_id)
VALUES
    ('whatsapp_enabled', 'false', 'Enable WhatsApp messaging (with SMS fallback)', NULL),
    ('whatsapp_provider', 'aisensy', 'WhatsApp API provider (aisensy/interakt)', NULL),
    ('whatsapp_api_key', '', 'WhatsApp API key from provider', NULL),
    ('whatsapp_cost_per_msg', '0.08', 'Cost per WhatsApp message in INR', NULL)
ON CONFLICT (setting_key) DO NOTHING;
