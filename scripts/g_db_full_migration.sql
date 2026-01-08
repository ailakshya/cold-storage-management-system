-- G Database Full Migration
-- This creates all tables needed to mirror the main cold_db schema
-- Modified to work without users table (PIN-based auth)

-- ============================================
-- Customers Table
-- ============================================
CREATE TABLE IF NOT EXISTS customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(15) UNIQUE NOT NULL,
    so VARCHAR(100),
    village VARCHAR(100),
    address TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_customers_phone ON customers(phone);
CREATE INDEX IF NOT EXISTS idx_g_customers_name ON customers(name);

-- ============================================
-- Entries Table (Thock/Truck entries)
-- ============================================
CREATE TABLE IF NOT EXISTS entries (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id) ON DELETE CASCADE,
    phone VARCHAR(15) NOT NULL,
    name VARCHAR(100) NOT NULL,
    village VARCHAR(100),
    so VARCHAR(100),
    expected_quantity INTEGER NOT NULL,
    thock_category VARCHAR(20) NOT NULL CHECK (thock_category IN ('seed', 'sell')),
    thock_number VARCHAR(50),
    created_by_user_id INTEGER, -- No FK since no users table
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_entries_customer_id ON entries(customer_id);
CREATE INDEX IF NOT EXISTS idx_g_entries_phone ON entries(phone);
CREATE INDEX IF NOT EXISTS idx_g_entries_created_at ON entries(created_at);
CREATE INDEX IF NOT EXISTS idx_g_entries_thock_number ON entries(thock_number);

-- ============================================
-- Room Entries Table (Location assignments)
-- ============================================
CREATE TABLE IF NOT EXISTS room_entries (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    thock_number VARCHAR(50) NOT NULL,
    room_no VARCHAR(10) NOT NULL,
    floor VARCHAR(10) NOT NULL,
    gate_no VARCHAR(50) NOT NULL,
    remark VARCHAR(100),
    quantity INTEGER NOT NULL,
    created_by_user_id INTEGER, -- No FK since no users table
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_room_entries_entry_id ON room_entries(entry_id);
CREATE INDEX IF NOT EXISTS idx_g_room_entries_thock_number ON room_entries(thock_number);
CREATE INDEX IF NOT EXISTS idx_g_room_entries_created_at ON room_entries(created_at);

-- ============================================
-- Gate Passes Table
-- ============================================
CREATE TABLE IF NOT EXISTS gate_passes (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    thock_number VARCHAR(50) NOT NULL,
    entry_id INTEGER REFERENCES entries(id) ON DELETE SET NULL,
    requested_quantity INTEGER NOT NULL,
    approved_quantity VARCHAR(20),
    final_approved_quantity INTEGER,
    gate_no VARCHAR(50),
    status VARCHAR(20) DEFAULT 'pending',
    payment_verified BOOLEAN DEFAULT false,
    payment_amount DECIMAL(10,2),
    total_picked_up INTEGER DEFAULT 0,
    issued_by_user_id INTEGER,
    approved_by_user_id INTEGER,
    created_by_customer_id INTEGER,
    request_source VARCHAR(20) DEFAULT 'employee',
    issued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    approval_expires_at TIMESTAMP,
    completed_at TIMESTAMP,
    remarks TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_gate_passes_customer_id ON gate_passes(customer_id);
CREATE INDEX IF NOT EXISTS idx_g_gate_passes_thock_number ON gate_passes(thock_number);
CREATE INDEX IF NOT EXISTS idx_g_gate_passes_status ON gate_passes(status);
CREATE INDEX IF NOT EXISTS idx_g_gate_passes_entry_id ON gate_passes(entry_id);

-- ============================================
-- Gate Pass Pickups Table
-- ============================================
CREATE TABLE IF NOT EXISTS gate_pass_pickups (
    id SERIAL PRIMARY KEY,
    gate_pass_id INTEGER NOT NULL REFERENCES gate_passes(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL,
    room_no VARCHAR(10),
    floor VARCHAR(10),
    gatar_no VARCHAR(50),
    created_by_user_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_gate_pass_pickups_gate_pass_id ON gate_pass_pickups(gate_pass_id);

-- ============================================
-- Entry Events Table (Audit log)
-- ============================================
CREATE TABLE IF NOT EXISTS entry_events (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    notes TEXT,
    created_by_user_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_g_entry_events_entry_id ON entry_events(entry_id);
CREATE INDEX IF NOT EXISTS idx_g_entry_events_created_at ON entry_events(created_at);
CREATE INDEX IF NOT EXISTS idx_g_entry_events_status ON entry_events(status);

-- ============================================
-- Rent Payments Table
-- ============================================
CREATE TABLE IF NOT EXISTS rent_payments (
    id SERIAL PRIMARY KEY,
    receipt_number VARCHAR(20) UNIQUE,
    entry_id INTEGER NOT NULL REFERENCES entries(id),
    customer_name VARCHAR(100) NOT NULL,
    customer_phone VARCHAR(15) NOT NULL,
    total_rent DECIMAL(10, 2) NOT NULL,
    amount_paid DECIMAL(10, 2) NOT NULL,
    balance DECIMAL(10, 2) NOT NULL,
    payment_type VARCHAR(10) DEFAULT 'cash',
    payment_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_by_user_id INTEGER,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Migration: Add payment_type column if it doesn't exist
ALTER TABLE rent_payments ADD COLUMN IF NOT EXISTS payment_type VARCHAR(10) DEFAULT 'cash';

CREATE INDEX IF NOT EXISTS idx_g_rent_payments_entry_id ON rent_payments(entry_id);
CREATE INDEX IF NOT EXISTS idx_g_rent_payments_phone ON rent_payments(customer_phone);
CREATE INDEX IF NOT EXISTS idx_g_rent_payments_date ON rent_payments(payment_date);
CREATE INDEX IF NOT EXISTS idx_g_rent_payments_receipt ON rent_payments(receipt_number);

-- ============================================
-- System Settings Table
-- ============================================
CREATE TABLE IF NOT EXISTS system_settings (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) UNIQUE NOT NULL,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings
INSERT INTO system_settings (key, value) VALUES
    ('rent_rate_per_bag', '2.50'),
    ('rent_period_days', '30'),
    ('company_name', 'Gallery Storage'),
    ('company_phone', ''),
    ('company_address', '')
ON CONFLICT (key) DO NOTHING;

-- ============================================
-- Sequences for numbering
-- ============================================
CREATE SEQUENCE IF NOT EXISTS g_seed_entry_sequence START WITH 1 INCREMENT BY 1;
CREATE SEQUENCE IF NOT EXISTS g_sell_entry_sequence START WITH 1501 INCREMENT BY 1;
CREATE SEQUENCE IF NOT EXISTS g_receipt_number_sequence START WITH 1 INCREMENT BY 1;

-- ============================================
-- Keep existing gallery tables for backward compatibility
-- (items, txns, cfg, access_log, sessions)
-- These will continue to work with old gallery system
-- ============================================

-- Note: The existing tables (items, txns, cfg, access_log, sessions)
-- are preserved and not modified by this migration
