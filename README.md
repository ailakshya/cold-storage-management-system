# Cold Storage Management System

A comprehensive web-based management system for cold storage facilities with role-based access control, inventory tracking, payment management, and reporting features. Deployed on a highly available Kubernetes cluster with automatic failover and self-healing capabilities.

![Version](https://img.shields.io/badge/version-2.0.0-blue)
![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-316192?logo=postgresql)
![Kubernetes](https://img.shields.io/badge/K3s-1.33-326CE5?logo=kubernetes)
![License](https://img.shields.io/badge/license-Proprietary-red)

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [System Architecture](#system-architecture)
- [Application Architecture](#application-architecture)
- [Technology Stack](#technology-stack)
- [Infrastructure Components](#infrastructure-components)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Database Setup](#database-setup)
- [User Roles & Permissions](#user-roles--permissions)
- [API Documentation](#api-documentation)
- [Frontend Pages](#frontend-pages)
- [Database Schema](#database-schema)
- [Development Guide](#development-guide)
- [Testing](#testing)
- [Production Deployment](#production-deployment)
- [Auto-Recovery System](#auto-recovery-system)
- [Troubleshooting](#troubleshooting)

---

## Overview

The Cold Storage Management System is a full-stack web application designed to manage the complete lifecycle of goods stored in cold storage facilities. It provides:

- **Multi-role access control** - Separate portals for employees, accountants, and administrators
- **Entry management** - Track incoming goods with automatic truck categorization
- **Room allocation** - Assign storage locations to entries
- **Payment processing** - Handle rent payments with cumulative balance tracking
- **Reporting & analytics** - Generate invoices, receipts, and reports
- **Real-time search** - Quick lookup of customers and entries

### Key Business Logic

- **Users** = Employees who operate the portal (authenticate with email/password)
- **Customers** = People who store goods in cold storage (searched by phone)
- **Entries** = Records of customer storage entries created by employees
- **Truck Categories** = Automatic assignment based on quantity (1-600: seed, 601-1500: sell)
- **Room Entries** = Physical storage location assignments with floor, gate, and room details
- **Payments** = Rent payment records with cumulative balance calculation

---

## Features

### Core Functionality

✅ **User Management**
- Role-based authentication (Employee, Accountant, Admin)
- JWT-based secure sessions
- Password hashing with bcrypt

✅ **Entry Management**
- Create storage entries with customer information
- Automatic truck number generation (SEED-001, SELL-001, etc.)
- Automatic category assignment based on quantity
- Entry tracking and history

✅ **Room Management**
- Assign entries to physical storage locations
- Track room number, floor, gate number
- Update and edit room assignments
- Quantity tracking per room

✅ **Payment Processing**
- Rent payment collection
- Cumulative balance calculation
- Payment history per customer/entry
- Print-friendly payment receipts

✅ **Search & Reporting**
- Item search by customer phone, truck number, room number
- Customer search by phone
- Entry filtering and sorting
- Loading invoice generation
- Payment receipt printing

✅ **System Settings**
- Configurable rent rates
- System-wide settings management
- Admin-controlled configurations

---

## System Architecture

### High Availability Infrastructure

```
                                    ┌─────────────────────────────────────────────────────────────┐
                                    │                    PRODUCTION CLUSTER                        │
                                    └─────────────────────────────────────────────────────────────┘

                                              ┌─────────────────────┐
                                              │   Virtual IP (VIP)  │
                                              │   192.168.15.200    │
                                              │      MetalLB        │
                                              └──────────┬──────────┘
                                                         │
                    ┌────────────────────────────────────┼────────────────────────────────────┐
                    │                                    │                                    │
           ┌────────▼────────┐                 ┌────────▼────────┐                 ┌────────▼────────┐
           │   K3s Node 1    │                 │   K3s Node 2    │                 │   K3s Node 3    │
           │  Control Plane  │                 │  Control Plane  │                 │  Control Plane  │
           │ 192.168.15.110  │                 │ 192.168.15.111  │                 │ 192.168.15.112  │
           └────────┬────────┘                 └────────┬────────┘                 └────────┬────────┘
                    │                                    │                                    │
           ┌────────▼────────┐                 ┌────────▼────────┐                           │
           │   K3s Node 4    │                 │   K3s Node 5    │                           │
           │     Worker      │                 │     Worker      │                           │
           │ 192.168.15.113  │                 │ 192.168.15.114  │                           │
           └─────────────────┘                 └─────────────────┘                           │
                                                                                             │
┌───────────────────────────────────────────────────────────────────────────────────────────┼───────┐
│                                     STORAGE LAYER                                          │       │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐  │       │
│  │                           Longhorn Distributed Storage                               │  │       │
│  │   - 3 replicas per volume for data redundancy                                       │  │       │
│  │   - Auto-healing and rebalancing                                                    │  │       │
│  │   - Snapshot and backup support                                                     │  │       │
│  └─────────────────────────────────────────────────────────────────────────────────────┘  │       │
└───────────────────────────────────────────────────────────────────────────────────────────┴───────┘

┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                     DATABASE LAYER                                                 │
│  ┌──────────────────────┐    ┌──────────────────────┐    ┌──────────────────────┐                │
│  │   PostgreSQL Pod 1   │    │   PostgreSQL Pod 2   │    │   PostgreSQL Pod 3   │                │
│  │      (Primary)       │◄──►│     (Replica)        │◄──►│     (Replica)        │                │
│  │   Streaming Repl.    │    │   Streaming Repl.    │    │   Streaming Repl.    │                │
│  └──────────────────────┘    └──────────────────────┘    └──────────────────────┘                │
│                              CloudNative-PG Operator (Auto-failover)                              │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   APPLICATION LAYER                                               │
│  ┌─────────────────────────────────────┐    ┌─────────────────────────────────────┐              │
│  │     Employee API Deployment         │    │     Customer API Deployment         │              │
│  │        (2-5 replicas)               │    │        (2-5 replicas)               │              │
│  │        Port: 8080                   │    │        Port: 8081                   │              │
│  └─────────────────────────────────────┘    └─────────────────────────────────────┘              │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────────────────────────────┐
│                                   BACKUP & MONITORING                                             │
│  ┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────────┐                   │
│  │   NAS Backup        │    │   Metrics Server    │    │   Auto-Recovery     │                   │
│  │  192.168.15.52      │    │    Port: 9100       │    │      Daemon         │                   │
│  │  Daily Snapshots    │    │  System Metrics     │    │  Self-Healing       │                   │
│  └─────────────────────┘    └─────────────────────┘    └─────────────────────┘                   │
└───────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Cluster Specifications

| Component | Specification |
|-----------|---------------|
| **Cluster Type** | K3s v1.33.6 (Lightweight Kubernetes) |
| **Control Plane** | 3 nodes (HA with embedded etcd) |
| **Worker Nodes** | 2 nodes |
| **Total Nodes** | 5 |
| **Storage** | Longhorn CSI (distributed block storage) |
| **Load Balancer** | MetalLB (Layer 2 mode) |
| **Database** | CloudNative-PG PostgreSQL 17 (3 replicas) |
| **VIP** | 192.168.15.200 |

---

## Application Architecture

### Project Structure

```
cold-backend/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── auth/                    # Authentication logic
│   │   ├── jwt.go              # JWT token management
│   │   └── password.go         # Password hashing
│   ├── config/                  # Configuration management
│   │   └── config.go
│   ├── db/                      # Database connection
│   │   └── postgres.go
│   ├── handlers/                # HTTP request handlers
│   │   ├── auth_handler.go
│   │   ├── customer_handler.go
│   │   ├── entry_handler.go
│   │   ├── entry_event_handler.go
│   │   ├── page_handler.go
│   │   ├── rent_payment_handler.go
│   │   ├── room_entry_handler.go
│   │   ├── system_setting_handler.go
│   │   └── user_handler.go
│   ├── http/                    # HTTP routing
│   │   └── router.go
│   ├── middleware/              # HTTP middleware
│   │   ├── auth.go             # Authentication middleware
│   │   └── cors.go             # CORS middleware
│   ├── models/                  # Data models
│   │   ├── customer.go
│   │   ├── entry.go
│   │   ├── entry_event.go
│   │   ├── rent_payment.go
│   │   ├── room_entry.go
│   │   ├── system_setting.go
│   │   └── user.go
│   ├── repositories/            # Data access layer
│   │   ├── customer_repository.go
│   │   ├── entry_event_repository.go
│   │   ├── entry_repository.go
│   │   ├── rent_payment_repository.go
│   │   ├── room_entry_repository.go
│   │   ├── system_setting_repository.go
│   │   └── user_repository.go
│   └── services/                # Business logic layer
│       ├── customer_service.go
│       ├── entry_event_service.go
│       ├── entry_service.go
│       ├── rent_payment_service.go
│       ├── room_entry_service.go
│       ├── system_setting_service.go
│       └── user_service.go
├── migrations/                  # Database migrations
│   ├── 001_create_users.sql
│   ├── 005_create_customers.sql
│   ├── 006_create_entries.sql
│   ├── 007_create_entry_events.sql
│   ├── 008_create_room_entries.sql
│   ├── 009_create_system_settings.sql
│   └── 010_create_rent_payments.sql
├── templates/                   # HTML templates
│   ├── user_login.html
│   ├── dashboard_employee.html
│   ├── dashboard_accountant.html
│   ├── dashboard_admin.html
│   ├── entry_room.html
│   ├── itam_serch.html
│   ├── rent_management.html
│   └── ...
├── static/                      # Static assets (CSS, JS, images)
├── .env                        # Environment variables (not in git)
├── .env.example                # Example environment file
├── go.mod                      # Go module definition
├── go.sum                      # Dependency checksums
└── README.md                   # This file
```

### Design Pattern: Layered Architecture

```
┌─────────────────────────────────────────────────┐
│              HTTP Layer (Router)                 │
│  - Route definitions                            │
│  - Middleware application                       │
└─────────────────┬───────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────┐
│           Handlers (Controllers)                │
│  - Request validation                           │
│  - Response formatting                          │
│  - HTTP-specific logic                          │
└─────────────────┬───────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────┐
│          Services (Business Logic)              │
│  - Business rules                               │
│  - Data validation                              │
│  - Orchestration                                │
└─────────────────┬───────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────┐
│       Repositories (Data Access)                │
│  - Database queries                             │
│  - CRUD operations                              │
│  - Data mapping                                 │
└─────────────────┬───────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────┐
│            PostgreSQL Database                  │
└─────────────────────────────────────────────────┘
```

---

## Technology Stack

### Backend
- **Go 1.22** - Primary programming language
- **Gorilla Mux** - HTTP routing and middleware
- **pgx/v5** - PostgreSQL driver
- **golang-jwt/jwt** - JWT authentication
- **bcrypt** - Password hashing
- **Viper** - Configuration management
- **godotenv** - Environment variable loading

### Frontend
- **HTML5** - Markup
- **Tailwind CSS** - Utility-first CSS framework
- **Vanilla JavaScript** - Client-side logic
- **Bootstrap Icons** - Icon library

### Database
- **PostgreSQL 15** - Primary database
- **Docker** - Database containerization

### DevOps & Infrastructure
- **K3s** - Lightweight Kubernetes distribution
- **Longhorn** - Distributed block storage
- **CloudNative-PG** - PostgreSQL operator for Kubernetes
- **MetalLB** - Bare metal load balancer
- **Docker** - Container runtime
- **Git** - Version control

---

## Infrastructure Components

### Kubernetes Resources

| Resource | Purpose | Location |
|----------|---------|----------|
| `01-secret.yaml` | Database credentials | `k8s/` |
| `02-configmap.yaml` | Application configuration | `k8s/` |
| `03-deployment-employee.yaml` | Employee API pods | `k8s/` |
| `04-deployment-customer.yaml` | Customer API pods | `k8s/` |
| `05-service.yaml` | ClusterIP services | `k8s/` |
| `metallb-config.yaml` | Load balancer pool | `k8s/` |
| `web-loadbalancer.yaml` | VIP service | `k8s/` |
| `postgres-loadbalancer.yaml` | Database VIP | `k8s/` |

### Infrastructure Scripts

| Script | Purpose |
|--------|---------|
| `cold_auto_recovery.py` | Auto-recovery daemon for all components |
| `power_recovery.sh` | Boot-time recovery after power loss |
| `metrics_server.py` | System metrics API (port 9100) |
| `get_system_metrics.sh` | Metrics collection |
| `backup_postgres.sh` | Database backup to NAS |
| `fix_longhorn_after_reboot.sh` | Storage recovery |

---

## Getting Started

### Prerequisites

- **Go 1.22 or higher** - [Download](https://golang.org/dl/)
- **PostgreSQL 15 or higher** - Can use Docker
- **Docker & Docker Compose** (optional) - [Download](https://www.docker.com/get-started)
- **Git** - [Download](https://git-scm.com/downloads)

### Installation

1. **Clone the repository**
```bash
git clone <repository-url>
cd cold-backend
```

2. **Install Go dependencies**
```bash
go mod download
```

3. **Set up environment variables**
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. **Start PostgreSQL** (using Docker)
```bash
docker run --name cold-storage-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=cold_db \
  -p 5432:5432 \
  -d postgres:15
```

5. **Run database migrations**
```bash
# Run migrations in order
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/001_create_users.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/002_add_auth_to_users.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/003_seed_admin_user.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/004_add_role_to_users.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/005_create_customers.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/006_create_entries.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/007_create_entry_events.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/008_create_room_entries.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/009_create_system_settings.sql
docker exec -i cold-storage-postgres psql -U postgres -d cold_db < migrations/010_create_rent_payments.sql
```

6. **Build the application**
```bash
go build -o cold-backend cmd/server/main.go
```

7. **Run the application**
```bash
./cold-backend
```

The application will start on `http://localhost:8080`

### Configuration

Edit the `.env` file to configure the application:

```env
# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=cold_db

# Server Configuration
SERVER_PORT=8080
```

### Database Setup

#### Quick Setup Script

```bash
# Create database and run all migrations
./scripts/setup_db.sh
```

#### Manual Setup

```bash
# Connect to PostgreSQL
docker exec -it cold-storage-postgres psql -U postgres

# Create database
CREATE DATABASE cold_db;

# Exit and run migrations
\q

# Run migrations (see Installation step 5)
```

#### Seed Admin User

The migration `003_seed_admin_user.sql` creates a default admin user:

- **Email:** `admin@cold.com`
- **Password:** `admin123`

**⚠️ Change this password immediately in production!**

---

## User Roles & Permissions

The system implements comprehensive role-based access control (RBAC):

### Role Definitions

| Role | Description | Dashboard |
|------|-------------|-----------|
| **Employee** | Can create entries and manage room assignments | `/dashboard` |
| **Accountant** | Can process payments and view financial data | `/accountant/dashboard` |
| **Admin** | Full system access including user management | `/admin/dashboard` |

### Permission Matrix

| Action | Employee | Accountant | Admin |
|--------|----------|------------|-------|
| **Entries** | | | |
| View Entries | ✅ | ✅ | ✅ |
| Create Entry | ✅ | ❌ | ✅ |
| Search Entries | ✅ | ✅ | ✅ |
| **Room Management** | | | |
| View Room Entries | ✅ | ✅ | ✅ |
| Create Room Entry | ✅ | ❌ | ✅ |
| Update Room Entry | ✅ | ❌ | ✅ |
| **Payments** | | | |
| View Payments | ❌ | ✅ | ✅ |
| Create Payment | ❌ | ✅ | ✅ |
| View Payment History | ❌ | ✅ | ✅ |
| **Customers** | | | |
| View Customers | ✅ | ✅ | ✅ |
| Search Customers | ✅ | ✅ | ✅ |
| Create Customer | ✅ | ✅ | ✅ |
| **System** | | | |
| View Settings | ❌ | ❌ | ✅ |
| Update Settings | ❌ | ❌ | ✅ |
| Manage Users | ❌ | ❌ | ✅ |

### Authentication Flow

```
┌─────────────────┐
│   User Login    │
│  POST /auth/login│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Validate       │
│  Credentials    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Generate JWT   │
│  with role      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Role-based     │
│  Redirect       │
└─────────────────┘
    │    │    │
    │    │    └─→ Admin → /admin/dashboard
    │    └──────→ Accountant → /accountant/dashboard
    └───────────→ Employee → /dashboard
```

---

## API Documentation

### Base URL
```
http://localhost:8080
```

### Authentication

All protected endpoints require a JWT token in the Authorization header:

```
Authorization: Bearer <token>
```

### Endpoints

#### Authentication

##### Login
```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

Response:
{
  "token": "eyJhbGc...",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "role": "employee",
    "name": "John Doe"
  }
}
```

##### Signup
```http
POST /auth/signup
Content-Type: application/json

{
  "email": "newuser@example.com",
  "password": "password123",
  "name": "Jane Doe",
  "role": "employee"
}
```

---

#### Entries

##### List All Entries
```http
GET /api/entries
Authorization: Bearer <token>

Response: Array of entries
```

##### Create Entry (Employee/Admin only)
```http
POST /api/entries
Authorization: Bearer <token>
Content-Type: application/json

{
  "phone": "9999999999",
  "name": "Customer Name",
  "village": "Village Name",
  "expected_quantity": 500,
  "truck_category": "seed"
}

Response:
{
  "id": 1,
  "customer_id": 5,
  "phone": "9999999999",
  "name": "Customer Name",
  "village": "Village Name",
  "expected_quantity": 500,
  "truck_category": "seed",
  "truck_number": "SEED-001",
  "created_by_user_id": 2,
  "created_at": "2025-12-13T10:00:00Z",
  "updated_at": "2025-12-13T10:00:00Z"
}
```

##### Get Entry by ID
```http
GET /api/entries/{id}
Authorization: Bearer <token>
```

##### Get Entries by Customer
```http
GET /api/entries/customer/{customer_id}
Authorization: Bearer <token>
```

---

#### Room Entries

##### List Room Entries
```http
GET /api/room-entries
Authorization: Bearer <token>
```

##### Create Room Entry (Employee/Admin only)
```http
POST /api/room-entries
Authorization: Bearer <token>
Content-Type: application/json

{
  "entry_id": 1,
  "room_no": "A-101",
  "floor": "1",
  "gate_no": "G1",
  "quantity": 500,
  "remark": "First floor storage"
}
```

##### Update Room Entry (Employee/Admin only)
```http
PUT /api/room-entries/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "room_no": "A-102",
  "floor": "1",
  "gate_no": "G1",
  "quantity": 450,
  "remark": "Updated location"
}
```

---

#### Payments

##### Create Payment (Accountant/Admin only)
```http
POST /api/rent-payments
Authorization: Bearer <token>
Content-Type: application/json

{
  "entry_id": 1,
  "customer_name": "Customer Name",
  "customer_phone": "9999999999",
  "total_rent": 10000,
  "amount_paid": 5000,
  "balance": -5000,
  "notes": "Partial payment"
}
```

##### List All Payments (Accountant/Admin only)
```http
GET /api/rent-payments
Authorization: Bearer <token>
```

##### Get Payments by Entry (Accountant/Admin only)
```http
GET /api/rent-payments/entry/{entry_id}
Authorization: Bearer <token>
```

##### Get Payments by Phone (Accountant/Admin only)
```http
GET /api/rent-payments/phone?phone=9999999999
Authorization: Bearer <token>
```

---

#### Customers

##### Search by Phone
```http
GET /api/customers/search?phone=9999999999
Authorization: Bearer <token>
```

##### Create Customer
```http
POST /api/customers
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Customer Name",
  "phone": "9999999999",
  "village": "Village Name",
  "address": "Full address"
}
```

---

#### System Settings (Admin only)

##### List Settings
```http
GET /api/settings
Authorization: Bearer <token>
```

##### Get Setting by Key
```http
GET /api/settings/{key}
Authorization: Bearer <token>

Example:
GET /api/settings/rent_per_item
```

##### Update Setting
```http
PUT /api/settings/{key}
Authorization: Bearer <token>
Content-Type: application/json

{
  "setting_value": "15.50"
}
```

---

#### Users (Admin only)

##### List Users
```http
GET /api/users
Authorization: Bearer <token>
```

##### Create User
```http
POST /api/users
Authorization: Bearer <token>
Content-Type: application/json

{
  "email": "employee@example.com",
  "password": "password123",
  "name": "Employee Name",
  "role": "employee"
}
```

##### Update User
```http
PUT /api/users/{id}
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Updated Name",
  "role": "accountant"
}
```

##### Delete User
```http
DELETE /api/users/{id}
Authorization: Bearer <token>
```

---

## Frontend Pages

### Public Pages

- **`/`** - Login page (redirects to `/login`)
- **`/login`** - User login form
- **`/logout`** - Logout and clear session

### Employee Dashboard

- **`/dashboard`** - Employee dashboard (entry statistics, quick actions)
- **`/entry-room`** - Create new entries
- **`/item-search`** - Search entries by phone/truck/room
- **`/room-entry-edit`** - Edit room assignments
- **`/loading-invoice`** - Generate loading invoices

### Accountant Dashboard

- **`/accountant/dashboard`** - Accountant dashboard (payment statistics)
- **`/rent-management`** - Process payments
- **`/item-search`** - Search entries (read-only)

### Admin Dashboard

- **`/admin/dashboard`** - Admin dashboard (complete overview)
- **`/employees`** - User management
- **`/system-settings`** - System configuration
- All employee and accountant pages

---

## Database Schema

### Tables Overview

```
users               - System users (employees, accountants, admins)
customers           - Cold storage customers
entries             - Storage entries
entry_events        - Entry lifecycle events
room_entries        - Physical storage locations
rent_payments       - Payment records
system_settings     - Configuration settings
```

### Detailed Schema

#### users
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100) UNIQUE NOT NULL,
    phone VARCHAR(15),
    village VARCHAR(100),
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'employee',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### customers
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
```

#### entries
```sql
CREATE TABLE entries (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id),
    phone VARCHAR(15) NOT NULL,
    name VARCHAR(100) NOT NULL,
    village VARCHAR(100),
    expected_quantity INTEGER NOT NULL,
    truck_category VARCHAR(10) NOT NULL,
    truck_number VARCHAR(20) NOT NULL,
    created_by_user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### room_entries
```sql
CREATE TABLE room_entries (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id) ON DELETE CASCADE,
    room_no VARCHAR(20) NOT NULL,
    floor VARCHAR(10) NOT NULL,
    gate_no VARCHAR(10) NOT NULL,
    quantity INTEGER NOT NULL,
    remark TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

#### rent_payments
```sql
CREATE TABLE rent_payments (
    id SERIAL PRIMARY KEY,
    entry_id INTEGER REFERENCES entries(id),
    customer_name VARCHAR(100) NOT NULL,
    customer_phone VARCHAR(15) NOT NULL,
    total_rent NUMERIC(10, 2) NOT NULL,
    amount_paid NUMERIC(10, 2) NOT NULL,
    balance NUMERIC(10, 2) NOT NULL,
    payment_date TIMESTAMP DEFAULT NOW(),
    processed_by_user_id INTEGER REFERENCES users(id),
    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### system_settings
```sql
CREATE TABLE system_settings (
    id SERIAL PRIMARY KEY,
    setting_key VARCHAR(100) UNIQUE NOT NULL,
    setting_value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
```

### Entity Relationships

```
users (1) ──────────< (many) entries [created_by_user_id]
users (1) ──────────< (many) rent_payments [processed_by_user_id]

customers (1) ──────< (many) entries [customer_id]

entries (1) ────────< (many) room_entries [entry_id]
entries (1) ────────< (many) entry_events [entry_id]
entries (1) ────────< (many) rent_payments [entry_id]
```

---

## Development Guide

### Running in Development

```bash
# Run with auto-reload (install air)
go install github.com/cosmtrek/air@latest
air

# Or run directly
go run cmd/server/main.go
```

### Project Conventions

#### Naming Conventions
- **Files**: `snake_case.go`
- **Packages**: `lowercase`
- **Types**: `PascalCase`
- **Functions**: `PascalCase` (exported), `camelCase` (unexported)
- **Variables**: `camelCase`
- **Constants**: `PascalCase` or `ALL_CAPS`

#### Code Organization
- **Handlers**: HTTP request/response only
- **Services**: Business logic
- **Repositories**: Database operations only
- **Models**: Data structures

#### Error Handling
```go
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

### Adding a New Feature

1. **Create model** in `internal/models/`
2. **Create migration** in `migrations/`
3. **Create repository** in `internal/repositories/`
4. **Create service** in `internal/services/`
5. **Create handler** in `internal/handlers/`
6. **Add routes** in `internal/http/router.go`
7. **Wire up** in `cmd/server/main.go`
8. **Create frontend** in `templates/`

### Database Migrations

Migrations are SQL files numbered sequentially:

```bash
migrations/
├── 001_create_users.sql
├── 002_add_auth_to_users.sql
└── ...
```

Always create new migrations, never modify existing ones.

---

## Testing

### Running Tests

See the comprehensive test documentation in `TEST_RESULTS.md` and `ROLE_BASED_ACCESS_TEST.md`.

#### Quick Test Suite

```bash
# Run API tests
./scripts/test_api.sh

# Test specific role
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"employee@test.com","password":"test123"}'
```

### Test Users

| Email | Password | Role |
|-------|----------|------|
| admin@cold.com | admin123 | admin |
| employee@test.com | test123 | employee |
| accountant@test.com | test123 | accountant |

---

## Production Deployment

### Kubernetes Deployment (Recommended)

```bash
# Build and push Docker image
docker build -t cold-backend:v2.0.0 .
docker save cold-backend:v2.0.0 -o cold-backend-v2.0.0.tar

# Import on all K3s nodes
for node in 192.168.15.110 192.168.15.111 192.168.15.112 192.168.15.113 192.168.15.114; do
    scp cold-backend-v2.0.0.tar root@$node:/tmp/
    ssh root@$node "ctr images import /tmp/cold-backend-v2.0.0.tar"
done

# Apply Kubernetes manifests
kubectl apply -f k8s/01-secret.yaml
kubectl apply -f k8s/02-configmap.yaml
kubectl apply -f k8s/03-deployment-employee.yaml
kubectl apply -f k8s/04-deployment-customer.yaml
kubectl apply -f k8s/05-service.yaml
kubectl apply -f k8s/web-loadbalancer.yaml

# Verify deployment
kubectl get pods -l app=cold-backend
kubectl get svc
```

### Access Points

| Service | URL | Port |
|---------|-----|------|
| Employee Portal | http://192.168.15.200:8080 | 8080 |
| Customer Portal | http://192.168.15.200:8081 | 8081 |
| PostgreSQL | 192.168.15.200 | 5432 |
| Metrics API | http://localhost:9100 | 9100 |

### Production Build (Standalone)

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 go build -o cold-backend cmd/server/main.go

# Build Docker image
docker build -t cold-backend:v2.0.0 .
```

---

## Auto-Recovery System

The system includes a comprehensive auto-recovery daemon that monitors and automatically fixes all failure scenarios.

### Components Monitored

| Component | Check Interval | Auto-Fix |
|-----------|---------------|----------|
| K3s Nodes | 30s | Restart k3s service |
| iSCSId Service | 30s | Enable on all nodes |
| Pod Health | 30s | Delete stuck/crashed pods |
| PostgreSQL Cluster | 30s | Trigger CNPG failover |
| Longhorn Volumes | 30s | Salvage faulted volumes |
| Docker Containers | 30s | Start stopped containers |
| Service Endpoints | 30s | Restart deployments |
| MetalLB VIP | 30s | Restart controller |
| NAS Mount | 30s | Remount shares |

### Installation

```bash
# Install auto-recovery system
sudo ./scripts/infrastructure/install_auto_recovery.sh

# Verify services
systemctl status cold-auto-recovery
systemctl status cold-metrics
systemctl status cold-power-recovery
```

### Systemd Services

| Service | Description |
|---------|-------------|
| `cold-auto-recovery.service` | Continuous monitoring daemon |
| `cold-metrics.service` | System metrics server |
| `cold-power-recovery.service` | Boot-time recovery (oneshot) |

### Logs

```bash
# View auto-recovery logs
journalctl -fu cold-auto-recovery

# View recovery actions
tail -f /var/log/cold-auto-recovery.log

# Manual health check
python3 scripts/infrastructure/cold_auto_recovery.py --once
```

---

## Troubleshooting

### Common Issues

#### Port Already in Use
```bash
# Find process using port 8080
lsof -i :8080

# Kill the process
kill -9 <PID>
```

#### Database Connection Failed
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Check connection
psql -h localhost -U postgres -d cold_db
```

#### Permission Denied Errors
```bash
# Ensure templates and static folders are readable
chmod -R 755 templates/ static/
```

#### JWT Token Expired
- Tokens expire after 24 hours by default
- User must log in again to get a new token

---

## Project Highlights

### High Availability Features
- ✅ 5-node K3s cluster with 3 control planes
- ✅ PostgreSQL HA with automatic failover (CloudNative-PG)
- ✅ Distributed storage with Longhorn (3 replicas)
- ✅ Virtual IP with MetalLB load balancer
- ✅ Auto-recovery daemon for self-healing
- ✅ Automatic power-loss recovery

### Security Features
- ✅ JWT-based authentication
- ✅ Password hashing with bcrypt
- ✅ Role-based access control
- ✅ SQL injection prevention (parameterized queries)
- ✅ CORS protection

### Performance Features
- ✅ Connection pooling (pgx)
- ✅ Horizontal pod autoscaling (2-5 replicas)
- ✅ Efficient database queries
- ✅ Minimal frontend dependencies

### Business Logic
- ✅ Automatic truck number generation
- ✅ Automatic category assignment based on quantity
- ✅ Cumulative payment balance calculation
- ✅ Entry tracking with user attribution
- ✅ Gate pass management with partial pickup
- ✅ Hindi receipt format support

---

## License

Copyright (c) 2025. All rights reserved.

This is proprietary software. Unauthorized copying, modification, distribution, or use of this software, via any medium, is strictly prohibited without explicit written permission from the copyright holder.

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| v2.0.0 | Dec 2025 | K3s HA cluster, auto-recovery system, PostgreSQL HA |
| v1.4.0 | Dec 2025 | Gate pass management, partial pickup feature |
| v1.3.0 | Dec 2025 | Kubernetes deployment, monitoring dashboard |
| v1.0.0 | Dec 2025 | Initial release |

---
