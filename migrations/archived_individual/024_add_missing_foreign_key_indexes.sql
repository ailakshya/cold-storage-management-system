-- Migration: Add missing indexes on foreign keys and common query patterns
-- This significantly improves query performance on JOIN operations and WHERE clauses

-- Foreign key indexes (critical for JOIN performance)
CREATE INDEX IF NOT EXISTS idx_entry_events_created_by
    ON entry_events(created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_room_entries_created_by
    ON room_entries(created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_gate_passes_issued_by
    ON gate_passes(issued_by_user_id);

CREATE INDEX IF NOT EXISTS idx_gate_passes_approved_by
    ON gate_passes(approved_by_user_id);

CREATE INDEX IF NOT EXISTS idx_rent_payments_customer_phone
    ON rent_payments(customer_phone);

-- Composite indexes for common query patterns (significant performance boost)

-- entry_events: Frequently queried by entry_id with status filter
CREATE INDEX IF NOT EXISTS idx_entry_events_entry_status
    ON entry_events(entry_id, status);

-- room_entries: Lookup by truck number, room, and floor combination
CREATE INDEX IF NOT EXISTS idx_room_entries_truck_room
    ON room_entries(truck_number, room_no, floor);

-- rent_payments: Entry payments sorted by date (descending for recent first)
CREATE INDEX IF NOT EXISTS idx_rent_payments_entry_date
    ON rent_payments(entry_id, payment_date DESC);

-- gate_passes: Customer's gate passes filtered by status
CREATE INDEX IF NOT EXISTS idx_gate_passes_customer_status
    ON gate_passes(customer_id, status);

-- gate_passes: Entry-based lookups for stock validation
CREATE INDEX IF NOT EXISTS idx_gate_passes_entry_status
    ON gate_passes(entry_id, status);

-- invoices: Customer invoices sorted by creation date
CREATE INDEX IF NOT EXISTS idx_invoices_customer_created
    ON invoices(customer_id, created_at DESC);

-- room_entries: Entry ID lookups (for validation)
CREATE INDEX IF NOT EXISTS idx_room_entries_entry_id
    ON room_entries(entry_id);
