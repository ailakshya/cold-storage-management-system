-- Migration: Add Ledger and Debt Request Tables
-- Version: 002
-- Description: Implements ledger-based accounting system with debt approval workflow

-- =============================================================================
-- LEDGER ENTRIES TABLE
-- =============================================================================
-- Stores all financial transactions for audit trail and balance calculation
-- Balance = SUM(debit) - SUM(credit) for each customer

CREATE TABLE IF NOT EXISTS ledger_entries (
    id SERIAL PRIMARY KEY,
    customer_phone VARCHAR(15) NOT NULL,
    customer_name VARCHAR(100) NOT NULL,
    customer_so VARCHAR(100),                    -- S/O (Son Of / Father's Name)
    entry_type VARCHAR(20) NOT NULL,             -- CHARGE, PAYMENT, CREDIT, REFUND, DEBT_APPROVAL
    description TEXT,
    debit DECIMAL(12,2) DEFAULT 0,               -- Money owed (increases balance)
    credit DECIMAL(12,2) DEFAULT 0,              -- Money paid/credited (decreases balance)
    running_balance DECIMAL(12,2) NOT NULL,      -- Balance after this entry
    reference_id INT,                            -- Links to entry_id, payment_id, gate_pass_id, debt_request_id
    reference_type VARCHAR(20),                  -- 'entry', 'payment', 'gate_pass', 'debt_request'
    created_by_user_id INT NOT NULL,
    created_by_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    notes TEXT,

    CONSTRAINT chk_entry_type CHECK (entry_type IN ('CHARGE', 'PAYMENT', 'CREDIT', 'REFUND', 'DEBT_APPROVAL')),
    CONSTRAINT chk_debit_credit CHECK (debit >= 0 AND credit >= 0)
);

-- Indexes for ledger_entries
CREATE INDEX IF NOT EXISTS idx_ledger_customer_phone ON ledger_entries(customer_phone);
CREATE INDEX IF NOT EXISTS idx_ledger_created_at ON ledger_entries(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledger_entry_type ON ledger_entries(entry_type);
CREATE INDEX IF NOT EXISTS idx_ledger_reference ON ledger_entries(reference_type, reference_id);

-- =============================================================================
-- DEBT REQUESTS TABLE
-- =============================================================================
-- Tracks requests from employees to allow item out when customer has outstanding balance
-- Workflow: Employee creates request -> Admin approves/rejects -> Gate pass can be issued

CREATE TABLE IF NOT EXISTS debt_requests (
    id SERIAL PRIMARY KEY,
    customer_phone VARCHAR(15) NOT NULL,
    customer_name VARCHAR(100) NOT NULL,
    customer_so VARCHAR(100),                    -- S/O (Son Of / Father's Name)
    thock_number VARCHAR(50) NOT NULL,
    requested_quantity INT NOT NULL,
    current_balance DECIMAL(12,2) NOT NULL,      -- How much customer owes at time of request
    requested_by_user_id INT NOT NULL,           -- Employee who made request
    requested_by_name VARCHAR(100),
    status VARCHAR(20) DEFAULT 'pending',        -- pending, approved, rejected, expired, used
    approved_by_user_id INT,
    approved_by_name VARCHAR(100),
    approved_at TIMESTAMP,
    rejection_reason TEXT,
    gate_pass_id INT,                            -- Linked gate pass after approval is used
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,                        -- Request expires after 24 hours

    CONSTRAINT chk_debt_status CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'used')),
    CONSTRAINT chk_requested_qty CHECK (requested_quantity > 0),
    CONSTRAINT chk_current_balance CHECK (current_balance >= 0)
);

-- Indexes for debt_requests
CREATE INDEX IF NOT EXISTS idx_debt_requests_status ON debt_requests(status);
CREATE INDEX IF NOT EXISTS idx_debt_requests_customer ON debt_requests(customer_phone);
CREATE INDEX IF NOT EXISTS idx_debt_requests_thock ON debt_requests(thock_number);
CREATE INDEX IF NOT EXISTS idx_debt_requests_created_at ON debt_requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_debt_requests_requested_by ON debt_requests(requested_by_user_id);

-- =============================================================================
-- SEQUENCE FOR LEDGER ENTRY IDs (if needed)
-- =============================================================================
-- PostgreSQL SERIAL handles this automatically, but explicit sequence for reference
CREATE SEQUENCE IF NOT EXISTS ledger_entries_id_seq;

-- =============================================================================
-- COMMENTS
-- =============================================================================
COMMENT ON TABLE ledger_entries IS 'Ledger for all financial transactions - CHARGE (rent), PAYMENT, CREDIT, REFUND, DEBT_APPROVAL';
COMMENT ON COLUMN ledger_entries.entry_type IS 'CHARGE=rent owed, PAYMENT=money received, CREDIT=discount, REFUND=money returned, DEBT_APPROVAL=audit record for credit withdrawal';
COMMENT ON COLUMN ledger_entries.running_balance IS 'Balance after this entry = previous_balance + debit - credit';
COMMENT ON COLUMN ledger_entries.reference_type IS 'Links to source: entry, payment, gate_pass, debt_request';

COMMENT ON TABLE debt_requests IS 'Requests for item withdrawal when customer has outstanding balance';
COMMENT ON COLUMN debt_requests.status IS 'pending=awaiting admin, approved=can create gate pass, rejected=denied, expired=24hr timeout, used=gate pass created';
COMMENT ON COLUMN debt_requests.gate_pass_id IS 'Populated when debt approval is used to create a gate pass';
