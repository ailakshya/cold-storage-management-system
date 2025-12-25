-- Add is_active column to users table
-- Default to true for all existing users
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE NOT NULL;

-- Update all existing users to be active
UPDATE users SET is_active = TRUE WHERE is_active IS NULL;
