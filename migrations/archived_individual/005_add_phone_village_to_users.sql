-- Add phone and village fields to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(15);
ALTER TABLE users ADD COLUMN IF NOT EXISTS village VARCHAR(100);

-- Create index on phone for faster searches
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

-- Update existing users with placeholder data
UPDATE users SET phone = '0000000000', village = 'Not specified' WHERE phone IS NULL;
