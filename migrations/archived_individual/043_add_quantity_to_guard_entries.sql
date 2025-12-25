-- Migration: Add quantity field to guard_entries
-- Guards can now specify approximate bag count when registering vehicles

ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS quantity INTEGER DEFAULT 0;

COMMENT ON COLUMN guard_entries.quantity IS 'Approximate number of bags in the vehicle';
