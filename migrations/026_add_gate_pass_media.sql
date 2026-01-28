-- Migration: Add gate pass media tracking
-- Description: Creates table to track photos/videos attached to gate passes (both entry and pickup)

CREATE TABLE IF NOT EXISTS gate_pass_media (
    id SERIAL PRIMARY KEY,
    gate_pass_id INTEGER REFERENCES gate_passes(id) ON DELETE CASCADE,
    gate_pass_pickup_id INTEGER REFERENCES gate_pass_pickups(id) ON DELETE CASCADE,
    thock_number VARCHAR(100) NOT NULL,
    media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('entry', 'pickup')),
    file_path TEXT NOT NULL,
    file_name TEXT NOT NULL,
    file_type VARCHAR(20) NOT NULL,  -- 'image', 'video'
    file_size BIGINT,
    uploaded_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Constraint: entry media should not have pickup_id, pickup media must have pickup_id
    CONSTRAINT gate_pass_media_type_check CHECK (
        (media_type = 'entry' AND gate_pass_pickup_id IS NULL) OR
        (media_type = 'pickup' AND gate_pass_pickup_id IS NOT NULL)
    )
);

-- Indexes for fast queries
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_gate_pass_media_thock') THEN
        CREATE INDEX idx_gate_pass_media_thock ON gate_pass_media(thock_number);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_gate_pass_media_gate_pass') THEN
        CREATE INDEX idx_gate_pass_media_gate_pass ON gate_pass_media(gate_pass_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_gate_pass_media_pickup') THEN
        CREATE INDEX idx_gate_pass_media_pickup ON gate_pass_media(gate_pass_pickup_id);
    END IF;
END $$;

-- Comments
COMMENT ON TABLE gate_pass_media IS 'Tracks photos and videos attached to gate passes during entry and pickup operations';
COMMENT ON COLUMN gate_pass_media.media_type IS 'Type of media: entry (captured during room entry) or pickup (captured during gate pass pickup)';
COMMENT ON COLUMN gate_pass_media.file_path IS 'Relative path to file from Room Config base directory';
COMMENT ON COLUMN gate_pass_media.file_name IS 'Structured filename with thock, customer, and employee info';
