-- Create season_requests table for dual admin approval workflow
CREATE TABLE IF NOT EXISTS season_requests (
    id SERIAL PRIMARY KEY,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, approved, rejected, completed, failed

    -- Initiator (Admin 1)
    initiated_by_user_id INTEGER NOT NULL REFERENCES users(id),
    initiated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Approver (Admin 2 - must be different)
    approved_by_user_id INTEGER REFERENCES users(id),
    approved_at TIMESTAMP,

    -- Results
    archive_location TEXT,
    records_archived JSONB,
    error_message TEXT,

    season_name VARCHAR(100),
    notes TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Ensure different admins initiate and approve
    CONSTRAINT chk_different_admins CHECK (approved_by_user_id IS NULL OR approved_by_user_id != initiated_by_user_id)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_season_requests_status ON season_requests(status);
CREATE INDEX IF NOT EXISTS idx_season_requests_initiated_by ON season_requests(initiated_by_user_id);
CREATE INDEX IF NOT EXISTS idx_season_requests_initiated_at ON season_requests(initiated_at DESC);
