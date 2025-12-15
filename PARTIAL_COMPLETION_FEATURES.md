# Gate Pass Partial Completion System - Feature Summary

## Overview
Implemented a comprehensive partial completion system for gate passes with multiple pickups, 15-hour approval window, and automatic expiration.

## Features Implemented

### 1. Database Schema
- **gate_passes table** - Added columns:
  - `total_picked_up` - Tracks total quantity picked up
  - `approval_expires_at` - 15-hour expiration timestamp set on approval
  - `final_approved_quantity` - Final quantity when expired

- **gate_pass_pickups table** - New audit table:
  - Records each pickup with quantity, room, floor, user, and timestamp
  - Full audit trail for all pickup activities

### 2. Gate Pass Workflow States
1. **pending** → Initial state (30-hour expiration)
2. **approved** → After admin approval (15-hour window starts)
3. **partially_completed** → Some items picked up (< requested quantity)
4. **completed** → All items picked up (= requested quantity)
5. **expired** → 15-hour window exceeded, remaining items stay in inventory

### 3. API Endpoints
- `POST /api/gate-passes` - Create gate pass
- `PUT /api/gate-passes/{id}/approve` - Approve and start 15-hour window
- `POST /api/gate-passes/pickup` - Record partial pickup
- `GET /api/gate-passes/{id}/pickups` - View pickup history
- `GET /api/gate-passes/approved` - List approved/partially_completed passes
- `GET /api/gate-passes/expired` - List expired passes (last 7 days)

### 4. Frontend Features

#### Unloading Tickets Page (/unloading-tickets)
**Pending Gate Passes:**
- Removed Payment column
- Added location display: Room, Floor, Gatar
- Room-config-1 style location boxes with red highlighting
- Color-coded Thock numbers (green ≤1500, orange >1500)
- 30-hour expiration timer with color coding

**Approval Form:**
- Compact room/floor grid selection
- Red highlighting for selected locations
- Gatar number display

**Pickup Recording Section:**
- List of approved/partially_completed gate passes
- Shows picked/total progress (e.g., "60/100")
- 15-hour countdown timer with color coding:
  - Green: >12 hours
  - Yellow: 6-12 hours
  - Orange: 3-6 hours
  - Red: <3 hours
- Pickup form with quantity, room, floor inputs
- Pickup history display with timestamps and user names
- Real-time inventory updates

#### Admin Report Page (/admin/report)
**New Tab: Expired Gate Passes**
- RED color scheme (background, borders, text)
- Shows expired passes from last 7 days
- Displays:
  - Thock NO (color-coded)
  - Customer name and phone
  - Requested quantity
  - Picked up quantity
  - Remaining quantity (in RED)
  - Final approved quantity
  - Approval expired timestamp
  - Status updated timestamp
- Search/filter functionality

**Updated Employee Activity:**
- Added "Gate Passes Issued" column
- Added "Gate Passes Approved" column
- Employee ID tracking

**Updated Gate Pass Logs:**
- Issued by employee with ID
- Approved by employee with ID
- Expires In countdown timer

### 5. Backend Logic

#### Partial Pickup Flow:
```
1. Admin approves gate pass → approval_expires_at = NOW + 15 hours
2. Customer picks up partial quantity → record in gate_pass_pickups
3. Update total_picked_up in gate_passes
4. If total_picked_up >= requested_quantity → status = 'completed'
5. If 0 < total_picked_up < requested_quantity → status = 'partially_completed'
```

#### Inventory Management:
- On each pickup, room_entries.quantity is reduced
- Validates sufficient quantity before pickup
- Prevents over-pickup beyond approved quantity

#### Expiration Logic:
```
1. Expiration check runs when /approved endpoint is called
2. For each gate pass with approval_expires_at < NOW:
   - Set status = 'expired'
   - Set final_approved_quantity = total_picked_up
   - Remaining items stay in inventory (not deducted)
```

### 6. Admin Action Logging
All gate pass actions logged with:
- CREATE - Gate pass issued
- UPDATE - Gate pass approved
- PICKUP - Partial pickup recorded
- COMPLETE - Gate pass completed
- User ID, IP address, description, timestamp

## Test Results

All scenarios tested and verified:
✓ Gate pass creation with pending status
✓ Approval sets 15-hour expiration window
✓ Multiple partial pickups recorded correctly
✓ Pickup history tracking with audit trail
✓ Auto-completion when full quantity picked up
✓ Approved passes listing with remaining time
✓ Expired passes endpoint returns last 7 days
✓ Inventory reduction on each pickup
✓ Employee activity tracking

## UI Color Coding

**Thock Numbers:**
- Green: ≤1500
- Orange: >1500

**Expiration Timers (30-hour pending):**
- Green: >24 hours
- Yellow: 12-24 hours
- Orange: 6-12 hours
- Red: <6 hours

**Expiration Timers (15-hour approval):**
- Green: >12 hours
- Yellow: 6-12 hours
- Orange: 3-6 hours
- Red: <3 hours

**Expired Gate Passes Tab:**
- RED background (#FEE)
- RED borders
- RED text highlights
- Red warning icon

## Files Modified

**Backend:**
- `/internal/db/postgres.go` - Database migrations
- `/internal/models/gate_pass.go` - Updated model
- `/internal/models/gate_pass_pickup.go` - New model
- `/internal/repositories/gate_pass_repository.go` - Updated methods
- `/internal/repositories/gate_pass_pickup_repository.go` - New repository
- `/internal/repositories/room_entry_repository.go` - Added ReduceQuantity
- `/internal/services/gate_pass_service.go` - New methods
- `/internal/handlers/gate_pass_handler.go` - New handlers
- `/internal/http/router.go` - New routes
- `/cmd/server/main.go` - Service initialization

**Frontend:**
- `/templates/unloading_tickets.html` - Major updates
- `/templates/admin_report.html` - Added expired tab

## Usage Example

```bash
# 1. Create gate pass (30-hour expiration)
POST /api/gate-passes
{
  "customer_id": 1,
  "truck_number": "TEST001",
  "requested_quantity": 150,
  "payment_verified": true,
  "payment_amount": 7500
}

# 2. Approve gate pass (15-hour window starts)
PUT /api/gate-passes/3/approve
{
  "approved_quantity": 150,
  "gate_no": "Gate A",
  "status": "approved"
}

# 3. Record first pickup (100/150)
POST /api/gate-passes/pickup
{
  "gate_pass_id": 3,
  "pickup_quantity": 100,
  "room_no": "R1",
  "floor": "F1",
  "remarks": "First batch"
}
# Status: partially_completed, total_picked_up: 100

# 4. Record second pickup (50/150)
POST /api/gate-passes/pickup
{
  "gate_pass_id": 3,
  "pickup_quantity": 50,
  "room_no": "R2",
  "floor": "F2",
  "remarks": "Final batch"
}
# Status: completed (auto-completed), total_picked_up: 150

# 5. View pickup history
GET /api/gate-passes/3/pickups
# Returns all pickups with timestamps and user names

# 6. Check expired passes (admin report)
GET /api/gate-passes/expired
# Returns passes that expired in last 7 days
```

## Future Enhancements (Optional)

1. **Fundas Calculation** - Calculate payment based on actual picked up quantity vs requested
2. **SMS Notifications** - Alert customers when approval is about to expire
3. **Automatic Cleanup** - Archive old expired passes after 30 days
4. **Reports** - Generate monthly reports of partial completions and expirations
5. **Dashboard Metrics** - Show expiration rate and average pickup time

---

**Status:** ✅ Fully Implemented and Tested
**Date:** December 14, 2025
**Version:** 1.0
