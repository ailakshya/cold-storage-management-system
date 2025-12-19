-- Migration: Add gatar-level breakdown to pickups
-- This allows tracking exactly which gatars items were picked from

CREATE TABLE IF NOT EXISTS gate_pass_pickup_gatars (
    id SERIAL PRIMARY KEY,
    pickup_id INTEGER NOT NULL REFERENCES gate_pass_pickups(id) ON DELETE CASCADE,
    gatar_no INTEGER NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient lookup by pickup_id
CREATE INDEX IF NOT EXISTS idx_pickup_gatars_pickup_id ON gate_pass_pickup_gatars(pickup_id);

-- Comment for documentation
COMMENT ON TABLE gate_pass_pickup_gatars IS 'Stores per-gatar quantity breakdown for each pickup';
