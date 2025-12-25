-- Room Entry Edit Logs Table
-- Tracks all changes made to room entries for audit purposes
CREATE TABLE IF NOT EXISTS room_entry_edit_logs (
    id SERIAL PRIMARY KEY,
    room_entry_id INTEGER REFERENCES room_entries(id) ON DELETE CASCADE,
    edited_by_user_id INTEGER REFERENCES users(id),
    old_room_no VARCHAR(50),
    new_room_no VARCHAR(50),
    old_floor VARCHAR(50),
    new_floor VARCHAR(50),
    old_gate_no VARCHAR(50),
    new_gate_no VARCHAR(50),
    old_quantity INTEGER,
    new_quantity INTEGER,
    old_remark TEXT,
    new_remark TEXT,
    edited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_edit_logs_room_entry_id ON room_entry_edit_logs(room_entry_id);
CREATE INDEX IF NOT EXISTS idx_edit_logs_edited_by ON room_entry_edit_logs(edited_by_user_id);
CREATE INDEX IF NOT EXISTS idx_edit_logs_edited_at ON room_entry_edit_logs(edited_at DESC);
