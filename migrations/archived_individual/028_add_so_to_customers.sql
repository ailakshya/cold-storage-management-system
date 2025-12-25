-- Add S/O (Son Of / Father's Name) column to customers table
-- The father's name is a property of the customer, not individual entries
ALTER TABLE customers ADD COLUMN IF NOT EXISTS so VARCHAR(100);

-- Create index for searching by father's name
CREATE INDEX IF NOT EXISTS idx_customers_so ON customers(so);
