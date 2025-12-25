-- Migration: Add partial processing support for guard entries
-- Allow seed and sell portions to be processed separately

-- Add columns to track which portions have been processed
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS seed_processed BOOLEAN DEFAULT FALSE;
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS sell_processed BOOLEAN DEFAULT FALSE;

-- Add columns to track who processed each portion
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS seed_processed_by INTEGER REFERENCES users(id);
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS sell_processed_by INTEGER REFERENCES users(id);
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS seed_processed_at TIMESTAMP;
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS sell_processed_at TIMESTAMP;

-- Index for finding entries with unprocessed portions
CREATE INDEX IF NOT EXISTS idx_guard_entries_partial ON guard_entries(seed_processed, sell_processed) WHERE status = 'pending';

COMMENT ON COLUMN guard_entries.seed_processed IS 'Whether seed portion has been processed';
COMMENT ON COLUMN guard_entries.sell_processed IS 'Whether sell portion has been processed';
