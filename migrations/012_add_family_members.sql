-- Migration: Add family members feature
-- Allows one customer (phone) to have multiple family members
-- Each entry can be assigned to a specific family member

-- Step 1: Create family_members table
CREATE TABLE IF NOT EXISTS family_members (
    id SERIAL PRIMARY KEY,
    customer_id INT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    relation VARCHAR(50) DEFAULT 'Other',  -- Self, Son, Daughter, Brother, Sister, Father, Mother, Wife, Husband, Partner, Other
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(customer_id, name)
);

CREATE INDEX IF NOT EXISTS idx_family_members_customer_id ON family_members(customer_id);

-- Step 2: Add family_member columns to entries table
ALTER TABLE entries ADD COLUMN IF NOT EXISTS family_member_id INT REFERENCES family_members(id) ON DELETE SET NULL;
ALTER TABLE entries ADD COLUMN IF NOT EXISTS family_member_name VARCHAR(100);

CREATE INDEX IF NOT EXISTS idx_entries_family_member_id ON entries(family_member_id);

-- Step 3: Auto-create family members from existing entry names
-- For each customer, create family members based on unique names in their entries
INSERT INTO family_members (customer_id, name, relation, is_default)
SELECT DISTINCT
    c.id as customer_id,
    e.name as name,
    CASE WHEN LOWER(TRIM(e.name)) = LOWER(TRIM(c.name)) THEN 'Self' ELSE 'Other' END as relation,
    CASE WHEN LOWER(TRIM(e.name)) = LOWER(TRIM(c.name)) THEN true ELSE false END as is_default
FROM customers c
JOIN entries e ON e.customer_id = c.id
WHERE c.status = 'active'
ON CONFLICT (customer_id, name) DO NOTHING;

-- Step 4: Link entries to their family members by matching name
UPDATE entries e
SET family_member_id = fm.id,
    family_member_name = fm.name
FROM family_members fm
WHERE fm.customer_id = e.customer_id
AND fm.name = e.name;

-- Step 5: For customers without any family members yet (no entries), we don't create anything
-- Family members will be created automatically when first entry is made
