-- Migration: Add driver_no to guard_entries and token_color to system_settings

-- Add driver_no column to guard_entries
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS driver_no VARCHAR(15);

-- Add index for driver_no
CREATE INDEX IF NOT EXISTS idx_guard_entries_driver_no ON guard_entries(driver_no);

-- Add token_color setting to system_settings
INSERT INTO system_settings (setting_key, setting_value, description)
VALUES ('token_color', 'RED', 'Token color of the day - set by admin')
ON CONFLICT (setting_key) DO NOTHING;

COMMENT ON COLUMN guard_entries.driver_no IS 'Driver mobile number (optional)';
