-- Update existing NULL values to empty string
UPDATE entries SET so = '' WHERE so IS NULL;

-- Set default value for future entries
ALTER TABLE entries ALTER COLUMN so SET DEFAULT '';

-- Make the column NOT NULL now that we've cleaned up
ALTER TABLE entries ALTER COLUMN so SET NOT NULL;
