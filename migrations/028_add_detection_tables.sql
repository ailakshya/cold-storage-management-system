-- Migration: Add detection tables for YOLOv8 bag counting
-- Description: Creates tables to store detection sessions (per vehicle unloading),
--              junction table for many-to-many room entry (thock) linking
--              (1 vehicle → many thocks, or 1 large thock → multiple vehicles),
--              and video proof per session stored via file manager + NAS.
--
-- Linking: detection_sessions → guard_entries (vehicle registration at gate)
--          detection_room_entries → room_entries (thocks created from vehicle's bags)

-- detection_sessions: One row per vehicle unloading event at a gate
CREATE TABLE IF NOT EXISTS detection_sessions (
    id SERIAL PRIMARY KEY,
    gate_id VARCHAR(50) NOT NULL,
    guard_entry_id INTEGER REFERENCES guard_entries(id),
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration_seconds INTEGER,
    estimated_total INTEGER NOT NULL DEFAULT 0,
    unique_bag_count INTEGER NOT NULL DEFAULT 0,
    bag_cluster_count INTEGER NOT NULL DEFAULT 0,
    peak_bags_in_frame INTEGER NOT NULL DEFAULT 0,
    vehicle_confidence REAL,
    avg_bag_confidence REAL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'verified', 'discarded')),
    manual_count INTEGER,
    count_discrepancy INTEGER,
    video_path TEXT,
    video_size_bytes BIGINT,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- detection_room_entries: Many-to-many link between sessions and room entries (thocks)
-- 1 vehicle can carry bags for multiple thocks (multiple customers)
-- 1 large thock can arrive across multiple vehicles (multiple sessions)
CREATE TABLE IF NOT EXISTS detection_room_entries (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES detection_sessions(id) ON DELETE CASCADE,
    room_entry_id INTEGER NOT NULL REFERENCES room_entries(id) ON DELETE CASCADE,
    bag_count_for_entry INTEGER,
    linked_by_user_id INTEGER REFERENCES users(id),
    linked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(session_id, room_entry_id)
);

-- detection_events: Individual frame-level detections (optional, for debugging/audit)
CREATE TABLE IF NOT EXISTS detection_events (
    id BIGSERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES detection_sessions(id) ON DELETE CASCADE,
    frame_timestamp TIMESTAMP NOT NULL,
    bag_count INTEGER NOT NULL DEFAULT 0,
    cluster_count INTEGER NOT NULL DEFAULT 0,
    vehicle_detected BOOLEAN NOT NULL DEFAULT false,
    detections JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common queries
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_sessions_gate_id') THEN
        CREATE INDEX idx_detection_sessions_gate_id ON detection_sessions(gate_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_sessions_started_at') THEN
        CREATE INDEX idx_detection_sessions_started_at ON detection_sessions(started_at DESC);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_sessions_status') THEN
        CREATE INDEX idx_detection_sessions_status ON detection_sessions(status);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_sessions_guard_entry') THEN
        CREATE INDEX idx_detection_sessions_guard_entry ON detection_sessions(guard_entry_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_room_entries_session') THEN
        CREATE INDEX idx_detection_room_entries_session ON detection_room_entries(session_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_room_entries_re') THEN
        CREATE INDEX idx_detection_room_entries_re ON detection_room_entries(room_entry_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_events_session_id') THEN
        CREATE INDEX idx_detection_events_session_id ON detection_events(session_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_events_timestamp') THEN
        CREATE INDEX idx_detection_events_timestamp ON detection_events(frame_timestamp DESC);
    END IF;
END $$;
