-- Migration: Add customer portal support to gate_passes table
-- Created: 2025-12-15
-- Description: Adds tracking fields to distinguish between employee-created and customer-created gate passes

-- Add customer portal tracking columns
ALTER TABLE gate_passes
ADD COLUMN IF NOT EXISTS created_by_customer_id INTEGER REFERENCES customers(id) ON DELETE SET NULL,
ADD COLUMN IF NOT EXISTS request_source VARCHAR(20) DEFAULT 'employee'
    CHECK (request_source IN ('employee', 'customer_portal'));

-- Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_gate_passes_created_by_customer
ON gate_passes(created_by_customer_id) WHERE created_by_customer_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_gate_passes_request_source
ON gate_passes(request_source);

-- Backfill existing records as employee-created
UPDATE gate_passes SET request_source = 'employee' WHERE request_source IS NULL;

-- Add comments for documentation
COMMENT ON COLUMN gate_passes.created_by_customer_id IS 'Customer ID if created via customer portal, NULL if created by employee';
COMMENT ON COLUMN gate_passes.request_source IS 'Source of request: employee (manual entry) or customer_portal (self-service)';
