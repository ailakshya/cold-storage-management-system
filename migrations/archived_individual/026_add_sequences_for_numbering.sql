-- Migration: Add sequences for receipt and invoice numbering
-- Replaces slow COUNT(*) operations with O(1) sequence operations

-- Create sequence for receipt numbers (rent payments)
CREATE SEQUENCE IF NOT EXISTS receipt_number_sequence START 1;

-- Create sequence for invoice numbers
CREATE SEQUENCE IF NOT EXISTS invoice_number_sequence START 1;

-- Set sequences based on existing data
DO $$
DECLARE
    max_receipt_num INT;
    max_invoice_num INT;
BEGIN
    -- Extract numeric part from existing receipt numbers (format: RCP-000001)
    SELECT COALESCE(
        MAX(CAST(SUBSTRING(receipt_number FROM 'RCP-(\d+)') AS INT)),
        0
    ) INTO max_receipt_num
    FROM rent_payments
    WHERE receipt_number ~ '^RCP-\d+$';

    -- Extract numeric part from existing invoice numbers (format: INV-000001)
    SELECT COALESCE(
        MAX(CAST(SUBSTRING(invoice_number FROM 'INV-(\d+)') AS INT)),
        0
    ) INTO max_invoice_num
    FROM invoices
    WHERE invoice_number ~ '^INV-\d+$';

    -- Set sequences to next available number
    PERFORM setval('receipt_number_sequence', GREATEST(max_receipt_num + 1, 1), false);
    PERFORM setval('invoice_number_sequence', GREATEST(max_invoice_num + 1, 1), false);
END $$;
