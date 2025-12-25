-- Migration: Add CHECK constraints for data validation at database level
-- Ensures data integrity regardless of application layer validation

-- Rent payments validation
ALTER TABLE rent_payments
    ADD CONSTRAINT IF NOT EXISTS check_rent_payment_amounts
    CHECK (amount_paid >= 0 AND amount_paid <= total_rent AND total_rent >= 0);

ALTER TABLE rent_payments
    ADD CONSTRAINT IF NOT EXISTS check_rent_balance_calculation
    CHECK (balance = total_rent - amount_paid);

-- Gate passes validation
ALTER TABLE gate_passes
    ADD CONSTRAINT IF NOT EXISTS check_gate_pass_quantities
    CHECK (requested_quantity > 0);

ALTER TABLE gate_passes
    ADD CONSTRAINT IF NOT EXISTS check_gate_pass_approved_quantity
    CHECK (approved_quantity IS NULL OR approved_quantity <= requested_quantity);

ALTER TABLE gate_passes
    ADD CONSTRAINT IF NOT EXISTS check_gate_pass_picked_up
    CHECK (total_picked_up >= 0 AND (approved_quantity IS NULL OR total_picked_up <= COALESCE(approved_quantity, requested_quantity)));

-- Room entries validation
ALTER TABLE room_entries
    ADD CONSTRAINT IF NOT EXISTS check_room_entry_quantity
    CHECK (quantity > 0);

-- Entries validation
ALTER TABLE entries
    ADD CONSTRAINT IF NOT EXISTS check_entry_expected_quantity
    CHECK (expected_quantity > 0);

-- Invoices validation
ALTER TABLE invoices
    ADD CONSTRAINT IF NOT EXISTS check_invoice_total_amount
    CHECK (total_amount >= 0);

ALTER TABLE invoices
    ADD CONSTRAINT IF NOT EXISTS check_invoice_items_count
    CHECK (items_count >= 0);

-- Invoice items validation
ALTER TABLE invoice_items
    ADD CONSTRAINT IF NOT EXISTS check_invoice_item_quantity
    CHECK (quantity > 0);

ALTER TABLE invoice_items
    ADD CONSTRAINT IF NOT EXISTS check_invoice_item_rate
    CHECK (rate >= 0);

ALTER TABLE invoice_items
    ADD CONSTRAINT IF NOT EXISTS check_invoice_item_amount
    CHECK (amount >= 0);

-- Gate pass pickups validation
ALTER TABLE gate_pass_pickups
    ADD CONSTRAINT IF NOT EXISTS check_pickup_quantity
    CHECK (pickup_quantity > 0);
