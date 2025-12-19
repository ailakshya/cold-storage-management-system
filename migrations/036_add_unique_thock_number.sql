-- Migration: Add unique constraint on thock_number
-- Purpose: Prevent duplicate entries with same thock_number
-- Date: 2025-12-20

-- First, check if there are existing duplicates and handle them
-- This creates a temporary fix by appending -DUP-N to duplicate thock_numbers
DO $$
DECLARE
    dup RECORD;
    counter INT;
BEGIN
    FOR dup IN
        SELECT thock_number, array_agg(id ORDER BY id) as ids
        FROM entries
        GROUP BY thock_number
        HAVING COUNT(*) > 1
    LOOP
        counter := 1;
        -- Skip the first ID (keep original), rename others
        FOR i IN 2..array_length(dup.ids, 1) LOOP
            UPDATE entries
            SET thock_number = dup.thock_number || '-DUP-' || counter
            WHERE id = dup.ids[i];
            counter := counter + 1;
        END LOOP;
    END LOOP;
END $$;

-- Now add the unique constraint
ALTER TABLE entries ADD CONSTRAINT entries_thock_number_unique UNIQUE (thock_number);

-- Also add unique constraint on room_entries if needed (entry_id + room_no combination)
-- This prevents same entry going to same room twice
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'room_entries_entry_room_unique'
    ) THEN
        ALTER TABLE room_entries ADD CONSTRAINT room_entries_entry_room_unique
        UNIQUE (entry_id, room_no);
    END IF;
EXCEPTION WHEN duplicate_object THEN
    NULL; -- Constraint already exists
END $$;
