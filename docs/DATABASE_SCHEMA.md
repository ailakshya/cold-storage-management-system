# Database Schema Documentation

Complete database schema and entity relationship documentation for the Cold Storage Management System.

**Database:** PostgreSQL 17 (CloudNative-PG)
**Schema Version:** 1.5.173
**Last Updated:** 2026-01-17

---

## Table of Contents

- [Entity Relationship Diagram](#entity-relationship-diagram)
- [Tables Overview](#tables-overview)
- [Detailed Schema](#detailed-schema)
- [Relationships](#relationships)
- [Indexes](#indexes)
- [Constraints](#constraints)
- [Sample Queries](#sample-queries)
- [Migration History](#migration-history)

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         DATABASE SCHEMA                             │
└─────────────────────────────────────────────────────────────────────┘

    ┌──────────────────┐
    │      users       │
    ├──────────────────┤
    │ id (PK)          │
    │ email (UNIQUE)   │
    │ password_hash    │
    │ role             │
    │ name             │
    │ phone            │
    │ village          │
    │ created_at       │
    │ updated_at       │
    └────────┬─────────┘
             │
             │ created_by_user_id
             │ processed_by_user_id
             │
    ┌────────┴─────────┬────────────────────────────────────┐
    │                  │                                    │
    ▼                  ▼                                    ▼
┌──────────┐    ┌──────────────┐                  ┌─────────────────┐
│ entries  │    │ rent_payments│                  │                 │
├──────────┤    ├──────────────┤                  │                 │
│ id (PK)  │◄───┤ entry_id (FK)│                  │                 │
│ customer_id   ├──────────────┤                  │                 │
│ phone    │    │ id (PK)      │                  │                 │
│ name     │    │ customer_name│                  │                 │
│ village  │    │ customer_phone                  │                 │
│ expected_qty  │ total_rent   │                  │                 │
│ truck_cat│    │ amount_paid  │                  │                 │
│ truck_no │    │ balance      │                  │                 │
│ created_by    │ payment_date │                  │                 │
│ created_at    │ processed_by │                  │                 │
│ updated_at    │ notes        │                  │                 │
└────┬─────┘    │ created_at   │                  │                 │
     │          └──────────────┘                  │                 │
     │                                            │                 │
     │ entry_id                                   │                 │
     │                                            │                 │
     ├────────────────────┬───────────────────────┤                 │
     │                    │                       │                 │
     ▼                    ▼                       │                 │
┌──────────────┐   ┌──────────────┐              │                 │
│ room_entries │   │ entry_events │              │                 │
├──────────────┤   ├──────────────┤              │                 │
│ id (PK)      │   │ id (PK)      │              │                 │
│ entry_id (FK)│   │ entry_id (FK)│              │                 │
│ room_no      │   │ event_type   │              │                 │
│ floor        │   │ description  │              │                 │
│ gate_no      │   │ created_at   │              │                 │
│ quantity     │   └──────────────┘              │                 │
│ remark       │                                 │                 │
│ created_at   │                                 │                 │
│ updated_at   │                                 │                 │
└──────────────┘                                 │                 │
                                                 │                 │
    ┌────────────────────────────────────────────┘                 │
    │                                                               │
    ▼                                                               │
┌────────────────┐                                                 │
│   customers    │                                                 │
├────────────────┤                                                 │
│ id (PK)        │◄────────────────────────────────────────────────┘
│ name           │  customer_id
│ phone (UNIQUE) │
│ village        │
│ address        │
│ created_at     │
│ updated_at     │
└────────────────┘


                    ┌─────────────────────┐
                    │  system_settings    │
                    ├─────────────────────┤
                    │ id (PK)             │
                    │ setting_key (UNIQUE)│
                    │ setting_value       │
                    │ description         │
                    │ created_at          │
                    │ updated_at          │
                    └─────────────────────┘
```

---

## Tables Overview

### Main Application Database

**Core Business Tables:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **users** | System users (employee, accountant, admin, guard) | Low | email, role, password_hash, is_active |
| **customers** | Cold storage customers | Medium | phone (unique), name, village |
| **entries** | Storage entries (truck arrivals) | High | truck_number, customer_id, expected_quantity, deleted_at |
| **room_entries** | Physical storage locations | High | entry_id, room_no, floor, gate_no |
| **room_entry_gatars** | Gatar-level tracking for room entries | High | room_entry_id, gatar_number, quantity |
| **entry_events** | Entry lifecycle events | High | entry_id, event_type |
| **system_settings** | Application configuration | Very Low | setting_key (unique), setting_value |

**Family & Customer Features:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **family_members** | Customer family members | Medium | customer_id, name, relation, phone |
| **customer_activity_logs** | Customer portal activity tracking | High | customer_id, activity_type, ip_address |
| **customer_otps** | Customer OTP codes for portal login | Medium | customer_id, otp_code, expires_at |
| **customer_login_logs** | Customer portal login audit | High | customer_id, ip_address, success |

**Guard & Token System:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **guard_entries** | Guard register truck entries | High | token_number, customer_name, seed_quantity, sell_quantity |
| **token_colors** | Daily token color assignments | Low | date (unique), color |
| **skipped_tokens** | Skipped/lost token tracking | Low | token_number, color, reason |

**Gate Pass & Invoicing:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **gate_passes** | Gate pass requests/approvals | High | entry_id, requested_quantity, status, family_member_id |
| **gate_pass_pickup_gatars** | Pickup gatar tracking | High | gate_pass_id, gatar_number, quantity_picked |
| **invoices** | Loading invoices | High | entry_id, invoice_number, total_amount |
| **invoice_items** | Invoice line items | High | invoice_id, description, quantity, rate |

**Financial & Accounting:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **rent_payments** | Payment records | High | entry_id, total_rent, amount_paid, balance, receipt_number, family_member_id |
| **ledger_entries** | Double-entry accounting ledger | High | customer_id, entry_type, debit, credit, balance |
| **debt_requests** | Debt approval workflow | Medium | customer_id, amount, status, approved_by |
| **online_transactions** | Razorpay payment transactions | Medium | customer_id, razorpay_order_id, amount, status |

**Audit & Logging:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **login_logs** | User login audit trail | High | user_id, ip_address, success, user_agent |
| **room_entry_edit_logs** | Room entry edit history | Medium | room_entry_id, old_values, new_values, edited_by |
| **entry_edit_logs** | Entry edit history | Medium | entry_id, field_name, old_value, new_value |
| **entry_management_logs** | Entry reassignment/merge logs | Low | action_type, source_entry_id, target_entry_id |
| **admin_action_logs** | Admin action audit trail | Medium | user_id, action_type, description |
| **merge_history** | Customer merge history | Low | source_customer_id, target_customer_id, merged_by |

**Communication:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **sms_logs** | SMS/WhatsApp delivery logs | High | phone_number, message, status, provider |

**Security & Session:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **totp_secrets** | 2FA TOTP secrets | Very Low | user_id, secret, enabled |
| **totp_backup_codes** | 2FA backup codes | Very Low | user_id, code, used |
| **totp_verification_attempts** | 2FA attempt tracking | Low | user_id, success, ip_address |

**Infrastructure:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **external_databases** | External DB connections | Very Low | name, host, port, credentials |
| **deployment_config** | Deployment configurations | Very Low | name, repository, branch |
| **deployment_history** | Deployment history | Medium | config_id, version, status, deployed_by |
| **infra_config** | Infrastructure settings | Very Low | key, value |
| **cluster_nodes** | K8s cluster nodes | Very Low | name, ip_address, role, status |
| **ssh_keys** | SSH keys for provisioning | Very Low | name, public_key, private_key |
| **node_provision_logs** | Node provisioning logs | Low | node_id, log_text, created_at |
| **infra_action_logs** | Infrastructure action logs | Medium | action_type, node_id, user_id |

**Season Management:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **season_requests** | Season change requests (dual approval) | Very Low | season_name, requested_by, approved_by, status |

**Protected Settings:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **pending_setting_changes** | Protected setting change requests | Low | setting_key, old_value, new_value, requested_by |
| **protected_settings** | List of protected settings | Very Low | setting_key, requires_dual_approval |

### TimescaleDB Metrics Database

**Performance Monitoring:**

| Table | Purpose | Records | Key Fields |
|-------|---------|---------|------------|
| **api_request_logs** | API request logging (hypertable) | Very High | endpoint, method, status_code, duration_ms, timestamp |
| **node_metrics** | K8s node metrics (hypertable) | Very High | node_name, cpu_usage, memory_usage, disk_usage, timestamp |
| **postgres_metrics** | PostgreSQL metrics (hypertable) | Very High | connections, transactions, cache_hit_ratio, timestamp |
| **monitoring_alerts** | System alerts (hypertable) | High | alert_type, severity, message, resolved, timestamp |
| **alert_thresholds** | Alert threshold configuration | Very Low | metric_name, warning_threshold, critical_threshold |
| **backup_history** | Backup history tracking | Medium | backup_type, size_bytes, success, duration_seconds |
| **vip_status** | VIP availability tracking | High | vip_address, is_available, checked_at |

---

## Detailed Schema

### 1. users

Stores system users who operate the application (employees, accountants, admins).

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE NOT NULL,
    phone VARCHAR(15),
    village VARCHAR(100),
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'employee' CHECK (role IN ('employee', 'accountant', 'admin')),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `name` - User's full name
- `email` - Unique email address (used for login)
- `phone` - Optional contact number
- `village` - Optional village/location
- `password_hash` - Bcrypt hashed password
- `role` - User role: employee, accountant, or admin
- `created_at` - Record creation timestamp
- `updated_at` - Last update timestamp

**Sample Data:**
```sql
INSERT INTO users (email, password_hash, name, role) VALUES
('admin@cold.com', '$2a$10$hash...', 'Admin User', 'admin'),
('employee@test.com', '$2a$10$hash...', 'Test Employee', 'employee'),
('accountant@test.com', '$2a$10$hash...', 'Test Accountant', 'accountant');
```

---

### 2. customers

Stores information about customers who use cold storage services.

```sql
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(15) UNIQUE NOT NULL,
    village VARCHAR(100),
    address TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_customers_phone ON customers(phone);
CREATE INDEX idx_customers_name ON customers(name);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `name` - Customer's full name
- `phone` - Unique phone number (used for searching)
- `village` - Customer's village/location
- `address` - Complete address (optional)
- `created_at` - Record creation timestamp
- `updated_at` - Last update timestamp

**Business Rules:**
- Phone number must be unique (primary identifier)
- Phone is used for customer lookup
- Name and village are required for entries

---

### 3. entries

Main table storing storage entry records (truck arrivals).

```sql
CREATE TABLE entries (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id),
    phone VARCHAR(15) NOT NULL,
    name VARCHAR(100) NOT NULL,
    village VARCHAR(100),
    expected_quantity INTEGER NOT NULL CHECK (expected_quantity > 0),
    truck_category VARCHAR(10) NOT NULL CHECK (truck_category IN ('seed', 'sell')),
    truck_number VARCHAR(20) NOT NULL,
    created_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_entries_customer_id ON entries(customer_id);
CREATE INDEX idx_entries_phone ON entries(phone);
CREATE INDEX idx_entries_truck_number ON entries(truck_number);
CREATE INDEX idx_entries_created_at ON entries(created_at);
CREATE INDEX idx_entries_truck_category ON entries(truck_category);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `customer_id` - Foreign key to customers table
- `phone` - Denormalized customer phone (snapshot)
- `name` - Denormalized customer name (snapshot)
- `village` - Denormalized village (snapshot)
- `expected_quantity` - Expected quantity to be stored (1-1500)
- `truck_category` - Category: 'seed' (1-600) or 'sell' (601-1500)
- `truck_number` - Auto-generated unique truck number (e.g., "SEED-001", "015/300")
- `created_by_user_id` - Foreign key to users (which employee created this)
- `created_at` - Record creation timestamp
- `updated_at` - Last update timestamp

**Business Rules:**
- Truck category automatically assigned based on quantity:
  - 1-600: "seed"
  - 601-1500: "sell"
- Truck number auto-generated sequentially per category
- Customer data denormalized for historical accuracy

---

### 4. room_entries

Physical storage location assignments for entries.

```sql
CREATE TABLE room_entries (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    room_no VARCHAR(20) NOT NULL,
    floor VARCHAR(10) NOT NULL,
    gate_no VARCHAR(10) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    remark TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_room_entries_entry_id ON room_entries(entry_id);
CREATE INDEX idx_room_entries_room_no ON room_entries(room_no);
CREATE INDEX idx_room_entries_floor ON room_entries(floor);
CREATE INDEX idx_room_entries_gate_no ON room_entries(gate_no);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `entry_id` - Foreign key to entries table (CASCADE DELETE)
- `room_no` - Room number (e.g., "A-101", "B-205")
- `floor` - Floor number (e.g., "1", "2", "G")
- `gate_no` - Gate number (e.g., "G1", "G2")
- `quantity` - Quantity stored in this room
- `remark` - Optional notes/remarks
- `created_at` - Record creation timestamp
- `updated_at` - Last update timestamp

**Business Rules:**
- One entry can have multiple room assignments
- Quantity can be split across multiple rooms
- Cascade delete when parent entry is deleted

---

### 5. entry_events

Audit trail for entry lifecycle events.

```sql
CREATE TABLE entry_events (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_entry_events_entry_id ON entry_events(entry_id);
CREATE INDEX idx_entry_events_created_at ON entry_events(created_at);
CREATE INDEX idx_entry_events_type ON entry_events(event_type);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `entry_id` - Foreign key to entries table (CASCADE DELETE)
- `event_type` - Type of event (e.g., "CREATED", "ROOM_ASSIGNED", "LOADED")
- `description` - Event description/details
- `created_at` - Event timestamp

**Event Types:**
- `CREATED` - Entry created
- `ROOM_ASSIGNED` - Assigned to storage room
- `ROOM_UPDATED` - Room assignment updated
- `LOADED` - Loaded onto truck
- `PAYMENT_RECEIVED` - Payment recorded

---

### 6. rent_payments

Payment records for storage rent.

```sql
CREATE TABLE rent_payments (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id),
    customer_name VARCHAR(100) NOT NULL,
    customer_phone VARCHAR(15) NOT NULL,
    total_rent NUMERIC(10, 2) NOT NULL CHECK (total_rent > 0),
    amount_paid NUMERIC(10, 2) NOT NULL CHECK (amount_paid > 0),
    balance NUMERIC(10, 2) NOT NULL,
    payment_date TIMESTAMP DEFAULT NOW(),
    processed_by_user_id INTEGER REFERENCES users(id),
    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_rent_payments_entry_id ON rent_payments(entry_id);
CREATE INDEX idx_rent_payments_customer_phone ON rent_payments(customer_phone);
CREATE INDEX idx_rent_payments_payment_date ON rent_payments(payment_date);
CREATE INDEX idx_rent_payments_processed_by ON rent_payments(processed_by_user_id);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `entry_id` - Foreign key to entries table
- `customer_name` - Customer name (denormalized snapshot)
- `customer_phone` - Customer phone (denormalized for searching)
- `total_rent` - Total rent amount
- `amount_paid` - Amount paid in this transaction
- `balance` - Cumulative balance (negative = owed, positive = credit)
- `payment_date` - Payment timestamp
- `processed_by_user_id` - Foreign key to users (accountant who processed)
- `notes` - Optional payment notes
- `created_at` - Record creation timestamp

**Business Rules:**
- Balance is cumulative: (Total Paid So Far + Current Amount) - Total Rent
- Negative balance indicates amount still owed
- Positive balance indicates overpayment/credit
- Multiple payments per entry allowed (installments)

**Balance Calculation Example:**
```
Entry Total Rent: ₹10,000

Payment 1: ₹3,000 paid → Balance: ₹3,000 - ₹10,000 = -₹7,000 (owed)
Payment 2: ₹4,000 paid → Balance: ₹7,000 - ₹10,000 = -₹3,000 (owed)
Payment 3: ₹3,000 paid → Balance: ₹10,000 - ₹10,000 = ₹0 (fully paid)
```

---

### 7. system_settings

Application-wide configuration settings.

```sql
CREATE TABLE system_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(100) UNIQUE NOT NULL,
    setting_value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_system_settings_key ON system_settings(setting_key);
```

**Fields:**
- `id` - Auto-incrementing primary key
- `setting_key` - Unique setting identifier
- `setting_value` - Setting value (stored as text)
- `description` - Human-readable description
- `created_at` - Record creation timestamp
- `updated_at` - Last update timestamp

**Default Settings:**
```sql
INSERT INTO system_settings (setting_key, setting_value, description) VALUES
('rent_per_item', '10.00', 'Rent price per item stored'),
('company_name', 'Cold Storage Solutions', 'Company name for receipts'),
('company_address', '123 Main Street', 'Company address for receipts'),
('currency', 'INR', 'Currency code');
```

---

## Relationships

### One-to-Many Relationships

1. **users → entries** (created_by_user_id)
   - One user creates many entries
   - Tracks which employee created each entry

2. **users → rent_payments** (processed_by_user_id)
   - One user processes many payments
   - Tracks which accountant processed each payment

3. **customers → entries** (customer_id)
   - One customer has many storage entries
   - Links entries to customer records

4. **entries → room_entries** (entry_id)
   - One entry can be split across multiple rooms
   - Allows partial room assignments

5. **entries → entry_events** (entry_id)
   - One entry has many lifecycle events
   - Complete audit trail

6. **entries → rent_payments** (entry_id)
   - One entry can have multiple payments
   - Supports installment payments

### Cascade Behaviors

- **room_entries.entry_id** → CASCADE DELETE
  - When entry deleted, all room assignments deleted

- **entry_events.entry_id** → CASCADE DELETE
  - When entry deleted, all events deleted

- **rent_payments.entry_id** → NO CASCADE
  - Payments retained even if entry deleted (financial records)

---

## Indexes

### Performance Indexes

```sql
-- Users
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Customers
CREATE INDEX idx_customers_phone ON customers(phone);
CREATE INDEX idx_customers_name ON customers(name);

-- Entries
CREATE INDEX idx_entries_customer_id ON entries(customer_id);
CREATE INDEX idx_entries_phone ON entries(phone);
CREATE INDEX idx_entries_truck_number ON entries(truck_number);
CREATE INDEX idx_entries_created_at ON entries(created_at);
CREATE INDEX idx_entries_truck_category ON entries(truck_category);

-- Room Entries
CREATE INDEX idx_room_entries_entry_id ON room_entries(entry_id);
CREATE INDEX idx_room_entries_room_no ON room_entries(room_no);
CREATE INDEX idx_room_entries_floor ON room_entries(floor);
CREATE INDEX idx_room_entries_gate_no ON room_entries(gate_no);

-- Entry Events
CREATE INDEX idx_entry_events_entry_id ON entry_events(entry_id);
CREATE INDEX idx_entry_events_created_at ON entry_events(created_at);
CREATE INDEX idx_entry_events_type ON entry_events(event_type);

-- Rent Payments
CREATE INDEX idx_rent_payments_entry_id ON rent_payments(entry_id);
CREATE INDEX idx_rent_payments_customer_phone ON rent_payments(customer_phone);
CREATE INDEX idx_rent_payments_payment_date ON rent_payments(payment_date);

-- System Settings
CREATE INDEX idx_system_settings_key ON system_settings(setting_key);
```

---

## Constraints

### Primary Keys
- All tables have `id SERIAL PRIMARY KEY`

### Foreign Keys
- `entries.customer_id` → `customers.id`
- `entries.created_by_user_id` → `users.id`
- `room_entries.entry_id` → `entries.id` (CASCADE DELETE)
- `entry_events.entry_id` → `entries.id` (CASCADE DELETE)
- `rent_payments.entry_id` → `entries.id`
- `rent_payments.processed_by_user_id` → `users.id`

### Unique Constraints
- `users.email` - UNIQUE
- `customers.phone` - UNIQUE
- `system_settings.setting_key` - UNIQUE

### Check Constraints
- `users.role` - IN ('employee', 'accountant', 'admin')
- `entries.expected_quantity` - > 0
- `entries.truck_category` - IN ('seed', 'sell')
- `room_entries.quantity` - > 0
- `rent_payments.total_rent` - > 0
- `rent_payments.amount_paid` - > 0

---

## Sample Queries

### Get all entries with customer and creator info
```sql
SELECT
    e.id,
    e.truck_number,
    e.expected_quantity,
    c.name AS customer_name,
    c.phone AS customer_phone,
    u.name AS created_by
FROM entries e
LEFT JOIN customers c ON e.customer_id = c.id
LEFT JOIN users u ON e.created_by_user_id = u.id
ORDER BY e.created_at DESC
LIMIT 50;
```

### Get payment history for a customer
```sql
SELECT
    rp.payment_date,
    rp.total_rent,
    rp.amount_paid,
    rp.balance,
    rp.notes,
    e.truck_number,
    u.name AS processed_by
FROM rent_payments rp
JOIN entries e ON rp.entry_id = e.id
JOIN users u ON rp.processed_by_user_id = u.id
WHERE rp.customer_phone = '9999999999'
ORDER BY rp.payment_date DESC;
```

### Get room allocation summary
```sql
SELECT
    re.floor,
    re.gate_no,
    COUNT(*) AS total_rooms_used,
    SUM(re.quantity) AS total_quantity
FROM room_entries re
GROUP BY re.floor, re.gate_no
ORDER BY re.floor, re.gate_no;
```

### Get entries without room assignments
```sql
SELECT
    e.id,
    e.truck_number,
    e.name,
    e.expected_quantity
FROM entries e
LEFT JOIN room_entries re ON e.id = re.entry_id
WHERE re.id IS NULL
ORDER BY e.created_at DESC;
```

### Get today's statistics
```sql
SELECT
    COUNT(DISTINCT e.id) AS entries_today,
    SUM(e.expected_quantity) AS total_quantity,
    COUNT(DISTINCT CASE WHEN e.truck_category = 'seed' THEN e.id END) AS seed_entries,
    COUNT(DISTINCT CASE WHEN e.truck_category = 'sell' THEN e.id END) AS sell_entries
FROM entries e
WHERE DATE(e.created_at) = CURRENT_DATE;
```

### Get payment summary by date range
```sql
SELECT
    DATE(payment_date) AS date,
    COUNT(*) AS transaction_count,
    SUM(amount_paid) AS total_collected,
    SUM(CASE WHEN balance < 0 THEN ABS(balance) ELSE 0 END) AS total_pending
FROM rent_payments
WHERE payment_date >= '2025-12-01' AND payment_date < '2025-13-01'
GROUP BY DATE(payment_date)
ORDER BY date DESC;
```

---

## Migration History

| Migration | Description | Date |
|-----------|-------------|------|
| 001_create_users.sql | Create users table | 2025-12-10 |
| 002_add_auth_to_users.sql | Add authentication fields | 2025-12-12 |
| 003_seed_admin_user.sql | Create default admin user | 2025-12-12 |
| 004_add_role_to_users.sql | Add role field | 2025-12-12 |
| 005_create_customers.sql | Create customers table | 2025-12-12 |
| 006_create_entries.sql | Create entries table | 2025-12-12 |
| 007_create_entry_events.sql | Create entry events table | 2025-12-13 |
| 008_create_room_entries.sql | Create room entries table | 2025-12-13 |
| 009_create_system_settings.sql | Create system settings table | 2025-12-13 |
| 010_create_rent_payments.sql | Create rent payments table | 2025-12-13 |

---

## Database Size Estimates

**Small Operation (< 1000 entries/month):**
- Total Size: ~50-100 MB
- entries: ~10,000 records
- customers: ~500 records
- rent_payments: ~5,000 records

**Medium Operation (1000-5000 entries/month):**
- Total Size: ~500 MB - 1 GB
- entries: ~50,000 records
- customers: ~2,000 records
- rent_payments: ~25,000 records

**Large Operation (> 5000 entries/month):**
- Total Size: ~2-5 GB+
- entries: ~200,000+ records
- customers: ~10,000+ records
- rent_payments: ~100,000+ records

---

## Backup Recommendations

### Daily Backup
```bash
# Full database dump
pg_dump -U postgres -d cold_db > cold_db_backup_$(date +%Y%m%d).sql

# Compressed backup
pg_dump -U postgres -d cold_db | gzip > cold_db_backup_$(date +%Y%m%d).sql.gz
```

### Table-specific Backup
```bash
# Backup critical tables only
pg_dump -U postgres -d cold_db -t users -t entries -t rent_payments > critical_backup.sql
```

### Restore
```bash
# Restore from backup
psql -U postgres -d cold_db < cold_db_backup_20251213.sql
```

---

**Schema Version:** 1.0.0
**Last Updated:** 2025-12-13
**Maintainer:** Cold Storage Development Team
