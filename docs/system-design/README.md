# System Design Documentation - Diagrams

This folder contains all architecture and system design diagrams for the Cold Storage Management System.

## Diagrams

### 01-system-architecture.png
**Complete System Architecture**

Shows the 6-layer architecture:
- **Layer 1**: Client Layer - Employee Portal, Customer Portal, Guard Dashboard, Admin Panel, Infrastructure UI
- **Layer 2**: Load Balancer - MetalLB VIP (192.168.15.200)
- **Layer 3**: Application Layer - Main Application (Go), Customer Portal Server, External Integrations
- **Layer 4**: Data Persistence - PostgreSQL 17 (CloudNative-PG), TimescaleDB (Metrics)
- **Layer 5**: Infrastructure - K3s Cluster (5 Nodes), Longhorn, Prometheus, Grafana
- **Layer 6**: Backup & Recovery - Cloudflare R2, Auto-Failover, Setup Mode

**Key Components**:
- 42 Handler Types
- 23 Service Layers
- 30 Repositories
- 90+ API Endpoints

---

### 02-component-flow.png
**Request Flow & Component Interactions**

Shows the detailed request/response flow through the system:
- **User Request**: HTTP Request with JWT token
- **Middleware Pipeline**: HTTPS Redirect → Security Headers → API Logging → Authentication → Authorization (RBAC)
- **Handler Layer**: Entry, Gate Pass, Customer, Payment, Room Entry handlers (42 total)
- **Service Layer**: Business logic, validation, state management
- **Repository Layer**: Data access, SQL queries, transaction management
- **Database**: PostgreSQL (main data), TimescaleDB (metrics)

**Special Flows**:
1. **Gate Pass Flow**: Create → Validate Stock → Log Event → Notify SMS → Complete
2. **Payment Flow**: Create Order → Razorpay → Verify → Update Ledger → Receipt
3. **Entry Flow**: Create Entry → Assign Room → Update Inventory → Log → Invoice

---

### 03-database-schema.png
**Database Schema - Entity Relationship Diagram (ERD)**

Shows all 30+ tables and their relationships:

**Core Entities** (Blue):
- `customers` - Customer master data (1:M with entries, gate_passes, payments)
- `entries` - Entry/truck registration (1:M with room_entries)
- `room_entries` - Storage location assignments
- `gate_passes` - Gate pass operations (1:M with gate_pass_pickups)
- `gate_pass_pickups` - Pickup records

**Payment Entities** (Purple):
- `rent_payments` - Payment transactions
- `ledger_entries` - Double-entry accounting
- `razorpay_transactions` - Online payments

**User & Auth** (Red):
- `users` - Employee/admin accounts
- `family_members` - Customer family members

**Logging & Audit** (Gray):
- `entry_events` - Entry lifecycle events
- `admin_action_logs` - Admin action tracking
- `login_logs` - Login history
- `customer_activity_logs` - Customer portal activity

**Advanced Features** (Teal):
- `debt_approval_requests` - Debt approval workflow
- `guard_entries` - Guard register
- `season_change_requests` - Season management
- `pending_setting_changes` - Protected settings (dual approval)

**Relationships**:
- customers → entries (1:M)
- entries → room_entries (1:M)
- customers → gate_passes (1:M)
- entries → gate_passes (1:M)
- gate_passes → gate_pass_pickups (1:M)
- customers → rent_payments (1:M)
- customers → ledger_entries (1:M)

---

### 04-gate-pass-bug-before-fix.png
**Gate Pass Inventory Bug - Before/After Fix**

Visual comparison showing the bug and the fix:

**LEFT - BEFORE FIX (Bug)**:
1. Initial State: 52 items in stock
2. Gate Pass #1: 52 items PENDING → **Not counted in validation** ❌
3. Gate Pass #2: 52 items PENDING → Validation passes (0 allocated) ❌
4. Both Approved
5. Final State: **-52 items in stock** ❌

**RIGHT - AFTER FIX (Correct)**:
1. Initial State: 52 items in stock
2. Gate Pass #1: 52 items PENDING → **Counted! 52 allocated** ✅
3. Gate Pass #2 attempt: **BLOCKED** → Error: Already allocated 52 of 52 items ✅
4. Final State: 0 items remaining (all allocated to GP#1) ✅

**The Fix**: Include 'pending' status in `GetTotalApprovedQuantityForEntry()`

---

### 05-gate-pass-bug-timeline.png
**Gate Pass Inventory Bugs - Complete Timeline**

Shows the actual timeline of what happened with the two bugs:

**13/01/2026 (Day 1)**:
- Gate Pass #191 Created: 52 items, status = PENDING
- BUG #1: Not counted in validation ❌

**15/01/2026 (Day 3)**:
- **SHOULD HAVE EXPIRED!** (30 hours passed)
- BUG #2: ExpireGatePasses() ignores pending status ❌
- GP #191 still PENDING (should be EXPIRED)

**16-19/01** (Days 4-6):
- GP #191 remains PENDING (zombie state)

**20/01/2026 (Day 7)**:
- Gate Pass #728 Created: 52 items, status = PENDING
- Validation: Only counts approved/completed ❌
- GP #191 is pending → NOT COUNTED ❌
- Result: 0 allocated, 52 available → PASS (incorrect) ❌

**Later (both approved)**:
- Both completed
- **RESULT: 104 items out of 52 = -52 INVENTORY** ❌

**AFTER FIX**:
- Fix #1: Count pending in validation
- Fix #2: Expire pending after 30h

---

## Usage in Documentation

These diagrams are referenced in:
- `/docs/SYSTEM_ARCHITECTURE.md` - Main architecture documentation
- `/docs/GATE_PASS_INVENTORY_BUG_FIX.md` - Bug fix documentation

## File Naming Convention

- `01-` to `05-` prefix indicates the order/sequence
- Descriptive names for easy identification
- PNG format for high quality and wide compatibility

## Viewing the Diagrams

You can view these diagrams:
1. **Directly**: Open the PNG files in any image viewer
2. **In IDE**: Most IDEs (VS Code, etc.) can preview PNG files
3. **In Documentation**: Referenced in markdown files with relative paths
4. **In Browser**: Can be viewed in any web browser

## Regenerating Diagrams

If you need to regenerate or update these diagrams, use the architecture documentation generation tooling with the latest system specifications.

## Diagram Sizes

- 01-system-architecture.png: ~817 KB
- 02-component-flow.png: ~607 KB
- 03-database-schema.png: ~734 KB
- 04-gate-pass-bug-before-fix.png: ~645 KB
- 05-gate-pass-bug-timeline.png: ~718 KB

**Total**: ~3.5 MB

---

**Last Updated**: January 20, 2026  
**Version**: 1.0
