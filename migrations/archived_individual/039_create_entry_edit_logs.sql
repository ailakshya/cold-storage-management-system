-- Create entry_edit_logs table to track changes to entries
CREATE TABLE IF NOT EXISTS entry_edit_logs (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER NOT NULL REFERENCES entries(id),
    edited_by_user_id INTEGER NOT NULL REFERENCES users(id),
    old_name VARCHAR(255),
    new_name VARCHAR(255),
    old_phone VARCHAR(20),
    new_phone VARCHAR(20),
    old_village VARCHAR(255),
    new_village VARCHAR(255),
    old_so VARCHAR(255),
    new_so VARCHAR(255),
    old_expected_quantity INTEGER,
    new_expected_quantity INTEGER,
    old_thock_category VARCHAR(20),
    new_thock_category VARCHAR(20),
    old_remark TEXT,
    new_remark TEXT,
    edited_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_entry_id ON entry_edit_logs(entry_id);
CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_edited_by ON entry_edit_logs(edited_by_user_id);
CREATE INDEX IF NOT EXISTS idx_entry_edit_logs_edited_at ON entry_edit_logs(edited_at DESC);
