-- Add soft delete columns to entries table
ALTER TABLE entries ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE entries ADD COLUMN IF NOT EXISTS deleted_by_user_id INTEGER REFERENCES users(id);

-- Index for filtering out deleted entries
CREATE INDEX IF NOT EXISTS idx_entries_deleted_at ON entries(deleted_at) WHERE deleted_at IS NULL;
