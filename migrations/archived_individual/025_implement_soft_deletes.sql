-- Migration: Implement soft deletes to prevent permanent data loss
-- Adds deleted_at columns and changes cascade behaviors to RESTRICT

-- Add deleted_at columns to key tables
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
ALTER TABLE entries ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;
ALTER TABLE room_entries ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

-- Add indexes on deleted_at for efficient filtering
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_customers_deleted_at ON customers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_entries_deleted_at ON entries(deleted_at);
CREATE INDEX IF NOT EXISTS idx_room_entries_deleted_at ON room_entries(deleted_at);

-- Change cascade behaviors from CASCADE to RESTRICT to prevent accidental data loss
-- This ensures that child records must be handled before parent deletion

-- Drop existing foreign key constraints
ALTER TABLE entries DROP CONSTRAINT IF EXISTS entries_customer_id_fkey;
ALTER TABLE entry_events DROP CONSTRAINT IF EXISTS entry_events_entry_id_fkey;
ALTER TABLE rent_payments DROP CONSTRAINT IF EXISTS rent_payments_entry_id_fkey;
ALTER TABLE room_entries DROP CONSTRAINT IF EXISTS room_entries_entry_id_fkey;

-- Recreate with RESTRICT behavior (must handle children before parent deletion)
ALTER TABLE entries
    ADD CONSTRAINT entries_customer_id_fkey
    FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE RESTRICT;

ALTER TABLE entry_events
    ADD CONSTRAINT entry_events_entry_id_fkey
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE RESTRICT;

ALTER TABLE rent_payments
    ADD CONSTRAINT rent_payments_entry_id_fkey
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE RESTRICT;

ALTER TABLE room_entries
    ADD CONSTRAINT room_entries_entry_id_fkey
    FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE RESTRICT;

-- Note: Application code should now use soft deletes by setting deleted_at timestamp
-- Example: UPDATE users SET deleted_at = NOW() WHERE id = $1
-- Queries should filter: WHERE deleted_at IS NULL
