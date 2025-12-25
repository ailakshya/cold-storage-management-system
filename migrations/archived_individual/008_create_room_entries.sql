-- Create room_entries table
CREATE TABLE IF NOT EXISTS room_entries (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    truck_number VARCHAR(50) NOT NULL,
    room_no VARCHAR(10) NOT NULL,
    floor VARCHAR(10) NOT NULL,
    gate_no VARCHAR(50) NOT NULL,
    remark VARCHAR(100),
    quantity INTEGER NOT NULL,
    created_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_room_entries_entry_id ON room_entries(entry_id);
CREATE INDEX idx_room_entries_truck_number ON room_entries(truck_number);
CREATE INDEX idx_room_entries_created_at ON room_entries(created_at);
