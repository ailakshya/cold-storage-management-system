-- Migration: Create token_colors table for date-based token colors
-- Allows admin to set token colors for specific dates (including future dates)
-- Prevents same color on consecutive days

CREATE TABLE IF NOT EXISTS token_colors (
    id SERIAL PRIMARY KEY,
    color_date DATE NOT NULL UNIQUE,
    color VARCHAR(20) NOT NULL,
    set_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for quick date lookups
CREATE INDEX IF NOT EXISTS idx_token_colors_date ON token_colors(color_date);

-- Insert today's default color if not exists
INSERT INTO token_colors (color_date, color)
VALUES (CURRENT_DATE, 'RED')
ON CONFLICT (color_date) DO NOTHING;

COMMENT ON TABLE token_colors IS 'Token colors by date - admin can set for today and future dates';
