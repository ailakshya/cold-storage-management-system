-- Migration: Create Guard Entries table for guard register feature
-- Guards log vehicle arrivals at the gate before formal processing by entry room

CREATE TABLE IF NOT EXISTS guard_entries (
    id SERIAL PRIMARY KEY,
    customer_name VARCHAR(100) NOT NULL,
    village VARCHAR(100) NOT NULL,
    mobile VARCHAR(15) NOT NULL,
    arrival_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    category VARCHAR(10) NOT NULL CHECK (category IN ('seed', 'sell', 'both')),
    remarks TEXT,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'processed')),
    created_by_user_id INTEGER NOT NULL REFERENCES users(id),
    processed_by_user_id INTEGER REFERENCES users(id),
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_guard_entries_status ON guard_entries(status);
CREATE INDEX IF NOT EXISTS idx_guard_entries_created_by ON guard_entries(created_by_user_id);
CREATE INDEX IF NOT EXISTS idx_guard_entries_arrival_time ON guard_entries(arrival_time);
CREATE INDEX IF NOT EXISTS idx_guard_entries_mobile ON guard_entries(mobile);
CREATE INDEX IF NOT EXISTS idx_guard_entries_date ON guard_entries(DATE(created_at));

-- Comments
COMMENT ON TABLE guard_entries IS 'Preliminary vehicle arrival records logged by security guards at the gate';
COMMENT ON COLUMN guard_entries.category IS 'Type of bags: seed, sell, or both (mixed)';
COMMENT ON COLUMN guard_entries.status IS 'pending = awaiting processing, processed = converted to entries';
