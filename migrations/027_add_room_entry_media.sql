-- Migration: Add room entry media tracking
-- Description: Creates table to track photos/videos attached to room entries

CREATE TABLE IF NOT EXISTS room_entry_media (
    id SERIAL PRIMARY KEY,
    room_entry_id INTEGER NOT NULL REFERENCES room_entries(id) ON DELETE CASCADE,
    thock_number VARCHAR(100) NOT NULL,
    media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('entry', 'edit')),
    file_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_size BIGINT,
    uploaded_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_room_entry_media_room_entry') THEN
        CREATE INDEX idx_room_entry_media_room_entry ON room_entry_media(room_entry_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_room_entry_media_thock') THEN
        CREATE INDEX idx_room_entry_media_thock ON room_entry_media(thock_number);
    END IF;
END $$;
