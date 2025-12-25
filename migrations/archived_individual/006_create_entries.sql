-- Create entries table for tracking customer storage entries
CREATE TABLE IF NOT EXISTS entries (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id) ON DELETE CASCADE,
    phone VARCHAR(15) NOT NULL,
    name VARCHAR(100) NOT NULL,
    village VARCHAR(100),
    expected_quantity INTEGER NOT NULL,
    truck_category VARCHAR(20) NOT NULL CHECK (truck_category IN ('seed', 'sell')),
    truck_number VARCHAR(50),
    created_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_entries_customer_id ON entries(customer_id);
CREATE INDEX IF NOT EXISTS idx_entries_phone ON entries(phone);
CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);
CREATE INDEX IF NOT EXISTS idx_entries_truck_number ON entries(truck_number);
CREATE INDEX IF NOT EXISTS idx_entries_created_by_user ON entries(created_by_user_id);
