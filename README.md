# Cold Storage Management System

Web-based management system for cold storage facilities with role-based access, inventory tracking, payment processing, and multi-language support.

**Version:** 1.5.51
**Last Updated:** December 24, 2025

## Features

- **Multi-Language Support:** English and Hindi (i18n)
- **Role-Based Access Control:** Admin, Employee, Accountant roles
- **Inventory Management:** Track items across rooms and gatars
- **Gate Pass System:** Issue, approve, and track item withdrawals
- **Payment Processing:** Rent calculations and payment tracking
- **Offline Mode:** Works on local network without internet
- **Customer Portal:** Self-service for customers to request gate passes
- **Auto-Recovery:** Automatic database fallback and setup wizard for disaster recovery
- **Cloud Backup:** Cloudflare R2 integration for offsite backups

## Tech Stack

- **Backend:** Go 1.23, Gorilla Mux, pgx/v5
- **Frontend:** HTML5, Tailwind CSS, Vanilla JS, Bootstrap Icons
- **Database:** PostgreSQL 17 (CloudNative-PG)
- **Infrastructure:** K3s, Longhorn, MetalLB
- **Monitoring:** Node Exporter, Custom metrics

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

| Role | Access |
|------|--------|
| Employee | Create entries, room assignments, gate pass entry |
| Accountant | Payment processing, rent management |
| Admin | Full access + user management, reports, system settings |

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

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /auth/login | User login |
| GET | /logout | User logout |

### Core APIs
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/entries | List entries |
| POST | /api/entries | Create entry |
| GET | /api/room-entries | List room entries |
| POST | /api/room-entries | Create room entry |
| PUT | /api/room-entries/:id | Update room entry |
| POST | /api/rent-payments | Create payment |
| GET | /api/gate-passes | List gate passes |
| POST | /api/gate-passes | Create gate pass |

### Admin APIs
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/users | List users |
| POST | /api/users | Create user |
| GET | /api/customers | List customers |

## Multi-Language Support

The application supports English and Hindi. Translation files are located in:
- `static/locales/en.json` - English translations
- `static/locales/hi.json` - Hindi translations

Users can switch languages using the dropdown in the header. The selected language is persisted in localStorage.

## Production Deployment

```bash
# Build Docker image
docker build -t cold-backend:v1.4.75 .

# Deploy to K3s
kubectl apply -f k8s/

# Or use deployment script
./scripts/deploy/deploy.sh v1.4.75
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
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=cold_db

# Server
PORT=8080
JWT_SECRET=your-secret-key
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
