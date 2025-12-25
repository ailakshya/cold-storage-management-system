-- Reset All User Data for Testing
-- This script clears all data while keeping the schema intact

-- Disable foreign key checks temporarily
SET session_replication_role = 'replica';

-- Clear all data from tables (in order to respect dependencies)
TRUNCATE TABLE rent_payments CASCADE;
TRUNCATE TABLE entry_events CASCADE;
TRUNCATE TABLE room_entries CASCADE;
TRUNCATE TABLE entries CASCADE;
TRUNCATE TABLE customers CASCADE;
TRUNCATE TABLE users CASCADE;
TRUNCATE TABLE system_settings CASCADE;

-- Re-enable foreign key checks
SET session_replication_role = 'origin';

-- Reset sequences
ALTER SEQUENCE users_id_seq RESTART WITH 1;
ALTER SEQUENCE customers_id_seq RESTART WITH 1;
ALTER SEQUENCE entries_id_seq RESTART WITH 1;
ALTER SEQUENCE entry_events_id_seq RESTART WITH 1;
ALTER SEQUENCE room_entries_id_seq RESTART WITH 1;
ALTER SEQUENCE rent_payments_id_seq RESTART WITH 1;
ALTER SEQUENCE system_settings_id_seq RESTART WITH 1;

-- Create default admin user for testing
-- Email: admin@cold.com
-- Password: admin123
INSERT INTO users (email, password_hash, name, role, created_at, updated_at) VALUES
('admin@cold.com', '$2a$10$N9qo8uLOickgx2ZMRZoMye7U4hWJQbFlLwt7xW.hQOKvH8QhPVN8S', 'Administrator', 'admin', NOW(), NOW());

-- Create default system settings
INSERT INTO system_settings (setting_key, setting_value, description, created_at, updated_at) VALUES
('rent_per_item', '10.00', 'Rent price per item stored', NOW(), NOW()),
('company_name', 'Cold Storage Solutions', 'Company name for receipts', NOW(), NOW()),
('company_address', '123 Main Street, City, State', 'Company address for receipts', NOW(), NOW()),
('company_phone', '+91-1234567890', 'Company phone number', NOW(), NOW());

-- Verification
SELECT 'Users Count: ' || COUNT(*) FROM users;
SELECT 'Customers Count: ' || COUNT(*) FROM customers;
SELECT 'Entries Count: ' || COUNT(*) FROM entries;
SELECT 'Room Entries Count: ' || COUNT(*) FROM room_entries;
SELECT 'Payments Count: ' || COUNT(*) FROM rent_payments;
SELECT 'System Settings Count: ' || COUNT(*) FROM system_settings;
