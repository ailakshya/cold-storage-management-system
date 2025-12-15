-- Create gate_passes table for unloading mode
CREATE TABLE IF NOT EXISTS gate_passes (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    truck_number VARCHAR(20) NOT NULL,
    entry_id INTEGER REFERENCES entries(id) ON DELETE SET NULL,
    requested_quantity INTEGER NOT NULL,
    approved_quantity INTEGER,
    gate_no VARCHAR(50),
    status VARCHAR(20) DEFAULT 'pending',
    payment_verified BOOLEAN DEFAULT false,
    payment_amount DECIMAL(10,2),
    issued_by_user_id INTEGER REFERENCES users(id),
    approved_by_user_id INTEGER REFERENCES users(id),
    issued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    remarks TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Add indexes
CREATE INDEX IF NOT EXISTS idx_gate_passes_customer_id ON gate_passes(customer_id);
CREATE INDEX IF NOT EXISTS idx_gate_passes_truck_number ON gate_passes(truck_number);
CREATE INDEX IF NOT EXISTS idx_gate_passes_status ON gate_passes(status);
CREATE INDEX IF NOT EXISTS idx_gate_passes_entry_id ON gate_passes(entry_id);

-- Status values: 'pending', 'approved', 'completed', 'cancelled'
