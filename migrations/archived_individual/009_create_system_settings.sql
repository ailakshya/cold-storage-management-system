-- Create system_settings table
CREATE TABLE IF NOT EXISTS system_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(100) UNIQUE NOT NULL,
    setting_value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by_user_id INTEGER REFERENCES users(id)
);

-- Insert default rent per item setting
INSERT INTO system_settings (setting_key, setting_value, description)
VALUES ('rent_per_item', '10', 'Rent price per item in rupees')
ON CONFLICT (setting_key) DO NOTHING;

-- Insert loading/unloading mode setting
INSERT INTO system_settings (setting_key, setting_value, description)
VALUES ('operation_mode', 'loading', 'System operation mode: loading (items in) or unloading (items out)')
ON CONFLICT (setting_key) DO NOTHING;

-- Create index on setting_key for fast lookups
CREATE INDEX IF NOT EXISTS idx_system_settings_key ON system_settings(setting_key);
