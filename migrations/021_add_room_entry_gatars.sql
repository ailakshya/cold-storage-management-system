-- Migration: 021_add_room_entry_gatars.sql
-- Purpose: Store per-gatar quantity for room entries

CREATE TABLE IF NOT EXISTS room_entry_gatars (
    id SERIAL PRIMARY KEY,
    room_entry_id INTEGER NOT NULL REFERENCES room_entries(id) ON DELETE CASCADE,
    gatar_no INTEGER NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 0,
    quality VARCHAR(10),          -- N, U, D, G
    remark VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient lookups by room entry
CREATE INDEX IF NOT EXISTS idx_room_entry_gatars_room_entry_id ON room_entry_gatars(room_entry_id);

-- Index for searching by gatar number
CREATE INDEX IF NOT EXISTS idx_room_entry_gatars_gatar_no ON room_entry_gatars(gatar_no);

-- Composite index for room visualization queries
CREATE INDEX IF NOT EXISTS idx_room_entry_gatars_composite ON room_entry_gatars(room_entry_id, gatar_no);

COMMENT ON TABLE room_entry_gatars IS 'Stores per-gatar quantity breakdown for room entries';
COMMENT ON COLUMN room_entry_gatars.gatar_no IS 'Gatar number within the room/floor';
COMMENT ON COLUMN room_entry_gatars.quantity IS 'Number of items stored in this gatar';
COMMENT ON COLUMN room_entry_gatars.quality IS 'Quality grade: N=Normal, U=Unka, D=Damaged, G=Good';
