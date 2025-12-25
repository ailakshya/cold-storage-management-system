-- Add receipt_number to rent_payments table
ALTER TABLE rent_payments ADD COLUMN IF NOT EXISTS receipt_number VARCHAR(50) UNIQUE;

-- Create index for faster lookups by receipt number
CREATE INDEX IF NOT EXISTS idx_rent_payments_receipt_number ON rent_payments(receipt_number);

-- Update existing records with generated receipt numbers
DO $$
DECLARE
    rec RECORD;
    date_prefix TEXT;
    count_val INTEGER;
    new_receipt_number TEXT;
BEGIN
    FOR rec IN SELECT id, payment_date FROM rent_payments WHERE receipt_number IS NULL ORDER BY id
    LOOP
        date_prefix := TO_CHAR(rec.payment_date, 'YYYYMMDD');

        SELECT COUNT(*) INTO count_val
        FROM rent_payments
        WHERE receipt_number LIKE 'RCP-' || date_prefix || '-%';

        new_receipt_number := 'RCP-' || date_prefix || '-' || LPAD((count_val + 1)::TEXT, 4, '0');

        UPDATE rent_payments SET receipt_number = new_receipt_number WHERE id = rec.id;
    END LOOP;
END $$;

-- Make receipt_number NOT NULL after populating existing records
ALTER TABLE rent_payments ALTER COLUMN receipt_number SET NOT NULL;
