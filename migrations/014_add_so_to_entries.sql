-- Add S/O (Son Of / Father's Name) column to entries table
ALTER TABLE entries ADD COLUMN IF NOT EXISTS so VARCHAR(100);
