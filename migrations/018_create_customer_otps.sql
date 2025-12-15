-- Migration: Create customer_otps table for SMS OTP verification
-- Created: 2025-12-15
-- Description: Stores OTP codes for customer portal login with expiration and rate limiting

CREATE TABLE IF NOT EXISTS customer_otps (
    id SERIAL PRIMARY KEY,
    phone VARCHAR(15) NOT NULL,
    otp_code VARCHAR(6) NOT NULL,
    ip_address VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    verified BOOLEAN DEFAULT FALSE,
    attempts INT DEFAULT 0,
    CONSTRAINT chk_otp_code_length CHECK (LENGTH(otp_code) = 6)
);

-- Indexes for performance and rate limiting
CREATE INDEX IF NOT EXISTS idx_customer_otps_phone_created ON customer_otps(phone, created_at);
CREATE INDEX IF NOT EXISTS idx_customer_otps_expires_at ON customer_otps(expires_at);
CREATE INDEX IF NOT EXISTS idx_customer_otps_ip_created ON customer_otps(ip_address, created_at);

-- Add comments for documentation
COMMENT ON TABLE customer_otps IS 'Stores OTP codes for customer login verification with rate limiting and expiration';
COMMENT ON COLUMN customer_otps.phone IS 'Customer phone number (10 digits)';
COMMENT ON COLUMN customer_otps.otp_code IS '6-digit OTP code sent via SMS';
COMMENT ON COLUMN customer_otps.ip_address IS 'IP address of the request for rate limiting';
COMMENT ON COLUMN customer_otps.expires_at IS 'OTP expiration time (typically 5 minutes from creation)';
COMMENT ON COLUMN customer_otps.verified IS 'TRUE if OTP has been successfully verified (prevents reuse)';
COMMENT ON COLUMN customer_otps.attempts IS 'Number of verification attempts (max 3 allowed)';
