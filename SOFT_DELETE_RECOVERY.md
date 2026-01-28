# Soft Delete Recovery Feature

## Overview

A comprehensive soft delete management feature has been implemented. When entries are deleted, they're marked as `status='deleted'` instead of being permanently removed from the database. This feature allows admins to:
- View all soft-deleted entries
- Restore entries to their previous state (active or transferred)
- Permanently delete entries from the database (with double confirmation)

## What Was Added

### 1. New Handler: DeletedEntriesHandler

**File:** `internal/handlers/deleted_entries_handler.go`

**Features:**
- View all soft-deleted entries
- Restore individual entries to their previous status
- Bulk restore multiple entries at once
- Statistics about deleted entries

**Endpoints:**
- `GET /api/admin/deleted-entries` - List all deleted entries
- `GET /api/admin/deleted-entries/stats` - Get deletion statistics
- `POST /api/admin/deleted-entries/{id}/restore` - Restore single entry
- `POST /api/admin/deleted-entries/restore-bulk` - Restore multiple entries
- `DELETE /api/admin/deleted-entries/{id}` - Permanently delete single entry
- `DELETE /api/admin/deleted-entries/bulk` - Permanently delete multiple entries
- `GET /admin/deleted-entries` - HTML page for viewing/restoring/deleting

### 2. New Template: Deleted Entries Page

**File:** `templates/deleted_entries.html`

**Features:**
- Interactive table showing all deleted entries
- Search and filter functionality
- Select all/individual checkboxes
- Single-click restore buttons
- Bulk restore for multiple entries
- Real-time statistics display

### 3. Smart Status Restoration

The system automatically determines the correct previous status:

| Condition | Previous Status |
|-----------|----------------|
| Entry has `transferred_to_customer_id` set | `transferred` |
| Entry doesn't have transfer info | `active` |

This ensures that when you restore an entry, it goes back to the exact state it was in before deletion.

## How It Works

### Entry Status Flow

```
1. Entry created → status='active'
2. Entry transferred → status='transferred', transferred_to_customer_id set
3. Entry deleted → status='deleted', deleted_at timestamp set
4. Entry restored → status='transferred' OR 'active' (based on history)
```

### Database Fields Used

- `status` - Current status ('active', 'transferred', 'deleted')
- `deleted_at` - Timestamp when entry was deleted
- `transferred_to_customer_id` - Indicates if entry was transferred before deletion
- `updated_at` - Auto-updated when status changes

## How to Use

### Access the Page

1. Login as admin at http://localhost:8080
2. Navigate to **Admin** → **Deleted Entries**
3. Or directly visit: http://localhost:8080/admin/deleted-entries

### View Deleted Entries

The page shows:
- Total deleted entries
- Oldest and newest deletion dates
- Full list of deleted entries with details
- Previous status indicator

### Restore Single Entry

1. Find the entry in the list
2. Click the **Restore** button on that row
3. Confirm the restoration
4. Entry is restored to its previous status

### Bulk Restore

1. Check the boxes next to entries you want to restore
2. Or use "Select All" checkbox
3. Click **Restore Selected** button at the top
4. Confirm the bulk restoration
5. All selected entries are restored

### Permanent Delete (CAUTION)

**Warning:** Permanent deletion CANNOT be undone. The entry will be completely removed from the database.

**Single Entry:**
1. Find the entry in the list
2. Click the red **Delete** button on that row
3. Confirm with first warning dialog
4. Confirm again with second warning dialog
5. Entry is permanently removed from database

**Bulk Permanent Delete:**
1. Check the boxes next to entries you want to permanently delete
2. Click red **Permanent Delete** button at the top
3. Confirm with first warning dialog
4. Confirm again with second warning dialog
5. All selected entries are permanently removed

**Safety Features:**
- Double confirmation required for all permanent deletions
- Can only permanently delete entries that are already soft-deleted
- All permanent deletions are logged in audit trail
- Separate red buttons to distinguish from restore (green) actions

### Search and Filter

- **Search box**: Filter by name, phone, or thock number
- **Category filter**: Filter by seed/sell
- **Previous status filter**: See only entries that were active or transferred

## API Usage

### Get Deleted Entries List

```bash
curl -X GET http://localhost:8080/api/admin/deleted-entries \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Response:
```json
{
  "success": true,
  "entries": [
    {
      "id": 2173,
      "customer_id": 123,
      "name": "Lakshya",
      "phone": "8650996363",
      "thock_number": "2471/200",
      "expected_quantity": 200,
      "status": "deleted",
      "previous_status": "active",
      "deleted_at": "2026-01-28T13:08:00Z",
      "created_at": "2026-01-27T11:01:50Z"
    }
  ],
  "count": 1
}
```

### Get Statistics

```bash
curl -X GET http://localhost:8080/api/admin/deleted-entries/stats \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Response:
```json
{
  "success": true,
  "stats": {
    "total_deleted": 38,
    "with_timestamp": 15,
    "oldest_deletion": "2026-01-03T18:35:35Z",
    "newest_deletion": "2026-01-28T13:08:00Z"
  }
}
```

### Restore Single Entry

```bash
curl -X POST http://localhost:8080/api/admin/deleted-entries/2173/restore \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Response:
```json
{
  "success": true,
  "message": "Entry restored successfully",
  "entry_id": 2173,
  "previous_status": "active"
}
```

### Bulk Restore

```bash
curl -X POST http://localhost:8080/api/admin/deleted-entries/restore-bulk \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entry_ids": [2173, 2172, 2171, 2170, 2169]
  }'
```

Response:
```json
{
  "success": true,
  "message": "Entries restored successfully",
  "restored_count": 5,
  "total_requested": 5
}
```

### Permanent Delete Single Entry (CAUTION)

**Warning:** This permanently removes the entry from the database. Cannot be undone.

```bash
curl -X DELETE http://localhost:8080/api/admin/deleted-entries/2173 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Response:
```json
{
  "success": true,
  "message": "Entry permanently deleted",
  "entry_id": 2173
}
```

### Bulk Permanent Delete (CAUTION)

**Warning:** This permanently removes multiple entries from the database. Cannot be undone.

```bash
curl -X DELETE http://localhost:8080/api/admin/deleted-entries/bulk \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "entry_ids": [2173, 2172, 2171]
  }'
```

Response:
```json
{
  "success": true,
  "message": "Entries permanently deleted",
  "deleted_count": 3,
  "total_requested": 3
}
```

## Current Deleted Entries

As of deployment, there are:
- **38 deleted entries** in the database
- **Most recent deletion**: Entry #2173 (Lakshya, 2471/200)
- **Oldest deletion**: Entry #2164 (Jan 3, 2026)

Recently deleted entries that can be restored:
- Entry #2173: Lakshya, 2471/200, 200 qty (deleted recently)
- Entry #2172: Lakshya, 1327/1, 1 qty
- Entry #2171: Lakshya, 1326/1, 1 qty
- Entry #2170: Lakshya, 1325/23, 23 qty
- Entry #2169: Lakshya, 2470/22, 22 qty

## Database Query Examples

### See All Deleted Entries

```sql
SELECT id, name, phone, thock_number, expected_quantity, status, deleted_at
FROM entries
WHERE status = 'deleted'
ORDER BY deleted_at DESC;
```

### Manually Restore Entry

```sql
UPDATE entries
SET status = 'active',
    deleted_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = 2173;
```

### Restore with Previous Status Detection

```sql
UPDATE entries
SET status = CASE
    WHEN transferred_to_customer_id IS NOT NULL THEN 'transferred'
    ELSE 'active'
END,
deleted_at = NULL,
updated_at = CURRENT_TIMESTAMP
WHERE id = 2173;
```

## Security

- **Admin-only access**: All endpoints require admin role
- **Authentication required**: JWT token must be provided
- **Audit logging**: All restore actions are logged in `admin_action_logs`
- **Transaction safety**: Bulk operations use database transactions

## Benefits

1. **No Data Loss**: Deleted entries are preserved, not permanently removed
2. **Easy Recovery**: Simple UI to restore accidentally deleted entries
3. **Bulk Operations**: Restore or permanently delete multiple entries at once
4. **Audit Trail**: All actions (restore and permanent delete) are logged with user ID and timestamp
5. **Smart Restore**: Entries return to their exact previous state (active or transferred)
6. **Search & Filter**: Quickly find specific deleted entries
7. **Permanent Cleanup**: Option to permanently delete old soft-deleted entries when needed
8. **Safety First**: Double confirmation required for all permanent deletions

## Technical Details

### Files Modified

- `internal/handlers/deleted_entries_handler.go` (NEW)
- `templates/deleted_entries.html` (NEW)
- `internal/http/router.go` (added routes)
- `cmd/server/main.go` (added handler initialization)

### Dependencies

- Existing `entries` table structure
- `admin_action_logs` table for audit trail
- No new database migrations required

### Performance

- Queries are limited to 100 most recent deletions
- Indexes on `status` field optimize filtering
- Bulk operations use single transaction for consistency

---

## Quick Test

1. Open http://localhost:8080/admin/deleted-entries
2. You should see 38 deleted entries
3. Click "Restore" on any entry
4. Entry moves back to active status
5. Check the main entries list - restored entry appears there

**The feature is now live and ready to use!** 🎉
