-- Migration: Add sequences for entry numbering to prevent race conditions
-- This fixes the critical bug where concurrent entry creation could generate duplicate truck numbers

-- Create sequence for SEED category (starts at 1)
CREATE SEQUENCE IF NOT EXISTS seed_entry_sequence START 1;

-- Create sequence for SELL category (starts at 1501)
CREATE SEQUENCE IF NOT EXISTS sell_entry_sequence START 1501;

-- Set sequence values based on existing data to avoid conflicts
DO $$
DECLARE
    max_seed_number INT;
    max_sell_number INT;
BEGIN
    -- Get highest existing seed entry number
    SELECT COALESCE(
        MAX(CAST(SPLIT_PART(truck_number, '/', 1) AS INT)),
        0
    ) INTO max_seed_number
    FROM entries
    WHERE truck_category = 'seed'
    AND truck_number ~ '^\d+/\d+$';

    -- Get highest existing sell entry number
    SELECT COALESCE(
        MAX(CAST(SPLIT_PART(truck_number, '/', 1) AS INT)),
        1500
    ) INTO max_sell_number
    FROM entries
    WHERE truck_category = 'sell'
    AND truck_number ~ '^\d+/\d+$';

    -- Set sequences to next available number
    PERFORM setval('seed_entry_sequence', GREATEST(max_seed_number + 1, 1), false);
    PERFORM setval('sell_entry_sequence', GREATEST(max_sell_number + 1, 1501), false);
END $$;
