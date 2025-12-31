-- Add family member support to gate_passes table
-- This allows tracking which family member is taking items out

ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS family_member_id INT REFERENCES family_members(id) ON DELETE SET NULL;
ALTER TABLE gate_passes ADD COLUMN IF NOT EXISTS family_member_name VARCHAR(100);

-- Index for efficient queries by family member
CREATE INDEX IF NOT EXISTS idx_gate_passes_family_member ON gate_passes(family_member_id);
