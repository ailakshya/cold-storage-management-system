-- Fix Family Member Payment Attribution
-- This script fixes mismatched family_member_name between entries and payments/ledger
-- Run this to ensure all payments are correctly attributed to family members

-- Step 1: Create a temp table with correct family member names from entries
CREATE TEMP TABLE correct_family_members AS
SELECT DISTINCT
    c.phone as customer_phone,
    e.family_member_name as correct_name,
    LOWER(TRIM(COALESCE(e.family_member_name, c.name))) as normalized_name
FROM entries e
JOIN customers c ON e.customer_id = c.id
WHERE COALESCE(e.status, 'active') != 'deleted'
  AND e.family_member_name IS NOT NULL
  AND e.family_member_name != '';

-- Step 2: For customers with only ONE family member, update ALL their payments to use that name
-- This fixes cases like Aakash/Aakesh where there's only one person storing items

-- First, identify customers with only one family member
CREATE TEMP TABLE single_family_customers AS
SELECT customer_phone, MAX(correct_name) as correct_name
FROM correct_family_members
GROUP BY customer_phone
HAVING COUNT(DISTINCT normalized_name) = 1;

-- Update ledger_entries for single-family customers
UPDATE ledger_entries le
SET family_member_name = sfc.correct_name
FROM single_family_customers sfc
WHERE le.customer_phone = sfc.customer_phone
  AND le.credit > 0  -- Only payment entries
  AND (le.family_member_name IS NULL
       OR le.family_member_name != sfc.correct_name);

-- Update rent_payments for single-family customers
UPDATE rent_payments rp
SET family_member_name = sfc.correct_name
FROM single_family_customers sfc
WHERE rp.customer_phone = sfc.customer_phone
  AND (rp.family_member_name IS NULL
       OR rp.family_member_name != sfc.correct_name);

-- Step 3: For multi-family customers, try to match by normalized name
UPDATE ledger_entries le
SET family_member_name = cfm.correct_name
FROM correct_family_members cfm
WHERE le.customer_phone = cfm.customer_phone
  AND LOWER(TRIM(COALESCE(le.family_member_name, ''))) = cfm.normalized_name
  AND le.family_member_name != cfm.correct_name;

UPDATE rent_payments rp
SET family_member_name = cfm.correct_name
FROM correct_family_members cfm
WHERE rp.customer_phone = cfm.customer_phone
  AND LOWER(TRIM(COALESCE(rp.family_member_name, ''))) = cfm.normalized_name
  AND rp.family_member_name != cfm.correct_name;

-- Step 4: For payments with empty family_member_name, set to customer name
UPDATE ledger_entries le
SET family_member_name = le.customer_name
WHERE le.credit > 0
  AND (le.family_member_name IS NULL OR le.family_member_name = '');

UPDATE rent_payments rp
SET family_member_name = rp.customer_name
WHERE rp.family_member_name IS NULL OR rp.family_member_name = '';

-- Step 5: Clean up temp tables
DROP TABLE IF EXISTS correct_family_members;
DROP TABLE IF EXISTS single_family_customers;

-- Step 6: Verify the fix - show any remaining mismatches
SELECT
    'Remaining mismatches' as status,
    le.customer_phone,
    le.customer_name,
    le.family_member_name as ledger_fm,
    e.family_member_name as entry_fm,
    le.credit as amount
FROM ledger_entries le
JOIN entries e ON le.customer_phone = (SELECT phone FROM customers WHERE id = e.customer_id)
WHERE le.credit > 0
  AND le.family_member_name != e.family_member_name
  AND e.family_member_name IS NOT NULL
  AND e.family_member_name != ''
  AND COALESCE(e.status, 'active') != 'deleted'
LIMIT 20;
