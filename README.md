# Cold Storage Management System

Web-based management system for cold storage facilities with role-based access, inventory tracking, and payment processing.

## Tech Stack

- **Backend:** Go 1.22, Gorilla Mux, pgx/v5
- **Frontend:** HTML5, Tailwind CSS, Vanilla JS
- **Database:** PostgreSQL 17
- **Infrastructure:** K3s, Longhorn, CloudNative-PG, MetalLB

## Project Structure

```
cold-backend/
├── cmd/server/          # Entry point
├── configs/             # Configuration
├── docs/                # Documentation
├── internal/
│   ├── handlers/        # HTTP handlers
│   ├── models/          # Data models
│   ├── repositories/    # Database layer
│   └── services/        # Business logic
├── k8s/                 # Kubernetes manifests
├── migrations/          # SQL migrations
├── scripts/
│   ├── deploy/          # Deployment scripts
│   └── test/            # Test scripts
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
go build -o bin/server cmd/server/main.go
./bin/server
```

Access at `http://localhost:8080`

## User Roles

| Role | Access |
|------|--------|
| Employee | Create entries, room assignments |
| Accountant | Payment processing |
| Admin | Full access + user management |

## Default Login

- **Email:** admin@cold.com
- **Password:** admin123

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /auth/login | User login |
| GET | /api/entries | List entries |
| POST | /api/entries | Create entry |
| GET | /api/room-entries | List room entries |
| POST | /api/room-entries | Create room entry |
| POST | /api/rent-payments | Create payment |
| GET | /api/gate-passes | List gate passes |

## Production Deployment

```bash
# Build image
docker build -t cold-backend:latest .

# Deploy to K3s
kubectl apply -f k8s/
```

**Production URL:** http://192.168.15.200:8080

## Documentation

See `docs/` folder:
- [API Documentation](docs/API_DOCUMENTATION.md)
- [Database Schema](docs/DATABASE_SCHEMA.md)
- [K3s Infrastructure](docs/K3S_INFRASTRUCTURE_DOCUMENTATION.md)
- [Room Layout](docs/ROOM_LAYOUT.md)

## License

Proprietary - All rights reserved.
