# Cold Storage Management System - Complete Architecture Documentation

**Version:** 1.5.173  
**Last Updated:** January 20, 2026  
**Author:** System Architecture Team

---

## Table of Contents

1. [Overview](#overview)
2. [System Architecture](#system-architecture)
3. [Request Flow & Components](#request-flow--components)
4. [Database Schema](#database-schema)
5. [Technology Stack](#technology-stack)
6. [Layer Details](#layer-details)
7. [Security Architecture](#security-architecture)
8. [Deployment Architecture](#deployment-architecture)
9. [Disaster Recovery](#disaster-recovery)
10. [Performance & Scalability](#performance--scalability)

---

## Overview

The Cold Storage Management System is a comprehensive enterprise-grade web application built for managing cold storage facilities. It handles inventory tracking, customer management, gate pass operations, payment processing, and real-time monitoring across a 5-node Kubernetes cluster.

### Key Statistics

- **90+ API Endpoints** across multiple domains
- **42 Handler Types** for different business operations
- **23 Service Layers** containing business logic
- **30+ Database Repositories** for data access
- **30+ PostgreSQL Tables** with complex relationships
- **5-Node K3s Cluster** for high availability
- **2 Database Systems**: PostgreSQL (main) + TimescaleDB (metrics)
- **5 User Roles**: Admin, Employee, Accountant, Guard, Customer

---

## System Architecture

> **📊 Visual Diagram**: See [system-design/01-system-architecture.png](system-design/01-system-architecture.png) for the complete visual architecture diagram.

### High-Level Architecture Diagram

The system follows a modern **layered architecture** pattern with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                              │
│  [Employee Portal] [Customer Portal] [Guard] [Admin] [Infra UI] │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LOAD BALANCER (MetalLB)                      │
│                   VIP: 192.168.15.200                           │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      APPLICATION LAYER                          │
│  ┌──────────────┐  ┌────────────────┐  ┌──────────────────┐   │
│  │ Main App (Go)│  │Customer Portal │  │   External APIs  │   │
│  │  - Router    │  │  - OTP Auth    │  │  - SMS/WhatsApp  │   │
│  │  - Middleware│  │  - Razorpay    │  │  - Payments      │   │
│  │  - Handlers  │  │  - Customer API│  │  - Printers      │   │
│  │  - Services  │  │                │  │                  │   │
│  │  - Repos     │  │                │  │                  │   │
│  └──────────────┘  └────────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    DATA PERSISTENCE LAYER                       │
│  ┌──────────────────────────┐  ┌────────────────────────────┐  │
│  │ PostgreSQL 17 (CloudNPG) │  │  TimescaleDB (Metrics)     │  │
│  │  - 30+ Tables            │  │  - API Logs                │  │
│  │  - 3-Replica HA Cluster  │  │  - Node Metrics            │  │
│  │  - R2 Cloud Backups      │  │  - System Metrics          │  │
│  └──────────────────────────┘  └────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                         │
│               K3s Cluster (5 Nodes)                             │
│  Master: .110 | Workers: .111, .112, .113, .114                │
│  - Longhorn Storage                                             │
│  - Prometheus + Grafana                                         │
│  - CloudNative-PG Operator                                      │
└─────────────────────────────────────────────────────────────────┘
```

### Architecture Highlights

#### 1. Client Layer
- **Employee Portal (8080)**: Main application for employees, accountants, and admins
- **Customer Portal (8081)**: Self-service portal for customers with mobile-first design
- **Guard Dashboard**: Token-based truck registration system
- **Admin Panel**: System administration and infrastructure management
- **Infrastructure UI**: Monitoring dashboards (Grafana, Prometheus)

#### 2. Load Balancer
- **MetalLB**: Provides Virtual IP (VIP) at 192.168.15.200
- **Layer 2 Mode**: ARP-based load balancing
- **High Availability**: Distributes traffic across multiple backend pods

#### 3. Application Layer

**Main Application (Go)**:
- **HTTP Router**: Gorilla Mux with 90+ routes
- **Middleware Stack**:
  - HTTPS Redirect
  - Security Headers (CORS, CSP, HSTS)
  - API Logging to TimescaleDB
  - JWT Authentication
  - Role-Based Authorization (RBAC)
  - Rate Limiting
  - Operation Mode Enforcement (Loading/Unloading)

- **Handler Layer** (42 types):
  - Entry Handler → Entry management
  - Gate Pass Handler → Gate pass operations
  - Customer Handler → Customer CRUD
  - Payment Handler → Rent payments
  - Room Entry Handler → Storage assignments
  - Invoice Handler → Invoice generation
  - User Handler → User management
  - Auth Handler → Authentication
  - Family Member Handler → Family member management
  - Guard Entry Handler → Guard register
  - Token Color Handler → Daily token colors
  - Season Handler → Season change (dual approval)
  - Debt Handler → Debt approval workflow
  - Ledger Handler → Double-entry ledger
  - Report Handler → PDF/CSV exports
  - SMS Handler → SMS/WhatsApp messaging
  - Monitoring Handler → Metrics dashboards
  - Infrastructure Handler → K8s cluster management
  - Node Provisioning Handler → Node management
  - Deployment Handler → Application deployments
  - Razorpay Handler → Payment gateway
  - TOTP Handler → 2FA management
  - Restore Handler → Point-in-time restore
  - Printer Handler → Label/receipt printing
  - ... and 18 more specialized handlers

- **Service Layer** (23 services):
  - Business logic validation
  - State management
  - Event logging
  - Complex calculations
  - Transaction orchestration
  - External API integration

- **Repository Layer** (30 repositories):
  - SQL query building
  - Transaction management
  - Connection pooling
  - Error handling
  - Data mapping

**Customer Portal Server**:
- Separate server on port 8081
- Customer JWT authentication
- OTP/SMS integration for login
- Razorpay payment gateway integration
- Customer-specific APIs (dashboard, gate pass requests, payments)

**External Integrations**:
- SMS/WhatsApp API for notifications
- Razorpay for online payments
- Translation API for Hindi support
- Brother Label Printer integration
- HP Receipt Printer integration

#### 4. Data Persistence Layer

**PostgreSQL 17 (CloudNative-PG)**:
- **High Availability**: 3-replica cluster with automatic failover
- **Tables**: 30+ tables with complex relationships
- **Backup Strategy**:
  - Continuous WAL archiving
  - Daily backups to Cloudflare R2
  - Point-in-time recovery support
- **Key Tables**:
  - `customers` - Customer master data
  - `entries` - Entry/truck registration
  - `room_entries` - Storage location assignments
  - `gate_passes` - Gate pass operations
  - `rent_payments` - Payment transactions
  - `ledger_entries` - Double-entry accounting
  - `users` - Employee/admin accounts
  - `guard_entries` - Guard register entries
  - `debt_approval_requests` - Debt workflow
  - ... and 21 more tables

**TimescaleDB (Metrics)**:
- Time-series database for metrics
- Hypertables for efficient time-based queries
- **Data Types**:
  - API request logs (endpoint, duration, status)
  - Node metrics (CPU, memory, disk)
  - System metrics (connections, queries)
  - Alert history
  - Backup history

#### 5. Infrastructure Layer

**K3s Cluster (5 Nodes)**:
- **Control Plane**: 192.168.15.110
- **Workers**: 192.168.15.111-114
- **Storage**: Longhorn distributed block storage
- **Operators**:
  - CloudNative-PG for PostgreSQL
  - MetalLB for load balancing
- **Monitoring Stack**:
  - Prometheus for metric collection
  - Grafana for visualization
  - Node Exporter for node metrics

#### 6. Backup & Recovery

- **Cloudflare R2 Backups**: Automated daily backups to object storage
- **Auto-Failover**: Automatic failover to backup DB (192.168.15.195:5434)
- **Setup Mode**: Disaster recovery wizard when DB is unavailable
- **Point-in-Time Restore**: Restore to any point in last 30 days

---

## Request Flow & Components

> **📊 Visual Diagram**: See [system-design/02-component-flow.png](system-design/02-component-flow.png) for the complete request flow diagram.

### Standard Request Flow

```
User → HTTP Request → Middleware Pipeline → Handler → Service → Repository → Database
                                                                               ↓
User ← HTTP Response ← Middleware ← Handler ← Service ← Repository ← Database Response
```

### Middleware Pipeline (Sequential)

1. **HTTPS Redirect**
   - Forces all HTTP requests to HTTPS
   - Ensures secure communication

2. **Security Headers**
   - Sets CORS policies
   - Content Security Policy (CSP)
   - HTTP Strict Transport Security (HSTS)
   - X-Frame-Options, X-Content-Type-Options

3. **API Logging** (Optional)
   - Logs request details to TimescaleDB
   - Captures: endpoint, method, duration, status, user ID
   - Used for analytics and monitoring

4. **Authentication**
   - Validates JWT token from Authorization header
   - Extracts user information
   - Sets user context for request
   - **Bypass**: Public endpoints (login, health checks)

5. **Authorization (RBAC)**
   - Checks user role and permissions
   - Validates access rights for the endpoint
   - **Roles**: admin, employee, accountant, guard, customer
   - **Special Checks**:
     - Operation mode enforcement (loading/unloading)
     - Dual admin approval for sensitive operations
     - Accountant access for financial operations

**Authorization Results**:
- ✅ **Authorized**: Request proceeds to handler
- ❌ **Unauthorized**: Returns 401 (not authenticated) or 403 (forbidden)

### Handler → Service → Repository Pattern

**Handler Responsibilities**:
- Parse HTTP request (body, params, query)
- Validate input data
- Call appropriate service method
- Format HTTP response
- Handle errors gracefully

**Service Responsibilities**:
- Business logic validation
- Complex calculations
- State management
- Event logging
- Transaction orchestration
- External API calls

**Repository Responsibilities**:
- SQL query building
- Database transactions
- Connection management
- Error handling
- Data mapping (DB ↔ Go structs)

### Special Business Flows

#### Gate Pass Flow
```
Create Gate Pass
    ↓
Validate Inventory (check available stock)
    ↓
Create gate_passes record (status: pending)
    ↓
Log GATE_PASS_ISSUED event
    ↓
[Employee Approves]
    ↓
Validate inventory again (approval check)
    ↓
Update gate_passes.status = approved
    ↓
Set approval_expires_at (15 hours)
    ↓
Log GATE_PASS_APPROVED event
    ↓
[Record Pickups]
    ↓
Create gate_pass_pickups record
    ↓
Update gate_passes.total_picked_up
    ↓
Update status to partially_completed/completed
    ↓
Log ITEMS_OUT event
    ↓
[Optional] Send SMS notification
```

#### Payment Flow
```
Create Payment
    ↓
Validate customer and entry
    ↓
Calculate rent amount
    ↓
[If Online] Create Razorpay order
    ↓
Customer completes payment
    ↓
Razorpay webhook → Verify signature
    ↓
Create rent_payments record
    ↓
Create ledger_entries (debit customer, credit income)
    ↓
Update customer balance
    ↓
Generate receipt number
    ↓
Create PDF receipt
    ↓
[Optional] Send SMS/WhatsApp receipt
```

#### Entry Creation Flow
```
Create Entry
    ↓
Validate customer
    ↓
Generate thock number (format: XXXX/QQ)
    ↓
Create entries record
    ↓
Log ENTRY_CREATED event
    ↓
[Assign to Storage]
    ↓
Create room_entries record (room, floor, gatar)
    ↓
Update inventory
    ↓
Log ROOM_ASSIGNED event
    ↓
Calculate rent (based on quantity and days)
    ↓
Generate loading invoice PDF
    ↓
[Optional] Send SMS notification
```

---

## Database Schema

> **📊 Visual Diagram**: See [system-design/03-database-schema.png](system-design/03-database-schema.png) for the complete entity-relationship diagram.

### Entity Relationship Overview

The database contains **30+ tables** organized into the following functional groups:

### Core Business Entities

#### 1. Customer Management
- **`customers`**: Master customer data
  - Primary Key: `id`
  - Unique: `phone`
  - Fields: name, village, father_name, paid_status
  - Relationships: 1:M with entries, gate_passes, payments, ledger_entries

- **`family_members`**: Customer family members
  - Foreign Key: `customer_id → customers.id`
  - Fields: name, relation, phone
  - Purpose: Delegated access for gate pass requests

#### 2. Inventory Management
- **`entries`**: Entry/truck registration
  - Primary Key: `id`
  - Unique: `thock_number` (format: XXXX/QQ)
  - Foreign Keys: `customer_id`, `family_member_id`
  - Fields: expected_quantity, commodity, entry_type (seed/sell), is_deleted
  - Soft Delete: `is_deleted` flag instead of hard delete

- **`room_entries`**: Storage location assignments
  - Primary Key: `id`
  - Foreign Key: `entry_id → entries.id`
  - Fields: room_no, floor, gate_no (gatar), quantity, quantity_breakdown
  - Purpose: Track physical storage locations

- **`entry_events`**: Event tracking for entries
  - Foreign Key: `entry_id → entries.id`
  - Fields: event_type (ENTRY_CREATED, ROOM_ASSIGNED, ITEMS_OUT, etc.), status, notes
  - Purpose: Complete audit trail of entry lifecycle

#### 3. Gate Pass System
- **`gate_passes`**: Gate pass operations
  - Primary Keys: `id`
  - Foreign Keys: `customer_id`, `entry_id`, `family_member_id`
  - Fields:
    - `requested_quantity` - Items requested
    - `approved_quantity` - Items approved by employee
    - `total_picked_up` - Items actually picked up
    - `status` - pending, approved, completed, partially_completed, expired, rejected
    - `expires_at` - 30-hour approval deadline
    - `approval_expires_at` - 15-hour pickup deadline
  - Workflow: Created → Approved → Partial Pickup → Complete

- **`gate_pass_pickups`**: Pickup records
  - Foreign Key: `gate_pass_id → gate_passes.id`
  - Fields: pickup_quantity, room_no, floor, gatar_breakdown
  - Purpose: Track partial pickups and inventory reduction

#### 4. Payment & Accounting
- **`rent_payments`**: Payment transactions
  - Foreign Keys: `customer_id`, `entry_id`, `created_by_user_id`
  - Unique: `receipt_number`
  - Fields: amount, payment_method, remarks
  - Purpose: Track rent payments

- **`ledger_entries`**: Double-entry accounting system
  - Foreign Key: `customer_id`
  - Fields: entry_type (debit/credit), amount, description, balance, related_type, related_id
  - Purpose: Complete financial audit trail

- **`razorpay_transactions`**: Online payment tracking
  - Foreign Key: `customer_id`
  - Fields: order_id, payment_id, amount, status
  - Purpose: Reconcile Razorpay payments

- **`debt_approval_requests`**: Debt approval workflow
  - Foreign Keys: `customer_id`, `requested_by_user_id`, `approved_by_user_id`
  - Fields: amount, reason, status (pending/approved/rejected/used)
  - Purpose: Admin approval for debt/credit

#### 5. User Management & Authentication
- **`users`**: Employee/admin accounts
  - Unique: `email`
  - Fields: password_hash, name, role, permissions (JSON), totp_secret, totp_enabled, is_active
  - Roles: admin, employee, accountant, guard
  - Purpose: System user management

- **`login_logs`**: User login tracking
  - Foreign Key: `user_id`
  - Fields: ip_address, user_agent, success, login_at, logout_at
  - Purpose: Security audit trail

- **`customer_login_logs`**: Customer portal login tracking
  - Foreign Key: `customer_id`
  - Fields: login_method (otp/simple), ip_address, success
  - Purpose: Customer activity monitoring

#### 6. Guard System
- **`guard_entries`**: Guard register entries
  - Fields: token_number, customer_name, phone, vehicle_number, entry_type, portion_seed, portion_sell
  - Status: pending, processed, deleted
  - Purpose: Token-based truck registration by guards

- **`token_colors`**: Daily token color assignments
  - Unique: `date`
  - Fields: color, notes
  - Purpose: Color-coded token tracking

#### 7. Audit & Logging
- **`entry_edit_logs`**: Entry modification tracking
  - Foreign Keys: `entry_id`, `edited_by_user_id`
  - Fields: field_name, old_value, new_value
  - Purpose: Track all entry edits

- **`room_entry_edit_logs`**: Room entry modification tracking
  - Foreign Keys: `room_entry_id`, `edited_by_user_id`
  - Fields: field_name, old_value, new_value

- **`admin_action_logs`**: Admin action tracking
  - Foreign Key: `user_id`
  - Fields: action, target_type, target_id, details, ip_address
  - Purpose: Track sensitive admin operations

- **`entry_management_logs`**: Entry reassignment/merge tracking
  - Fields: action_type (reassign/merge), from_customer_id, to_customer_id, entry_ids, performed_by
  - Purpose: Track customer merges and entry transfers

- **`customer_activity_logs`**: Customer portal activity
  - Foreign Key: `customer_id`
  - Fields: activity_type, activity_data, ip_address
  - Purpose: Monitor customer portal usage

- **`sms_logs`**: SMS/WhatsApp delivery tracking
  - Foreign Key: `customer_id`
  - Fields: phone, message_type, message, provider_response, status
  - Purpose: Track communication history

#### 8. Advanced Features
- **`season_change_requests`**: End-of-season archival
  - Foreign Keys: `requested_by`, `approved_by`
  - Fields: season_name, reason, status (pending/approved/rejected/completed)
  - Purpose: Dual admin approval for season closure

- **`pending_setting_changes`**: Protected setting changes
  - Foreign Keys: `requested_by`, `approved_by`
  - Fields: setting_key, old_value, new_value, reason, status
  - Purpose: Dual admin approval for sensitive settings

- **`customer_merge_history`**: Customer merge tracking
  - Fields: from_customer_id, to_customer_id, merged_data, can_undo
  - Purpose: Track and potentially undo customer merges

- **`system_settings`**: Dynamic configuration
  - Unique: `key`
  - Fields: value, description, updated_by
  - Purpose: Runtime configuration

- **`invoices`**: Loading invoice tracking
  - Foreign Keys: `entry_id`, `customer_id`, `created_by_user_id`
  - Unique: `invoice_number`
  - Fields: commodity, quantity, rate, amount
  - Purpose: Invoice generation

### Database Relationships

**Key Relationships**:
- customers → entries (1:M)
- customers → gate_passes (1:M)
- customers → rent_payments (1:M)
- customers → ledger_entries (1:M)
- customers → family_members (1:M)
- entries → room_entries (1:M)
- entries → gate_passes (1:M)
- entries → entry_events (1:M)
- gate_passes → gate_pass_pickups (1:M)
- users → *_logs tables (1:M)

### Indexing Strategy

**Primary Indexes**:
- All primary keys (id columns)
- Unique constraints (phone, email, receipt_number, thock_number)

**Foreign Key Indexes**:
- All foreign key columns for join performance

**Custom Indexes**:
- `customers.phone` - Fast customer lookup
- `entries.thock_number` - Fast entry lookup
- `entries.customer_id, is_deleted` - Customer entries with soft delete filter
- `gate_passes.status, expires_at` - Expiration processing
- `ledger_entries.customer_id, created_at` - Ledger queries
- `login_logs.user_id, login_at` - Login history

---

## Technology Stack

### Backend
- **Language**: Go 1.23
- **HTTP Framework**: Gorilla Mux (routing)
- **Database Driver**: pgx/v5 (PostgreSQL driver with connection pooling)
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **2FA**: TOTP (pquerna/otp)
- **Password Hashing**: bcrypt
- **Environment**: godotenv for .env files

### Frontend
- **HTML5**: Semantic markup
- **CSS**: Tailwind CSS 3.x
- **Icons**: Bootstrap Icons
- **JavaScript**: Vanilla JS (no framework)
- **i18n**: Custom translation system with locales (en.json, hi.json)
- **WebFonts**: Inter, Roboto

### Databases
- **PostgreSQL 17**: Main application database
- **TimescaleDB**: Time-series metrics database
- **CloudNative-PG**: Kubernetes operator for PostgreSQL HA

### Infrastructure
- **Container Orchestration**: K3s (lightweight Kubernetes)
- **Container Runtime**: containerd
- **Storage**: Longhorn (distributed block storage)
- **Load Balancer**: MetalLB (Layer 2)
- **Monitoring**: Prometheus + Grafana
- **Metrics Export**: Node Exporter, PostgreSQL Exporter

### External Services
- **Payment Gateway**: Razorpay
- **SMS Provider**: Custom SMS API integration
- **WhatsApp**: WhatsApp Business API
- **Cloud Storage**: Cloudflare R2 (S3-compatible)
- **Printers**: Brother QL series (label), HP (receipt)

### Development Tools
- **Version Control**: Git
- **Container Build**: Docker
- **Deployment**: kubectl, custom scripts
- **Database Migrations**: SQL files

---

## Layer Details

### 1. Presentation Layer (Templates)

**Location**: `templates/` directory

**58 HTML Templates**:
- Dashboard pages (employee, admin, accountant, guard, customer)
- Entry management (main entry, entry room, room configuration)
- Gate pass pages (gate pass entry, unloading tickets)
- Payment pages (rent, payment receipt, verify receipt)
- Admin pages (employees, system settings, logs, reports)
- Customer portal pages (login, dashboard)
- Guard pages (guard dashboard, guard register)
- Infrastructure pages (monitoring, node provisioning)
- Special pages (login, portfolio, setup, restore)

**Template Features**:
- Server-side rendering with Go html/template
- Embedded static assets
- Multi-language support (data-i18n attributes)
- Responsive design (Tailwind CSS)
- Client-side authentication check (localStorage)

### 2. HTTP Layer (Router & Middleware)

**Location**: `internal/http/router.go`

**Router Configuration**:
- Main app router (port 8080) - 750+ lines
- Customer portal router (port 8081) - 130+ lines
- Route groups by domain (API, pages, public)

**Middleware Stack**:
1. **HTTPSRedirect**: Force HTTPS in production
2. **SecurityHeaders**: Set CORS, CSP, HSTS headers
3. **APILoggingMiddleware**: Log to TimescaleDB (optional)
4. **AuthMiddleware**: JWT validation
5. **OperationModeMiddleware**: Enforce loading/unloading mode
6. **RateLimiter**: Limit login attempts

**Route Protection**:
- Public routes: /, /login, /health, /auth/*
- Authenticated routes: /api/* (requires JWT)
- Role-protected routes: admin, employee, accountant, guard
- Operation mode routes: loading-only, unloading-only

### 3. Handler Layer

**Location**: `internal/handlers/`

**42 Handler Types** (selected):

**Core Business**:
- `entry_handler.go` - Entry CRUD, bulk operations
- `room_entry_handler.go` - Room assignment
- `customer_handler.go` - Customer management, merge
- `gate_pass_handler.go` - Gate pass operations, pickup
- `rent_payment_handler.go` - Payment processing
- `invoice_handler.go` - Invoice generation

**User Management**:
- `auth_handler.go` - Login, signup, 2FA
- `user_handler.go` - User CRUD
- `family_member_handler.go` - Family member management

**Guard System**:
- `guard_entry_handler.go` - Guard register
- `token_color_handler.go` - Daily token colors

**Accounting**:
- `ledger_handler.go` - Ledger queries
- `debt_handler.go` - Debt approval workflow
- `account_handler.go` - Account summary

**Administration**:
- `system_setting_handler.go` - System settings
- `season_handler.go` - Season management (dual approval)
- `pending_setting_handler.go` - Protected settings (dual approval)
- `admin_action_log_handler.go` - Admin action logs

**Reports & Analytics**:
- `report_handler.go` - PDF/CSV exports
- `monitoring_handler.go` - Metrics dashboards
- `items_in_stock_handler.go` - Stock visualization
- `room_visualization_handler.go` - Room occupancy

**Infrastructure**:
- `infrastructure_handler.go` - K8s cluster status
- `node_provisioning_handler.go` - Node management
- `deployment_handler.go` - Application deployments
- `health_handler.go` - Health checks

**Communication**:
- `sms_handler.go` - SMS/WhatsApp bulk messaging
- `razorpay_handler.go` - Payment gateway

**Utilities**:
- `printer_handler.go` - Label/receipt printing
- `restore_handler.go` - Point-in-time restore
- `page_handler.go` - Template rendering
- `customer_portal_handler.go` - Customer portal APIs

**Logging**:
- `login_log_handler.go` - Login tracking
- `entry_edit_log_handler.go` - Entry edits
- `room_entry_edit_log_handler.go` - Room entry edits
- `entry_management_log_handler.go` - Reassignments/merges
- `customer_activity_log_handler.go` - Customer portal activity
- `merge_history_handler.go` - Customer merge history

Handler responsibilities:
- Parse and validate HTTP request
- Call appropriate service method
- Format HTTP response (JSON/HTML)
- Handle errors and return appropriate status codes

### 4. Service Layer

**Location**: `internal/services/`

**23 Service Types** containing business logic:

- `entry_service.go` - Entry business logic
- `customer_service.go` - Customer operations
- `gate_pass_service.go` - Gate pass workflow (fixed inventory bug!)
- `room_entry_service.go` - Storage assignment logic
- `rent_payment_service.go` - Payment calculations
- `ledger_service.go` - Double-entry accounting
- `invoice_service.go` - Invoice generation
- `auth_service.go` - Authentication logic
- `user_service.go` - User management
- `season_service.go` - Season archival logic
- `guard_entry_service.go` - Guard register logic
- `debt_service.go` - Debt approval workflow
- `sms_service.go` - SMS/WhatsApp messaging
- `razorpay_service.go` - Payment gateway integration
- `deployment_service.go` - Application deployment logic
- `monitoring_service.go` - Metrics collection
- `report_service.go` - Report generation
- ... and 6 more

Service responsibilities:
- Business logic validation
- Complex calculations (rent, inventory)
- State management
- Event logging
- Transaction orchestration
- External API integration

### 5. Repository Layer

**Location**: `internal/repositories/`

**30 Repository Types** for data access:

- `entry_repository.go` - Entry CRUD
- `customer_repository.go` - Customer CRUD
- `gate_pass_repository.go` - Gate pass CRUD (fixed queries!)
- `room_entry_repository.go` - Room entry CRUD
- `rent_payment_repository.go` - Payment CRUD
- `ledger_repository.go` - Ledger CRUD
- `user_repository.go` - User CRUD
- `invoice_repository.go` - Invoice CRUD
- `entry_event_repository.go` - Event logging
- `family_member_repository.go` - Family member CRUD
- `guard_entry_repository.go` - Guard entry CRUD
- `season_repository.go` - Season CRUD
- `debt_repository.go` - Debt request CRUD
- `sms_repository.go` - SMS log CRUD
- `razorpay_repository.go` - Transaction CRUD
- `monitoring_repository.go` - Metrics queries
- ... and 14 more

Repository responsibilities:
- SQL query building
- Database transactions
- Connection pooling (pgx.Pool)
- Error handling
- Data mapping (DB rows ↔ Go structs)

### 6. Model Layer

**Location**: `internal/models/`

**30 Model Types** defining data structures:

- `customer.go` - Customer struct
- `entry.go` - Entry struct
- `gate_pass.go` - GatePass struct
- `room_entry.go` - RoomEntry struct
- `rent_payment.go` - RentPayment struct
- `user.go` - User struct
- `ledger_entry.go` - LedgerEntry struct
- `invoice.go` - Invoice struct
- ... and 22 more

Models include:
- Struct definitions with JSON tags
- Request/response DTOs
- Validation rules

---

## Security Architecture

### Authentication

**Employee/Admin Authentication**:
1. User submits email + password
2. Server validates credentials (bcrypt)
3. If 2FA enabled: require TOTP code
4. Generate JWT token (HS256)
5. Return token to client
6. Client stores in localStorage
7. Client sends token in Authorization header for all requests

**Customer Authentication**:
1. **OTP Method**:
   - Customer submits phone number
   - Server generates 6-digit OTP
   - Send OTP via SMS
   - Customer submits OTP for verification
   - Generate customer JWT token
   
2. **Simple Method** (if enabled):
   - Customer submits phone + simple password
   - Server validates
   - Generate customer JWT token

**JWT Structure**:
```json
{
  "user_id": 123,
  "email": "user@example.com",
  "role": "employee",
  "permissions": ["can_manage_entries"],
  "exp": 1640000000
}
```

**Token Expiry**:
- Employee/Admin: 24 hours
- Customer: 7 days

### Authorization (RBAC)

**Role Hierarchy**:
1. **Admin**: Full access to all features
2. **Employee**: Entry creation, gate pass operations, debt requests
3. **Accountant**: Payment processing, reports, ledger access
4. **Guard**: Guard register, token management
5. **Customer**: Customer portal self-service

**Permission Checks**:
- Route-level: `RequireRole("admin", "employee")`
- Feature-level: `RequireAccountantAccess`, `RequireAdmin`
- Operation mode: `RequireLoadingMode`, `RequireUnloadingMode`

**Special Authorization**:
- **Dual Admin Approval**: Season changes, protected settings
- **Permission JSON**: Custom permissions per user (can_manage_entries, accountant_access)

### Security Features

**1. Two-Factor Authentication (2FA)**
- TOTP-based (Time-based One-Time Password)
- QR code setup for authenticator apps
- Backup codes for account recovery
- Admin-only feature

**2. Rate Limiting**
- Login attempts: 5 per 15 minutes per IP
- API calls: Configurable per endpoint

**3. Password Security**
- bcrypt hashing (cost factor 10)
- Minimum 6 characters
- Password change requires old password

**4. Session Management**
- JWT-based (stateless)
- Logout logs timestamp in login_logs
- No server-side session storage

**5. Audit Logging**
- All logins (success/failure) logged
- Admin actions logged with details
- Customer activity tracked
- Entry edits tracked with old/new values

**6. Security Headers**
- Content-Security-Policy (CSP)
- X-Frame-Options: DENY
- X-Content-Type-Options: nosniff
- HTTP Strict Transport Security (HSTS)
- CORS configuration

**7. Input Validation**
- SQL injection prevention (prepared statements via pgx)
- XSS prevention (HTML escaping in templates)
- CSRF protection (JWT token validation)

---

## Deployment Architecture

### Production Environment

**Network Topology**:
```
Internet
    │
    ▼
[Local Network: 192.168.15.0/24]
    │
    ├─ VIP: 192.168.15.200 (MetalLB)
    │    │
    │    └─ Load balances to backend pods
    │
    ├─ K3s Master: 192.168.15.110
    │    └─ Control Plane, etcd, API Server
    │
    ├─ K3s Worker 1: 192.168.15.111
    │    └─ Backend pods, PostgreSQL replica 1
    │
    ├─ K3s Worker 2: 192.168.15.112
    │    └─ Backend pods, PostgreSQL replica 2
    │
    ├─ K3s Worker 3: 192.168.15.113
    │    └─ Backend pods, PostgreSQL replica 3
    │
    ├─ K3s Worker 4: 192.168.15.114
    │    └─ Backend pods, Monitoring stack
    │
    └─ Backup DB: 192.168.15.195:5434
         └─ Standalone PostgreSQL (fallback)
```

### Kubernetes Resources

**Namespaces**:
- `default` - Application pods
- `postgresql` - CloudNative-PG cluster
- `monitoring` - Prometheus, Grafana
- `longhorn-system` - Storage system

**Deployments**:
- `cold-backend` - Main application (3 replicas)
- `cold-customer-portal` - Customer portal (2 replicas)

**StatefulSets**:
- `cold-backend-postgresql` - PostgreSQL cluster (3 replicas)
- `prometheus` - Metrics database
- `timescaledb` - Time-series database

**Services**:
- `cold-backend-service` - LoadBalancer (MetalLB)
- `cold-backend-postgresql-rw` - PostgreSQL read-write service
- `cold-backend-postgresql-ro` - PostgreSQL read-only service
- `prometheus` - ClusterIP
- `grafana` - ClusterIP

**Storage**:
- `longhorn` - StorageClass for persistent volumes
- PVCs for PostgreSQL data, Prometheus data, Grafana data

### High Availability

**Application Layer**:
- **3 backend replicas** across worker nodes
- Load balanced via MetalLB
- Rolling updates (max unavailable: 1)
- Health checks (readiness, liveness probes)

**Database Layer**:
- **3 PostgreSQL replicas** (1 primary, 2 replicas)
- Synchronous replication
- Automatic failover (< 30 seconds)
- Read-only replicas for reporting
- WAL archiving to Cloudflare R2

**Storage Layer**:
- **Longhorn**: 3 replicas per volume
- Distributed block storage
- Snapshot support
- Cross-node replication

### Deployment Process

**CI/CD Pipeline**:
1. Build Docker image
2. Tag with version number (v1.5.173)
3. Push to container registry (optional)
4. Update Kubernetes manifest with new image
5. Apply manifest: `kubectl apply -f k8s/`
6. Rolling update (zero-downtime)
7. Health check validation
8. Rollback if health checks fail

**Manual Deployment**:
```bash
# Build binary
go build -o server ./cmd/server/

# Build Docker image
docker build -t cold-backend:v1.5.173 .

# Deploy to K3s
kubectl apply -f k8s/deployment.yaml
kubectl rollout status deployment/cold-backend

# Verify
kubectl get pods
kubectl logs -f deployment/cold-backend
```

### Monitoring & Observability

**Prometheus Metrics**:
- Node metrics (CPU, memory, disk, network)
- PostgreSQL metrics (connections, queries, replication lag)
- Application metrics (request count, duration, errors)
- Custom business metrics (entries created, payments processed)

**Grafana Dashboards**:
- Cluster overview
- Node resource usage
- PostgreSQL performance
- Application metrics
- Alert history

**Logging**:
- Application logs: stdout (captured by kubectl logs)
- API logs: TimescaleDB (api_logs table)
- Audit logs: PostgreSQL (various *_logs tables)

**Alerts**:
- High CPU/memory usage
- PostgreSQL replication lag
- Disk space low
- Pod restarts
- API error rate spike

---

## Disaster Recovery

### Backup Strategy

**PostgreSQL Backups**:
1. **Continuous WAL Archiving**:
   - WAL segments archived to Cloudflare R2
   - Enables point-in-time recovery
   - Retention: 30 days

2. **Daily Full Backups**:
   - Scheduled at 2:00 AM
   - Uploaded to Cloudflare R2
   - Retention: 7 daily, 4 weekly, 12 monthly

3. **On-Demand Backups**:
   - Triggered via `/api/infrastructure/trigger-backup`
   - Admin-only access

**Backup Verification**:
- Daily backup health checks
- Restore testing monthly
- Backup size monitoring

### Database Failover

**Automatic Failover Sequence**:
1. Application starts
2. Try to connect to Primary DB (192.168.15.200:5432)
3. **If Primary fails**:
   - Try Backup DB (192.168.15.195:5434)
4. **If both fail**:
   - Enter Setup Mode
   - Show setup wizard at `/setup`

**CloudNative-PG Failover**:
- Automatic within PostgreSQL cluster
- Promoted replica becomes new primary
- < 30 seconds downtime
- Application reconnects automatically (pgx connection pool retry)

### Point-in-Time Recovery

**Use Cases**:
- Accidental data deletion
- Corrupted data rollback
- Testing/staging environment creation

**Recovery Process**:
1. Admin accesses `/admin/restore`
2. Selects target date/time
3. System finds closest WAL backup
4. Preview restore (shows what will change)
5. Confirm restore
6. System restores to new database
7. Admin validates data
8. Switch application to restored database

**Limitations**:
- Maximum 30 days back
- Requires WAL archives availability
- Downtime during restore (5-30 minutes)

### Disaster Recovery Scenarios

**Scenario 1: Single Node Failure**
- **Impact**: Pods on failed node restart on other nodes
- **Recovery Time**: < 5 minutes (automatic)
- **Data Loss**: None (Longhorn replication)

**Scenario 2: Database Cluster Failure**
- **Impact**: All PostgreSQL replicas down
- **Recovery**: Automatic failover to Backup DB (192.168.15.195:5434)
- **Recovery Time**: < 1 minute
- **Data Loss**: Up to last checkpoint (usually < 1 minute)

**Scenario 3: Complete Cluster Failure**
- **Impact**: All K3s nodes down
- **Recovery**:
  1. Extract recovery package at `/home/lakshya/backups/cold-backend/`
  2. Run standalone binary with `.env` pointing to Backup DB
  3. Restore from R2 backup if needed
- **Recovery Time**: 10-30 minutes
- **Data Loss**: Up to last R2 backup (max 24 hours)

**Scenario 4: Data Corruption**
- **Impact**: Corrupted data in database
- **Recovery**: Point-in-time restore to before corruption
- **Recovery Time**: 30-60 minutes
- **Data Loss**: Data entered after restore point

### Recovery Package

**Location**: `/home/lakshya/backups/cold-backend/`

**Contents**:
- `server` - Linux binary (31 MB)
- `templates/` - 58 HTML templates
- `static/` - CSS, JS, fonts, locales
- `.env.example` - Environment template
- `RECOVERY.md` - Recovery instructions

**Quick Recovery**:
```bash
# Extract and run
cd /home/lakshya/backups/cold-backend/
./server

# Or with custom DB
DB_HOST=192.168.15.195 DB_PORT=5434 ./server
```

---

## Performance & Scalability

### Current Performance

**Response Times** (95th percentile):
- API calls: < 100ms
- Database queries: < 50ms
- Page loads: < 500ms
- PDF generation: < 2s

**Throughput**:
- API requests: 1000+ req/s
- Concurrent users: 100+
- Database connections: 50 (pooled)

### Scalability Strategy

**Horizontal Scaling**:
- Backend pods: Scale to 5-10 replicas
- PostgreSQL: Add read replicas for reporting
- Strategy: Add more worker nodes

**Vertical Scaling**:
- Worker nodes: Upgrade RAM/CPU
- PostgreSQL: Increase shared_buffers, work_mem

**Database Optimization**:
- Connection pooling (pgx.Pool)
- Prepared statements
- Indexes on foreign keys
- Partitioning for large tables (future)

**Caching Strategy** (future):
- Redis for session cache
- Application-level caching
- CDN for static assets

### Performance Optimizations

**Database**:
- Indexes on all foreign keys
- Composite indexes for common queries
- Connection pooling (50 connections)
- Read replicas for reporting queries

**Application**:
- Embedded static assets (no file I/O)
- Gzip compression for API responses
- Efficient JSON marshaling
- Minimal memory allocations

**Frontend**:
- Minified CSS/JS
- Web fonts preloaded
- Lazy loading for images
- Client-side caching (localStorage)

### Capacity Planning

**Current Capacity**:
- 30,000+ entries
- 10,000+ customers
- 50,000+ gate passes
- 100,000+ transactions

**Growth Projections** (next 3 years):
- Entries: 100,000
- Customers: 30,000
- Gate passes: 200,000
- Transactions: 500,000

**Scaling Plan**:
- Year 1: Current setup sufficient
- Year 2: Add 2 more worker nodes, read replicas
- Year 3: Implement table partitioning, add caching layer

---

## Conclusion

The Cold Storage Management System is a robust, enterprise-grade application built with modern technologies and best practices. It successfully handles complex business workflows, ensures data integrity, provides high availability, and scales to meet growing demands.

**Key Achievements**:
- ✅ Zero-downtime deployments
- ✅ < 30s automatic failover
- ✅ Complete audit trail
- ✅ Multi-language support
- ✅ Comprehensive monitoring
- ✅ Disaster recovery tested

**Future Enhancements**:
- Mobile app (iOS/Android)
- Advanced analytics dashboard
- Machine learning for demand forecasting
- Blockchain integration for supply chain transparency
- Multi-facility support

---

**Document Version**: 1.0  
**Last Updated**: January 20, 2026
**Maintained By**: System Architecture Team
