# Pickup Form Update - Room-Config-1 Style Display

## Changes Made

Updated the pickup recording form in `/templates/unloading_tickets.html` to display storage location using the same Approval Form format with room-config-1 style grid layout.

## What Changed

### 1. Visual Location Display
**Before:**
- Simple text input fields for Room No and Floor
- No visual representation of location

**After:**
- Room-config-1 style grid layout with visual boxes
- RED highlighting for the selected room and floor
- Gatar number display
- Same format as Approval Form

### 2. Location Grid Layout

#### Room Display (Left Side):
```
┌─────────┬─────────┐
│ Room 2  │ Room 1  │
├─────────┴─────────┤
│      Gallery      │
├─────────┬─────────┤
│ Room 3  │ Room 4  │
└─────────┴─────────┘
```

#### Floor Display (Right Side):
```
┌─────┐
│  5  │
├─────┤
│  4  │
├─────┤
│  3  │
├─────┤
│  2  │
├─────┤
│  1  │
└─────┘
```

### 3. RED Color Highlighting
When a gate pass is selected for pickup:
- The system fetches room_entries for the truck number
- The corresponding room box is highlighted in RED
- The corresponding floor box is highlighted in RED
- Gatar number is displayed

**Color Scheme:**
- Default: Gray background (`bg-gray-100`), Gray text (`text-gray-400`)
- Selected: RED background (`bg-red-500`), White text (`text-white`), Red border (`border-red-700`)

### 4. Automatic Location Detection

The `selectForPickup()` function now:
1. Fetches all room entries via `/api/room-entries`
2. Filters entries matching the gate pass truck number
3. Extracts room_no, floor, and gate_no from first entry
4. Highlights the corresponding boxes in RED
5. Fills hidden input fields with room/floor values
6. Displays gatar number

### 5. Form Structure

```html
Pickup Form
├── Thock NO Display (Orange box)
├── Quantity Summary (Blue box)
│   ├── Requested
│   ├── Already Picked
│   └── Remaining
├── Storage Location Display (White box) ← NEW!
│   ├── Room Grid (with RED highlighting)
│   ├── Floor Grid (with RED highlighting)
│   └── Gatar Display
├── Pickup Quantity Input
├── Remarks Input
└── Action Buttons
    ├── Record Pickup
    └── Cancel
```

## Code Changes

### HTML Structure
- Added room-config-1 style grid layout
- Small boxes with `py-1` padding and `text-xs` font
- Classes: `pickup-location-box`, `pickup-room-{X}`, `pickup-floor-{X}`
- Hidden inputs for `pickupRoomNo` and `pickupFloor`

### JavaScript Updates

#### `selectForPickup()` - Now async
```javascript
async function selectForPickup(gatePass) {
    // ... existing code ...

    // Reset all location boxes
    document.querySelectorAll('.pickup-location-box').forEach(box => {
        box.classList.remove('bg-red-500', 'text-white', 'border-red-700');
        box.classList.add('bg-gray-100', 'text-gray-400');
    });

    // Fetch room entries and highlight
    const response = await fetch('/api/room-entries', ...);
    const truckEntries = allRoomEntries.filter(re => re.truck_number === gatePass.truck_number);

    // Highlight room box in RED
    const roomBox = document.querySelector(`.pickup-room-${roomNo}`);
    roomBox.classList.add('bg-red-500', 'text-white', 'border-red-700');

    // Highlight floor box in RED
    const floorBox = document.querySelector(`.pickup-floor-${floor}`);
    floorBox.classList.add('bg-red-500', 'text-white', 'border-red-700');
}
```

#### `clearPickupSelection()` - Updated
```javascript
function clearPickupSelection() {
    // ... existing code ...

    // Reset all location boxes to default gray
    document.querySelectorAll('.pickup-location-box').forEach(box => {
        box.classList.remove('bg-red-500', 'text-white', 'border-red-700');
        box.classList.add('bg-gray-100', 'text-gray-400');
    });
    document.getElementById('pickupGatar').textContent = '-';
}
```

## User Flow

1. User clicks on an approved gate pass in the left table
2. Pickup form opens on the right
3. **Location boxes automatically highlight in RED** showing where items are stored
4. User sees:
   - Room highlighted in RED (e.g., Room 1)
   - Floor highlighted in RED (e.g., Floor 2)
   - Gatar number displayed (e.g., Gate-A-2)
5. User enters pickup quantity
6. User clicks "Record Pickup"
7. System records pickup with room/floor automatically filled

## Benefits

1. **Visual Consistency** - Same format as Approval Form
2. **Better UX** - Clear visual indication of storage location
3. **Error Reduction** - No manual typing of room/floor needed
4. **Professional Look** - Compact, grid-based design
5. **Color Coding** - RED highlighting makes location obvious

## Example Scenario

**Gate Pass:** TEST001
**Stored At:** Room 1, Floor 2, Gatar: Gate-A-2

**Visual Display:**
```
Room No:
┌─────────┬─────────┐
│ Room 2  │ Room 1  │ ← RED
├─────────┴─────────┤
│      Gallery      │
├─────────┬─────────┤
│ Room 3  │ Room 4  │
└─────────┴─────────┘

Floor:
┌─────┐
│  5  │
├─────┤
│  4  │
├─────┤
│  3  │
├─────┤
│  2  │ ← RED
├─────┤
│  1  │
└─────┘

Gatar: Gate-A-2
```

## Files Modified

- `/templates/unloading_tickets.html`
  - Updated pickup form HTML structure
  - Added location grid display
  - Updated `selectForPickup()` function (now async)
  - Updated `clearPickupSelection()` function

## Testing

Server restarted and verified:
- ✅ Approved gate passes endpoint working
- ✅ Pickup form displays correctly
- ✅ Location grid renders properly
- ✅ RED highlighting works when gate pass is selected
- ✅ Form submission includes room/floor data

---

**Status:** ✅ Completed and Deployed
**Date:** December 14, 2025
