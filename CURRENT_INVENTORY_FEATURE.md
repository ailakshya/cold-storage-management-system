# Current Inventory Per Truck Feature

## Feature Description

Added a **separate section** in Account Management showing the actual current quantity of items in each truck after accounting for outgoing entries (completed gate passes).

## What It Shows

For each truck with outgoing entries, displays:
- **Original Quantity** - Total items originally stored (incoming)
- **Outgoing Quantity** - Total items withdrawn via gate passes
- **Current Quantity** - Actual items remaining (Original - Outgoing)
- **Current Rent** - Rent for remaining items

## Visual Design

### Section Header
```
📦 Current Inventory Per Truck (After Outgoing)
```
- BLUE color scheme (bg-blue-100, border-blue-500)
- Displayed ABOVE the "All Trucks" detailed transaction list
- Only shown if customer has outgoing entries

### Table Columns

| Truck Number | Original | Outgoing | Current | Current Rent |
|--------------|----------|----------|---------|--------------|
| 1513/299 (ORANGE) | 299 | 30 | **269** | ₹1345 |
| 609/200 (ORANGE) | 200 | 100 | **100** | ₹500 |
| 1200/100 (GREEN) | 100 | 100 | ~~0 (Empty)~~ | ₹0 |

### Color Coding

1. **Truck Number** - Same color logic as incoming entries:
   - GREEN (1-1500) = SEED trucks
   - ORANGE (1501-3000) = SELL trucks

2. **Outgoing Column** - RED-600 bold text

3. **Current Quantity**:
   - **Blue bold** if reduced (current < original)
   - **Gray strikethrough** if empty (current = 0)
   - Shows "(Empty)" label if fully withdrawn

4. **Row Background**:
   - GRAY-50 if truck is empty
   - WHITE if truck has remaining items

## Example Display

### Customer: Rakesh (9917585586)

```
┌─────────────────────────────────────────────────────────────────┐
│ 📦 Current Inventory Per Truck (After Outgoing)                │
├────────────────┬──────────┬──────────┬──────────┬──────────────┤
│ Truck Number   │ Original │ Outgoing │ Current  │ Current Rent │
├────────────────┼──────────┼──────────┼──────────┼──────────────┤
│ 609/200        │ 200      │ 0        │ 200      │ ₹1000        │ (No change)
│ 1513/299       │ 299      │ 30       │ 269      │ ₹1345        │ (Reduced - BLUE)
└────────────────┴──────────┴──────────┴──────────┴──────────────┘
```

### Customer: Manoj Saini (Multiple Withdrawals)

```
┌─────────────────────────────────────────────────────────────────┐
│ 📦 Current Inventory Per Truck (After Outgoing)                │
├────────────────┬──────────┬──────────┬──────────┬──────────────┤
│ Truck Number   │ Original │ Outgoing │ Current  │ Current Rent │
├────────────────┼──────────┼──────────┼──────────┼──────────────┤
│ 1200/100       │ 100      │ 100      │ 0 (Empty)│ ₹0           │ (Empty - GRAY)
│ 1600/200       │ 200      │ 100      │ 100      │ ₹500         │ (Reduced - BLUE)
└────────────────┴──────────┴──────────┴──────────┴──────────────┘
```

## Implementation Details

### Location
File: `/templates/rent_management.html`

Inserted **before** the "All Trucks" section inside the collapsible truck list.

### Logic

```javascript
// Group trucks by truck_number
const truckInventory = new Map();

customer.trucks.forEach(truck => {
    if (!truckInventory.has(truck.truck_number)) {
        truckInventory.set(truck.truck_number, {
            truck_number: truck.truck_number,
            incoming: 0,
            outgoing: 0,
            incomingRent: 0,
            outgoingRent: 0
        });
    }

    const inv = truckInventory.get(truck.truck_number);
    if (truck.type === 'incoming') {
        inv.incoming += truck.quantity;
        inv.incomingRent += truck.rent;
    } else {
        inv.outgoing += Math.abs(truck.quantity);
        inv.outgoingRent += Math.abs(truck.rent);
    }
});

// Calculate current = incoming - outgoing
const current = inv.incoming - inv.outgoing;
const currentRent = inv.incomingRent - inv.outgoingRent;
```

### Conditional Display

- **Only shown** if customer has at least one outgoing entry
- If no outgoing entries, section is hidden (empty string returned)

```javascript
const hasOutgoing = inventoryArray.some(inv => inv.outgoing > 0);

return hasOutgoing ? `<div>...</div>` : '';
```

## Page Layout (After Clicking "Show Trucks")

```
┌───────────────────────────────────────────────────────────────────┐
│ 📦 Current Inventory Per Truck (After Outgoing) [NEW SECTION]    │
│ ─────────────────────────────────────────────────────────────────│
│ Shows per-truck inventory after withdrawals                       │
└───────────────────────────────────────────────────────────────────┘
          ↓
┌────────────────────────────┬──────────────────────────────────────┐
│ 🚛 All Trucks              │ 🧾 Payment History                   │
│ (Detailed transaction list)│ (Payment collection records)         │
│ - Incoming (➡️ IN)          │ - Receipt history                    │
│ - Outgoing (⬅️ OUT)         │ - Employee who collected             │
│ - Totals                   │                                      │
└────────────────────────────┴──────────────────────────────────────┘
```

## Benefits

1. **At-a-glance Inventory** - Quickly see what's left in each truck
2. **Empty Detection** - Easily identify fully withdrawn trucks
3. **Rent Calculation** - Shows rent only for remaining items
4. **Audit Trail** - Track original vs current quantities
5. **Separate from Transactions** - Summary view before detailed list

## Use Cases

### Use Case 1: Check Available Items
**Scenario**: Customer wants to withdraw more items
**Solution**: Look at "Current" column to see what's available per truck

### Use Case 2: Identify Empty Trucks
**Scenario**: Need to know which trucks are fully cleared
**Solution**: Look for gray rows with "(Empty)" label

### Use Case 3: Calculate Actual Rent
**Scenario**: Customer paid for original quantity but withdrew some
**Solution**: "Current Rent" shows rent for items still in storage

### Use Case 4: Verify Withdrawals
**Scenario**: Customer disputes how much was withdrawn
**Solution**: Compare Original vs Outgoing vs Current columns

## Files Modified

- `/templates/rent_management.html`
  - Added `truckInventory` map to group by truck_number
  - Added calculation for incoming/outgoing per truck
  - Added new table section with blue color scheme
  - Conditional rendering based on outgoing entries

## Testing

To verify the feature:

1. Go to Account Management (`/rent-management`)
2. Find a customer with completed gate passes
3. Click "Show Trucks"
4. Verify:
   - ✅ "Current Inventory Per Truck" section appears at top
   - ✅ Shows Original, Outgoing, Current columns
   - ✅ Current = Original - Outgoing
   - ✅ Empty trucks shown with strikethrough and gray background
   - ✅ Reduced quantities shown in blue bold
   - ✅ Section hidden if no outgoing entries

## Example Data

### Before Withdrawals:
```
Truck 1513/299: 299 items in storage
```

### After Partial Withdrawal:
```
Original: 299
Outgoing: 30 (withdrawn via gate pass)
Current: 269 (still in storage)
Current Rent: ₹1345 (for 269 items only)
```

### After Full Withdrawal:
```
Original: 100
Outgoing: 100 (fully withdrawn)
Current: 0 (Empty) - shown with strikethrough
Current Rent: ₹0
```

---

**Status:** ✅ Completed and Deployed
**Date:** December 15, 2025
**Impact:** High - Clear visibility of actual current inventory per truck
**Related Features:**
- Outgoing Entries Display (OUTGOING_ENTRIES_FEATURE.md)
- Inventory Bug Fix (INVENTORY_BUG_FIX.md)
