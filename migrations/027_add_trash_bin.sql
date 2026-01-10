-- Migration 027: Add trash bin for undo functionality
-- Stores deleted records as JSON for easy restoration

-- Trash bin table to store deleted records
CREATE TABLE IF NOT EXISTS trash_bin (
    id SERIAL PRIMARY KEY,
    table_name VARCHAR(100) NOT NULL,
    record_id INTEGER NOT NULL,
    record_data JSONB NOT NULL, -- Full record as JSON
    related_data JSONB, -- Related records (e.g., room_entries for an entry)
    deleted_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_by_user_id INTEGER REFERENCES users(id),
    deleted_by_username VARCHAR(100),
    delete_reason VARCHAR(500),
    expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '30 days'), -- Auto-purge after 30 days
    restored_at TIMESTAMPTZ,
    restored_by_user_id INTEGER REFERENCES users(id),
    UNIQUE(table_name, record_id, deleted_at)
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_trash_bin_table ON trash_bin(table_name);
CREATE INDEX IF NOT EXISTS idx_trash_bin_deleted_at ON trash_bin(deleted_at DESC);
CREATE INDEX IF NOT EXISTS idx_trash_bin_expires_at ON trash_bin(expires_at) WHERE restored_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_trash_bin_record ON trash_bin(table_name, record_id);

-- View for active (non-expired, non-restored) trash items
CREATE OR REPLACE VIEW v_trash_bin_active AS
SELECT
    tb.*,
    u.name as deleted_by_name
FROM trash_bin tb
LEFT JOIN users u ON tb.deleted_by_user_id = u.id
WHERE tb.restored_at IS NULL AND tb.expires_at > NOW()
ORDER BY tb.deleted_at DESC;

-- Function to move a record to trash bin
CREATE OR REPLACE FUNCTION move_to_trash(
    p_table_name VARCHAR,
    p_record_id INTEGER,
    p_deleted_by_user_id INTEGER DEFAULT NULL,
    p_delete_reason VARCHAR DEFAULT NULL
) RETURNS INTEGER AS $$
DECLARE
    v_record_data JSONB;
    v_related_data JSONB;
    v_deleted_by_username VARCHAR;
    v_trash_id INTEGER;
BEGIN
    -- Get username
    SELECT name INTO v_deleted_by_username FROM users WHERE id = p_deleted_by_user_id;

    -- Get record data based on table
    IF p_table_name = 'entries' THEN
        SELECT row_to_json(e)::JSONB INTO v_record_data
        FROM entries e WHERE id = p_record_id;

        -- Get related room_entries
        SELECT jsonb_agg(row_to_json(re)::JSONB) INTO v_related_data
        FROM room_entries re WHERE entry_id = p_record_id;

    ELSIF p_table_name = 'customers' THEN
        SELECT row_to_json(c)::JSONB INTO v_record_data
        FROM customers c WHERE id = p_record_id;

        -- Get related entries and family members
        SELECT jsonb_build_object(
            'entries', (SELECT jsonb_agg(row_to_json(e)::JSONB) FROM entries e WHERE customer_id = p_record_id),
            'family_members', (SELECT jsonb_agg(row_to_json(fm)::JSONB) FROM family_members fm WHERE customer_id = p_record_id)
        ) INTO v_related_data;

    ELSIF p_table_name = 'gate_passes' THEN
        SELECT row_to_json(gp)::JSONB INTO v_record_data
        FROM gate_passes gp WHERE id = p_record_id;

        -- Get related pickups
        SELECT jsonb_agg(row_to_json(gpp)::JSONB) INTO v_related_data
        FROM gate_pass_pickups gpp WHERE gate_pass_id = p_record_id;

    ELSIF p_table_name = 'room_entries' THEN
        SELECT row_to_json(re)::JSONB INTO v_record_data
        FROM room_entries re WHERE id = p_record_id;

        -- Get related gatars
        SELECT jsonb_agg(row_to_json(reg)::JSONB) INTO v_related_data
        FROM room_entry_gatars reg WHERE room_entry_id = p_record_id;

    ELSE
        -- Generic: try to get record as JSON
        EXECUTE format('SELECT row_to_json(t)::JSONB FROM %I t WHERE id = $1', p_table_name)
        INTO v_record_data USING p_record_id;
    END IF;

    -- If record not found, return null
    IF v_record_data IS NULL THEN
        RETURN NULL;
    END IF;

    -- Insert into trash bin
    INSERT INTO trash_bin (table_name, record_id, record_data, related_data, deleted_by_user_id, deleted_by_username, delete_reason)
    VALUES (p_table_name, p_record_id, v_record_data, v_related_data, p_deleted_by_user_id, v_deleted_by_username, p_delete_reason)
    RETURNING id INTO v_trash_id;

    RETURN v_trash_id;
END;
$$ LANGUAGE plpgsql;

-- Function to restore a record from trash bin
CREATE OR REPLACE FUNCTION restore_from_trash(
    p_trash_id INTEGER,
    p_restored_by_user_id INTEGER DEFAULT NULL
) RETURNS BOOLEAN AS $$
DECLARE
    v_trash_record trash_bin%ROWTYPE;
    v_success BOOLEAN := FALSE;
BEGIN
    -- Get trash record
    SELECT * INTO v_trash_record FROM trash_bin WHERE id = p_trash_id AND restored_at IS NULL;

    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    -- Restore based on table type
    IF v_trash_record.table_name = 'entries' THEN
        -- Update entry to undelete (assuming soft delete)
        UPDATE entries SET
            status = COALESCE((v_trash_record.record_data->>'status')::VARCHAR, 'active'),
            deleted_at = NULL,
            deleted_by_user_id = NULL
        WHERE id = v_trash_record.record_id;

        v_success := TRUE;

    ELSIF v_trash_record.table_name = 'customers' THEN
        -- Update customer to undelete
        UPDATE customers SET
            status = COALESCE((v_trash_record.record_data->>'status')::VARCHAR, 'active')
        WHERE id = v_trash_record.record_id;

        v_success := TRUE;

    ELSIF v_trash_record.table_name = 'gate_passes' THEN
        -- Update gate pass to undelete
        UPDATE gate_passes SET
            status = COALESCE((v_trash_record.record_data->>'status')::VARCHAR, 'pending')
        WHERE id = v_trash_record.record_id;

        v_success := TRUE;

    ELSE
        -- For other tables, we'd need specific restore logic
        v_success := FALSE;
    END IF;

    IF v_success THEN
        -- Mark as restored
        UPDATE trash_bin SET
            restored_at = NOW(),
            restored_by_user_id = p_restored_by_user_id
        WHERE id = p_trash_id;
    END IF;

    RETURN v_success;
END;
$$ LANGUAGE plpgsql;

-- Function to permanently delete expired trash items
CREATE OR REPLACE FUNCTION purge_expired_trash() RETURNS INTEGER AS $$
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM trash_bin
    WHERE expires_at < NOW() AND restored_at IS NULL;

    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;

    IF v_deleted_count > 0 THEN
        RAISE NOTICE 'Purged % expired trash items', v_deleted_count;
    END IF;

    RETURN v_deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Comment on tables
COMMENT ON TABLE trash_bin IS 'Stores deleted records for undo/restore functionality. Records auto-expire after 30 days.';
