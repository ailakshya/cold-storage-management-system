-- Add has_accountant_access field to users table
-- This allows employees to have accountant permissions

ALTER TABLE users ADD COLUMN IF NOT EXISTS has_accountant_access BOOLEAN DEFAULT FALSE;

-- Update existing users (set to false by default)
UPDATE users SET has_accountant_access = FALSE WHERE has_accountant_access IS NULL;

-- Admins automatically have accountant access
UPDATE users SET has_accountant_access = TRUE WHERE role = 'admin';
