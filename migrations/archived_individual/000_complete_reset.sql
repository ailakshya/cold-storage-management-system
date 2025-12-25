-- Complete database reset and setup
-- Drop all tables in correct order (reverse of dependencies)
DROP TABLE IF EXISTS entries CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Create users table (for employee authentication)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'employee',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on email
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Create customers table (for cold storage customers)
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(15) UNIQUE NOT NULL,
    village VARCHAR(100),
    address TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes on customers
CREATE INDEX idx_customers_phone ON customers(phone);
CREATE INDEX idx_customers_name ON customers(name);

-- Create entries table (for storage entries)
CREATE TABLE entries (
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

-- Create indexes on entries
CREATE INDEX idx_entries_customer_id ON entries(customer_id);
CREATE INDEX idx_entries_phone ON entries(phone);
CREATE INDEX idx_entries_created_at ON entries(created_at);
CREATE INDEX idx_entries_truck_number ON entries(truck_number);
CREATE INDEX idx_entries_created_by_user ON entries(created_by_user_id);

-- Seed admin user (password: admin123)
INSERT INTO users (name, email, password_hash, role)
VALUES (
    'Administrator',
    'admin@cold.com',
    '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5BQJF.3fXv0jO',
    'admin'
);
