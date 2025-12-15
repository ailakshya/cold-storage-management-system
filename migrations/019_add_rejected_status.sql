-- Add 'rejected' status to gate_passes
-- This allows employees to reject customer gate pass requests

-- Drop existing constraint if it exists
ALTER TABLE gate_passes DROP CONSTRAINT IF EXISTS gate_passes_status_check;

-- Add new constraint with 'rejected' status
ALTER TABLE gate_passes
ADD CONSTRAINT gate_passes_status_check
CHECK (status IN ('pending', 'approved', 'completed', 'expired', 'rejected', 'partially_completed'));

-- Add comment
COMMENT ON COLUMN gate_passes.status IS 'Status: pending, approved, completed, expired, rejected, partially_completed';
