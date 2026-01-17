# Cold Storage Management System

Web-based management system for cold storage facilities with role-based access, inventory tracking, payment processing, and multi-language support.

**Version:** 1.5.173
**Last Updated:** January 17, 2026

## Features

### Core Features
- **Multi-Language Support:** English and Hindi (i18n) with real-time switching
- **Role-Based Access Control:** Admin, Employee, Accountant, Guard, and Customer roles
- **Inventory Management:** Track items across rooms and gatars with visual occupancy maps
- **Gate Pass System:** Issue, approve, and track item withdrawals with pickup history
- **Payment Processing:** Rent calculations, payment tracking, and double-entry ledger system
- **Offline Mode:** Works on local network without internet connectivity
- **Operation Modes:** Loading mode and Unloading mode with workflow restrictions

### Customer Features
- **Customer Portal:** Dedicated self-service portal on port 8081
- **OTP Authentication:** Secure OTP-based login for customers
- **Online Payments:** Razorpay integration for online rent payments
- **Gate Pass Requests:** Customers can request gate passes online
- **Family Members:** Link family members for delegated access
- **Transaction History:** View payment history and receipts

### Guard & Entry System
- **Guard Register:** Token-based truck registration system
- **Daily Token Colors:** Color-coded tokens for daily tracking
- **Dual Entry Types:** Seed and Sell categorization
- **Token Management:** Skip lost tokens, track token usage
- **Guard Dashboard:** Real-time stats and pending entry tracking

### Accounting & Finance
- **Ledger System:** Double-entry accounting with complete audit trail
- **Debt Management:** Debt approval workflow with admin oversight  
- **Balance Tracking:** Real-time customer balance and debtors list
- **Payment Reminders:** Automated SMS/WhatsApp payment reminders
- **Receipt Generation:** Automatic receipt numbering and PDF generation
- **Online Transactions:** Razorpay payment reconciliation

### Communication
- **SMS Integration:** Bulk SMS, payment reminders, and notifications
- **WhatsApp Support:** WhatsApp messaging for customer communication
- **Boli Notifications:** Loading completion notifications
- **Message Logs:** Complete SMS/WhatsApp delivery tracking

### Security & Compliance
- **Two-Factor Authentication:** TOTP-based 2FA for admin accounts
- **Dual Admin Approval:** Protected settings require two admin approvals
- **Comprehensive Audit Logs:** Track all edits, logins, and admin actions
- **Customer Activity Tracking:** Monitor customer portal usage
- **Session Management:** Secure JWT-based session handling
- **Rate Limiting:** Login attempt rate limiting

### Infrastructure & Monitoring
- **TimescaleDB Metrics:** Dedicated metrics database for performance monitoring
- **Prometheus Integration:** System-wide metric collection
- **Grafana Dashboards:** Visual monitoring dashboards
- **API Analytics:** Request logging, top endpoints, slowest queries
- **Node Metrics:** K8s node CPU, memory, and disk monitoring
- **PostgreSQL Metrics:** Database performance and health tracking
- **Alert System:** Configurable alerts with thresholds
- **Auto-Recovery:** Automatic database fallback and setup wizard
- **Cloud Backup:** Cloudflare R2 integration for offsite backups
- **Point-in-Time Restore:** Snapshot-based database restoration

### Advanced Features
- **Node Provisioning:** Automated K8s node setup and management
- **Deployment System:** One-click application deployments with rollback
- **Season Management:** End-of-season data archival with dual approval
- **Customer Merge:** Merge duplicate customer records with undo
- **Room Visualization:** Visual gatar occupancy and stock levels
- **Soft Delete:** Recoverable entry deletion with restore capability
- **Bulk Operations:** Bulk reassignment and deletion of entries
- **Label Printing:** Brother label printer and HP receipt printer integration
- **Advanced Reports:** PDF/CSV exports for customers and daily summaries

## Tech Stack

- **Backend:** Go 1.23, Gorilla Mux, pgx/v5
- **Frontend:** HTML5, Tailwind CSS, Vanilla JS, Bootstrap Icons
- **Database:** 
  - PostgreSQL 17 (CloudNative-PG) - Main application database
  - TimescaleDB - Metrics and monitoring database
- **Infrastructure:** K3s (5-node cluster), Longhorn (storage), MetalLB (load balancer)
- **Monitoring:** Prometheus, Grafana, Node Exporter, Custom metrics
- **Payments:** Razorpay integration for online payments
- **Communication:** SMS/WhatsApp API integration
- **Cloud Storage:** Cloudflare R2 for backups

## Project Structure

```
cold-backend/
├── cmd/server/          # Application entry point
├── configs/             # Configuration files
├── docs/                # Documentation
├── internal/
│   ├── handlers/        # HTTP request handlers
│   ├── http/            # Router and middleware
│   ├── models/          # Data models
│   ├── repositories/    # Database operations
│   └── services/        # Business logic
├── k8s/                 # Kubernetes manifests
├── migrations/          # SQL migrations
├── scripts/             # Deployment and utility scripts
├── static/              # Static assets (CSS, JS, fonts)
│   ├── css/             # Tailwind, Bootstrap Icons
│   ├── fonts/           # Web fonts
│   ├── js/              # JavaScript (i18n)
│   └── locales/         # Translation files (en.json, hi.json)
└── templates/           # HTML templates
```

## Quick Start

```bash
# Install dependencies
go mod download

# Start PostgreSQL
docker run --name cold-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=cold_db -p 5432:5432 -d postgres:17

# Run migrations
for f in migrations/*.sql; do docker exec -i cold-postgres psql -U postgres -d cold_db < "$f"; done

# Build & run
go build -o server ./cmd/server/
./server
```

Access at `http://localhost:8080`

## User Roles

| Role | Access | Portal |
|------|--------|--------|
| **Employee** | Create entries, room assignments, gate pass operations, debt requests | Main App |
| **Accountant** | Payment processing, rent management, reports, ledger access | Main App |
| **Admin** | Full access + user management, infrastructure, monitoring, dual approvals | Main App |
| **Guard** | Token-based truck registration, entry creation, guard dashboard | Main App |
| **Customer** | View entries, request gate passes, make payments, view receipts | Customer Portal (8081) |

## Default Login

- **Email:** admin@cold.com
- **Password:** admin123

## Storage Layout

The facility has 5 storage areas:

| Room | Type | Gatars | Status |
|------|------|--------|--------|
| Room 1 | Seed | 1-680 | Active |
| Room 2 | Seed | 681-1360 | Active |
| Room 3 | Sell | 1361-2040 | Active |
| Room 4 | Sell | TBD | Pending |
| Gallery | Sell | TBD | Pending |

Each room has 5 floors (0-4) with gatar ranges defined per floor.

## API Endpoints

The system has 90+ API endpoints across multiple categories. See [API Documentation](docs/API_DOCUMENTATION.md) for complete reference.

### Authentication & Security
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /auth/login | User login with JWT |
| POST | /auth/signup | User registration |
| POST | /auth/verify-2fa | Verify TOTP 2FA code |
| POST | /auth/send-otp | Customer OTP login |
| POST | /auth/verify-otp | Verify customer OTP |

### Core Business APIs
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | /api/entries | Entry management |
| GET/POST/PUT | /api/room-entries | Room assignment |
| GET/POST | /api/customers | Customer management |
| GET | /api/customers/{id}/family-members | Family member management |
| GET/POST | /api/gate-passes | Gate pass operations (unloading mode) |
| POST | /api/rent-payments | Payment processing |
| GET | /api/ledger/customer/{phone} | Customer ledger |

### Guard & Token System
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | /api/guard/entries | Guard entry management |
| GET | /api/guard/next-token | Get next available token |
| GET/PUT | /api/token-color | Daily token color management |

### Administration
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST/PUT/DELETE | /api/users | User management |
| GET/PUT | /api/settings | System settings |
| POST | /api/admin/setting-changes | Request protected setting change |
| GET/POST | /api/season | Season management (dual approval) |
| GET | /api/reports/* | PDF/CSV report generation |

### Infrastructure & Monitoring
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/infrastructure/* | K8s cluster status, backups, PostgreSQL health |
| GET/POST | /api/infrastructure/nodes | Node provisioning and management |
| GET | /api/monitoring/* | TimescaleDB metrics, API analytics, alerts |
| GET/POST | /api/deployments | Application deployment management |
| GET | /health | Health checks and readiness probes |
| GET | /metrics | Prometheus metrics (admin auth) |

### Communication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/sms/bulk | Send bulk SMS |
| POST | /api/sms/payment-reminders | Payment reminder SMS |
| GET | /api/sms/logs | SMS delivery logs |

### Customer Portal (Port 8081)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/dashboard | Customer dashboard data |
| POST | /api/gate-pass-requests | Request gate pass |
| POST | /api/payment/create-order | Create Razorpay order |
| POST | /api/payment/verify | Verify payment |
| GET | /api/payment/transactions | Transaction history |

## Multi-Language Support

The application supports English and Hindi. Translation files are located in:
- `static/locales/en.json` - English translations
- `static/locales/hi.json` - Hindi translations

Users can switch languages using the dropdown in the header. The selected language is persisted in localStorage.

## Production Deployment

```bash
# Build Docker image
docker build -t cold-backend:v1.5.173 .

# Deploy to K3s
kubectl apply -f k8s/

# Or use deployment script
./scripts/deploy/deploy.sh v1.5.173
```

**Production URL:** http://192.168.15.200:8080

### K3s Cluster

| Node | IP | Role |
|------|-----|------|
| k3s-master | 192.168.15.110 | Control Plane |
| k3s-worker-1 | 192.168.15.111 | Worker |
| k3s-worker-2 | 192.168.15.112 | Worker |
| k3s-worker-3 | 192.168.15.113 | Worker |
| k3s-worker-4 | 192.168.15.114 | Worker |

**VIP:** 192.168.15.200 (MetalLB)

## Documentation

See `docs/` folder for detailed documentation:

- [API Documentation](docs/API_DOCUMENTATION.md) - Complete API reference
- [Database Schema](docs/DATABASE_SCHEMA.md) - Database design
- [K3s Infrastructure](docs/K3S_INFRASTRUCTURE_DOCUMENTATION.md) - Cluster setup
- [Room Layout](docs/ROOM_LAYOUT.md) - Gatar mapping
- [Documentation Index](docs/DOCUMENTATION_INDEX.md) - Full index

## Environment Variables

```env
# Main Application Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=cold_db

# TimescaleDB Metrics Database (optional)
TIMESCALE_HOST=localhost
TIMESCALE_PORT=5432
TIMESCALE_USER=postgres
TIMESCALE_PASSWORD=postgres
TIMESCALE_DB=metrics_db

# Server Configuration
PORT=8080
CUSTOMER_PORTAL_PORT=8081
JWT_SECRET=your-secret-key
CUSTOMER_JWT_SECRET=your-customer-jwt-secret

# Razorpay Payment Integration (optional)
RAZORPAY_KEY_ID=your_razorpay_key_id
RAZORPAY_KEY_SECRET=your_razorpay_key_secret
RAZORPAY_WEBHOOK_SECRET=your_webhook_secret

# SMS/WhatsApp Integration (optional)
SMS_API_KEY=your_sms_api_key
SMS_API_URL=https://your-sms-provider.com/api
WHATSAPP_API_KEY=your_whatsapp_api_key

# Cloudflare R2 Backup (optional)
R2_ACCOUNT_ID=your_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET_NAME=cold-storage-backups

# Printer Integration (optional)
LABEL_PRINTER_IP=192.168.15.x
HP_PRINTER_IP=192.168.15.y
```

## Disaster Recovery

The application includes built-in disaster recovery features:

### Automatic Database Fallback

When the app starts, it tries to connect to databases in order:
1. **K8s Cluster (Primary):** 192.168.15.200:5432
2. **Backup Server:** 192.168.15.195:5434

If both fail, the app enters **Setup Mode**.

### Setup Mode

When no database is available, the app shows a setup wizard:
- Configure database connection manually
- Restore from Cloudflare R2 backup

Access the setup screen at `http://localhost:8080/setup`

### Recovery Package

A standalone recovery package is available at `/home/lakshya/backups/cold-backend/`:
- `server` - Linux binary (31 MB)
- `templates/` - HTML templates
- `static/` - CSS, JS, fonts
- `RECOVERY.md` - Step-by-step recovery guide

### Quick Recovery

```bash
# 1. Extract recovery package
tar xzf cold-backend.tar.gz
cd cold-backend

# 2. Configure database (or use setup wizard)
cat > .env << 'EOF'
DB_HOST=192.168.15.195
DB_PORT=5434
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=cold_db
JWT_SECRET=cold-backend-jwt-secret-2025
EOF

# 3. Run
./server
```

## License

Proprietary - All rights reserved.
