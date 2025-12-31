-- Add skipped_tokens table for tracking lost/skipped physical tokens
CREATE TABLE IF NOT EXISTS skipped_tokens (
    id SERIAL PRIMARY KEY,
    token_number INTEGER NOT NULL,
    skip_date DATE NOT NULL DEFAULT CURRENT_DATE,
    reason VARCHAR(255),
    skipped_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(token_number, skip_date)
);

CREATE INDEX IF NOT EXISTS idx_skipped_tokens_date ON skipped_tokens(skip_date);
