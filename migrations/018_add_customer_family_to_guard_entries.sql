-- Add customer_id and family_member_id to guard_entries table
-- This allows linking guard entries to existing customers and family members

ALTER TABLE guard_entries
ADD COLUMN IF NOT EXISTS customer_id INTEGER REFERENCES customers(id),
ADD COLUMN IF NOT EXISTS family_member_id INTEGER REFERENCES family_members(id);

-- Add indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_guard_entries_customer_id ON guard_entries(customer_id);
CREATE INDEX IF NOT EXISTS idx_guard_entries_family_member_id ON guard_entries(family_member_id);
