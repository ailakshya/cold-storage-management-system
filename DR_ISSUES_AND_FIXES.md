# Disaster Recovery - Issues and Fixes

## Date: 2025-12-26

## Testing Summary

Tested DR flow on Proxmox VM (192.168.15.97) to verify:
1. Cascading DB connection (VIP → 195 → localhost → Setup mode)
2. Setup page functionality
3. R2 backup listing
4. R2 restore process
5. Auto table creation

---

## Issues Found

### Issue 1: Migrations Directory Not Embedded in Binary

**Problem:**
- Binary requires external `migrations/` directory
- When running standalone binary, migrations fail with:
  ```
  Failed to run migrations: failed to read migrations directory: open migrations: no such file or directory
  ```

**Impact:** Server crashes if migrations folder missing

**Fix Options:**
1. **Embed migrations in binary** using Go embed
2. **Fetch from GitHub** at runtime
3. **Include in deployment package** (current workaround)

**Recommended:** Embed migrations in binary for true standalone operation

---

### Issue 2: R2 ListBackups Limited to 1000 Objects (FIXED)

**Problem:**
- S3 ListObjectsV2 returns max 1000 objects by default
- With 4200+ backups, only oldest 1000 were shown (alphabetical order)
- Setup page showed Dec 24 backups instead of Dec 26 (latest)

**Fix Applied:**
- Added pagination to fetch ALL objects
- Sort by LastModified descending
- Return only latest 50 backups
- Added total_count to response

**File:** `internal/handlers/setup_handler.go`

**Status:** FIXED

---

### Issue 3: Migration State Corruption

**Problem:**
- If server connects to DB with partial migration state, subsequent migrations fail
- Example: migrations_applied table exists but dependent tables missing
- Error: `relation "customers" does not exist`

**Impact:** Cannot run fresh migrations on corrupted state

**Fix:**
- R2 restore is the correct DR path (bypasses migrations)
- OR add "reset migrations" option in setup page

---

### Issue 4: Local Postgres Auto-Connect

**Problem:**
- Cascading fallback tries localhost:5432 with common passwords
- If local postgres running with default credentials, it connects
- Then tries to run migrations which may fail

**Expected Behavior:** This is correct - cascading should try all options

**Improvement:**
- If migrations fail, fall back to setup mode instead of crashing
- Add graceful degradation

---

## Correct DR Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    DISASTER SCENARIO                         │
│              (K3s cluster + all DBs down)                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 1: Download binary from GitHub                        │
│  curl -L https://github.com/.../releases/latest -o server   │
│  chmod +x server                                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 2: Run binary                                          │
│  ./server -mode employee                                     │
│                                                              │
│  Output:                                                     │
│  [DB] Trying VIP-DB... FAILED                               │
│  [DB] Trying 195... FAILED                                  │
│  [DB] Trying localhost... FAILED                            │
│  ╔══════════════════════════════════════════════════════╗   │
│  ║  NO DATABASE AVAILABLE - ENTERING SETUP MODE         ║   │
│  ╚══════════════════════════════════════════════════════╝   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 3: Open browser → http://server:8080/setup            │
│                                                              │
│  Setup page shows:                                           │
│  - Database connection form                                  │
│  - R2 backup status (Connected, 4200+ backups)              │
│  - Restore from R2 button                                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 4: Enter NEW database details                          │
│                                                              │
│  Host: new-db-server.local                                   │
│  Port: 5432                                                  │
│  User: cold_user                                             │
│  Password: ********                                          │
│  Database: cold_db                                           │
│                                                              │
│  [Test Connection] → Success!                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 5: Click "Restore from R2"                             │
│                                                              │
│  - Downloads latest backup (cold_db_20251226_013337.sql)    │
│  - Runs: psql $DB_URL -f backup.sql                         │
│  - Creates ALL tables from backup                            │
│  - Saves .env with DB credentials                            │
│  - Restarts server                                           │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 6: Server restarts with restored database              │
│                                                              │
│  [DB] Connected to new-db-server.local                       │
│  [Migrations] All tables exist, skipping                     │
│  Server running on :8080                                     │
│                                                              │
│  ✅ DISASTER RECOVERY COMPLETE                               │
└─────────────────────────────────────────────────────────────┘
```

---

## Files to Modify

### 1. Embed Migrations (Priority: HIGH)

**File:** `internal/database/migrator.go`

```go
import "embed"

//go:embed migrations/*.sql
var migrationsFS embed.FS

func (m *Migrator) RunMigrations(ctx context.Context) error {
    // Use embedded FS instead of os.ReadDir
    entries, err := migrationsFS.ReadDir("migrations")
    // ...
}
```

### 2. Graceful Migration Failure (Priority: MEDIUM)

**File:** `cmd/server/main.go`

```go
if err := migrator.RunMigrations(ctx); err != nil {
    log.Printf("Migrations failed: %v", err)
    log.Println("Entering setup mode for manual recovery...")
    startSetupMode(cfg)
    return
}
```

### 3. GitHub Binary Fetch (Priority: LOW)

**File:** `internal/handlers/setup_handler.go`

Add endpoint to download latest release from GitHub if binary needs update.

---

## R2 Backup Status

| Metric | Value |
|--------|-------|
| Total Backups | 4,242 |
| Total Size | 2.1 GB |
| Latest Backup | 2025-12-26 01:33:37 |
| Backup Frequency | Hourly |
| Bucket | cold-db-backups |

---

## Test Results

| Test | Result | Notes |
|------|--------|-------|
| Cascading DB connection | ✅ PASS | VIP → 195 → localhost → Setup |
| Setup mode activation | ✅ PASS | Shows setup page when no DB |
| R2 connection check | ✅ PASS | /setup/r2-check returns success |
| R2 backup listing | ✅ PASS | Shows latest 50 of 4242 backups |
| R2 restore | ⚠️ UNTESTED | SSH timeouts prevented full test |
| Auto table creation | ❌ FAIL | Needs embedded migrations |

---

## Next Steps

1. [ ] Embed migrations in binary
2. [ ] Add graceful migration failure → setup mode fallback
3. [ ] Test complete R2 restore flow
4. [ ] Create GitHub release with standalone binary
5. [ ] Document DR procedure for operators
6. [ ] Set up Discord webhook alerts
