-- Add role field to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) DEFAULT 'employee';

-- Update admin user to have admin role
UPDATE users SET role = 'admin' WHERE email = 'admin@cold.com';

-- Create index on role for faster queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
