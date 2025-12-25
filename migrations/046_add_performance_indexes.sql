-- Migration: 046_add_performance_indexes.sql
-- Purpose: Add database indexes to improve query performance
-- These indexes support the Redis caching layer by reducing DB latency

-- ============================================
-- HIGH IMPACT INDEXES
-- ============================================

-- Customer search optimization
-- Used for phone search in entries/payments
CREATE INDEX IF NOT EXISTS idx_customers_phone_prefix ON customers(phone varchar_pattern_ops);

-- Guard entries (high traffic page)
-- Used for filtering pending entries by status and date
CREATE INDEX IF NOT EXISTS idx_guard_entries_status_date ON guard_entries(status, created_at DESC);

-- Entries by date range
-- Used for reports and customer entry listings
CREATE INDEX IF NOT EXISTS idx_entries_created_at_customer ON entries(created_at DESC, customer_id);

-- Payments by customer phone
-- Used for accountant page and customer portal
CREATE INDEX IF NOT EXISTS idx_rent_payments_phone_date ON rent_payments(customer_phone, payment_date DESC);

-- ============================================
-- MEDIUM IMPACT INDEXES
-- ============================================

-- Gate pass status filtering
-- Used for pending/approved gate pass lists
CREATE INDEX IF NOT EXISTS idx_gate_passes_status_updated ON gate_passes(status, updated_at DESC);

-- Room entries by location
-- Used for room visualization page
CREATE INDEX IF NOT EXISTS idx_room_entries_room_floor ON room_entries(room_no, floor);

-- Entries by thock number (frequent lookups)
CREATE INDEX IF NOT EXISTS idx_entries_thock_number ON entries(thock_number);

-- Gate passes by thock number
CREATE INDEX IF NOT EXISTS idx_gate_passes_thock_number ON gate_passes(thock_number);

-- ============================================
-- COMPOSITE INDEXES FOR COMMON QUERIES
-- ============================================

-- Entries: category + created_at for sequence generation
CREATE INDEX IF NOT EXISTS idx_entries_category_created ON entries(thock_category, created_at DESC);

-- Guard entries: user + date for "my entries" query
CREATE INDEX IF NOT EXISTS idx_guard_entries_user_created ON guard_entries(created_by_user_id, created_at DESC);

-- Room entries: entry_id for join optimization
CREATE INDEX IF NOT EXISTS idx_room_entries_entry_id ON room_entries(entry_id);

-- Gate pass pickups: gate_pass_id for history queries
CREATE INDEX IF NOT EXISTS idx_gate_pass_pickups_gate_pass ON gate_pass_pickups(gate_pass_id, pickup_time DESC);
