-- Migration: Migrate Existing Payments to Ledger
-- Version: 003
-- Description: Populates ledger_entries from existing rent_payments data
-- Run this AFTER 002_add_ledger_tables.sql has been applied

-- =============================================================================
-- MIGRATE EXISTING PAYMENTS TO LEDGER
-- =============================================================================
-- Creates PAYMENT entries for all existing rent_payments
-- This preserves the audit trail from existing data

-- Insert PAYMENT entries from rent_payments
INSERT INTO ledger_entries (
    customer_phone,
    customer_name,
    customer_so,
    entry_type,
    description,
    debit,
    credit,
    running_balance,
    reference_id,
    reference_type,
    created_by_user_id,
    created_by_name,
    created_at,
    notes
)
SELECT
    rp.customer_phone,
    rp.customer_name,
    COALESCE(e.so, '') as customer_so,
    'PAYMENT' as entry_type,
    'Rent payment (migrated from existing data)' as description,
    0 as debit,
    rp.amount_paid as credit,
    0 as running_balance,  -- Will be recalculated
    rp.id as reference_id,
    'payment' as reference_type,
    COALESCE(rp.processed_by_user_id, 1) as created_by_user_id,
    COALESCE(rp.processed_by_name, 'System Migration') as created_by_name,
    rp.created_at,
    COALESCE(rp.notes, 'Migrated from rent_payments table')
FROM rent_payments rp
LEFT JOIN entries e ON rp.entry_id = e.id
WHERE rp.amount_paid > 0
AND NOT EXISTS (
    -- Don't insert if already migrated
    SELECT 1 FROM ledger_entries le
    WHERE le.reference_type = 'payment'
    AND le.reference_id = rp.id
)
ORDER BY rp.created_at;

-- =============================================================================
-- RECALCULATE RUNNING BALANCES
-- =============================================================================
-- Update running_balance for all entries based on chronological order per customer

WITH ordered_entries AS (
    SELECT
        id,
        customer_phone,
        debit,
        credit,
        SUM(debit - credit) OVER (
            PARTITION BY customer_phone
            ORDER BY created_at, id
        ) as calculated_balance
    FROM ledger_entries
)
UPDATE ledger_entries le
SET running_balance = oe.calculated_balance
FROM ordered_entries oe
WHERE le.id = oe.id;

-- =============================================================================
-- VERIFY MIGRATION
-- =============================================================================
-- This SELECT can be used to verify the migration results (not executed automatically)
-- SELECT
--     customer_phone,
--     COUNT(*) as entry_count,
--     SUM(credit) as total_payments,
--     MAX(running_balance) as final_balance
-- FROM ledger_entries
-- WHERE entry_type = 'PAYMENT'
-- GROUP BY customer_phone
-- ORDER BY total_payments DESC;

-- Add comment about migration
COMMENT ON TABLE ledger_entries IS 'Ledger for all financial transactions - includes migrated data from rent_payments';
