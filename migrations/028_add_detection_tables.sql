-- Migration: Add detection tables for YOLOv8 bag counting
-- Description: Creates tables to store detection sessions (per vehicle unloading)
--              and individual detection events from the Python inference service.

-- detection_sessions: One row per vehicle unloading event at a gate
CREATE TABLE IF NOT EXISTS detection_sessions (
    id SERIAL PRIMARY KEY,
    gate_id VARCHAR(50) NOT NULL,
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    duration_seconds INTEGER,
    estimated_total INTEGER NOT NULL DEFAULT 0,
    unique_bag_count INTEGER NOT NULL DEFAULT 0,
    bag_cluster_count INTEGER NOT NULL DEFAULT 0,
    peak_bags_in_frame INTEGER NOT NULL DEFAULT 0,
    vehicle_confidence REAL,
    avg_bag_confidence REAL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'discarded')),
    matched_gate_pass_id INTEGER REFERENCES gate_passes(id),
    manual_count INTEGER,
    count_discrepancy INTEGER,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_sessions_gate_pass') THEN
        CREATE INDEX idx_detection_sessions_gate_pass ON detection_sessions(matched_gate_pass_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_events_session_id') THEN
        CREATE INDEX idx_detection_events_session_id ON detection_events(session_id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_detection_events_timestamp') THEN
        CREATE INDEX idx_detection_events_timestamp ON detection_events(frame_timestamp DESC);
    END IF;
END $$;
