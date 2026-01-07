-- Migration: Add family_member_id and family_member_name columns to rent_payments table
-- This allows tracking which family member a payment is associated with

ALTER TABLE rent_payments ADD COLUMN IF NOT EXISTS family_member_id INT REFERENCES family_members(id) ON DELETE SET NULL;
ALTER TABLE rent_payments ADD COLUMN IF NOT EXISTS family_member_name VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_rent_payments_family_member_id ON rent_payments(family_member_id);
