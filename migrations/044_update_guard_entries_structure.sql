-- Migration: Update guard_entries structure
-- Add token number, S/O field, and separate seed/sell quantities

-- Add token_number column (daily auto-increment)
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS token_number INTEGER;

-- Add S/O (Son Of) field
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS so VARCHAR(100);

-- Add separate seed and sell quantity fields
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS seed_quantity INTEGER DEFAULT 0;
ALTER TABLE guard_entries ADD COLUMN IF NOT EXISTS sell_quantity INTEGER DEFAULT 0;

-- Create sequence for daily token numbers
CREATE SEQUENCE IF NOT EXISTS guard_entry_token_seq START 1;

-- Create index on token_number for quick lookups
CREATE INDEX IF NOT EXISTS idx_guard_entries_token ON guard_entries(token_number);

-- Comments
COMMENT ON COLUMN guard_entries.token_number IS 'Daily token number given to customer with colored token';
COMMENT ON COLUMN guard_entries.so IS 'Son Of / Father name (optional)';
COMMENT ON COLUMN guard_entries.seed_quantity IS 'Number of seed bags';
COMMENT ON COLUMN guard_entries.sell_quantity IS 'Number of sell bags';
