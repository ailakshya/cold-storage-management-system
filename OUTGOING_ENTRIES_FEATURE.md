# Outgoing Entries Display in Account Management

## Feature Description

Added display of **outgoing entries** (completed gate passes) in the "All Trucks for customer" section of the Account Management page with distinct RED color coding.

## What Changed

### Before:
- Only showed **incoming entries** (trucks stored in warehouse)
- Totals showed only incoming quantities
- No visibility of items taken out via gate passes

### After:
- Shows **both incoming AND outgoing entries**
- Color-coded for easy distinction:
  - **GREEN rows** → Incoming (SEED trucks ≤1500)
  - **ORANGE rows** → Incoming (SELL trucks >1500)
  - **RED rows** → Outgoing (completed gate passes) ⬅️
- Separate totals for incoming, outgoing, and net inventory
- Visual indicators: ➡️ IN for incoming, ⬅️ OUT for outgoing

## Color Logic

### Incoming Entries (➡️ IN):
```
Truck ≤ 1500  → GREEN color (SEED category)
Truck > 1500  → ORANGE color (SELL category)
```

### Outgoing Entries (⬅️ OUT):
```
All outgoing → RED background + RED text
- RED-50 background
- RED-500 left border (4px thick)
- RED-700 text color
```

## Display Format

### Trucks Table:

```
┌──────────────────┬──────────┬──────────┬────────────┐
│ Truck Number     │ Quantity │ Rent     │ Date       │
├──────────────────┼──────────┼──────────┼────────────┤
│ 1200/100 ➡️ IN   │ 100      │ ₹500     │ 15/12/2025 │ (Green - SEED IN)
│ 1600/200 ➡️ IN   │ 200      │ ₹1000    │ 15/12/2025 │ (Orange - SELL IN)
│ 1513/299 ⬅️ OUT  │ 30       │ ₹150     │ 15/12/2025 │ (RED - OUT)
├──────────────────┼──────────┼──────────┼────────────┤
│ SEED TOTAL       │ 100      │ ₹500     │ -          │ (Green total)
│ SELL TOTAL       │ 200      │ ₹1000    │ -          │ (Orange total)
│ ⬅️ OUTGOING TOTAL│ 30       │ ₹150     │ -          │ (RED total)
│ NET TOTAL        │ 270      │ ₹1350    │ -          │ (Black - IN-OUT)
└──────────────────┴──────────┴──────────┴────────────┘
```

## Implementation Details

### Backend Changes:
None! Uses existing `/api/gate-passes` endpoint

### Frontend Changes:
File: `/templates/rent_management.html`

#### 1. Fetch Completed Gate Passes:
```javascript
const gatePassesResponse = await fetch('/api/gate-passes?_=' + Date.now());
const gatePasses = await gatePassesResponse.json();
allGatePasses = gatePasses.filter(gp => gp.status === 'completed');
```

#### 2. Add to Customer Data:
```javascript
// Add outgoing entries
allGatePasses.forEach(gatePass => {
    const phone = gatePass.customer_phone;
    const customer = customerMap.get(phone);

    if (customer) {
        const quantity = gatePass.total_picked_up || gatePass.requested_quantity || 0;
        const rent = quantity * rentPerItem;

        customer.trucks.push({
            id: gatePass.id,
            truck_number: gatePass.truck_number,
            quantity: -quantity,  // Negative for outgoing
            rent: -rent,
            date: new Date(gatePass.completed_at).toLocaleDateString('en-IN'),
            type: 'outgoing'
        });

        customer.totalOutgoing += quantity;
        customer.outgoingRent += rent;
    }
});
```

#### 3. Display with Color Coding:
```javascript
const isOutgoing = truck.type === 'outgoing';
const rowClass = isOutgoing ? 'bg-red-50 border-l-4 border-red-500' : '';
const truckColorClass = isOutgoing ? 'text-red-700' : getTruckColor(truck.truck_number);
const typeLabel = isOutgoing ? ' ⬅️ OUT' : ' ➡️ IN';
```

#### 4. New Totals Row:
```html
<tr class="bg-red-100 font-bold border-t-2 border-red-500">
    <td class="p-3 text-red-700">⬅️ OUTGOING TOTAL</td>
    <td class="p-3 text-red-700">${customer.totalOutgoing}</td>
    <td class="p-3 text-red-700">₹${customer.outgoingRent}</td>
    <td class="p-3">-</td>
</tr>

<tr class="bg-gray-800 text-white font-bold border-t-4 border-black">
    <td class="p-3">NET TOTAL (IN - OUT)</td>
    <td class="p-3">${customer.totalQuantity - customer.totalOutgoing}</td>
    <td class="p-3">₹${(customer.totalRent - customer.outgoingRent)}</td>
    <td class="p-3">-</td>
</tr>
```

## Customer Data Structure

```javascript
{
    name: "Customer Name",
    phone: "9876543210",
    trucks: [
        {
            truck_number: "1200/100",
            quantity: 100,
            rent: 500,
            date: "15/12/2025",
            type: "incoming"  // ➡️ IN
        },
        {
            truck_number: "1513/299",
            quantity: -30,     // Negative for outgoing
            rent: -150,
            date: "15/12/2025",
            type: "outgoing"   // ⬅️ OUT
        }
    ],
    totalQuantity: 100,      // Total IN
    totalRent: 500,
    totalOutgoing: 30,       // Total OUT
    outgoingRent: 150,
    netQuantity: 70          // IN - OUT
}
```

## Visual Examples

### Example 1: Customer with Outgoing
```
Customer: Rakesh (9917585586)

All Trucks:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
609/200 ➡️ IN     | 200 | ₹1000 | 14/12/2025
1513/299 ➡️ IN    | 299 | ₹1495 | 15/12/2025
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[RED] 1513/299 ⬅️ OUT | 30  | ₹150  | 15/12/2025
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
SELL TOTAL        | 499 | ₹2495 | -
⬅️ OUTGOING TOTAL | 30  | ₹150  | - (RED)
NET TOTAL         | 469 | ₹2345 | -
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Example 2: Customer with Multiple Outgoing
```
Customer: Manoj Saini (9027750773)

All Trucks:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1200/100 ➡️ IN        | 100 | ₹500  | 10/12/2025 (GREEN)
1600/200 ➡️ IN        | 200 | ₹1000 | 11/12/2025 (ORANGE)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[RED] 1200/100 ⬅️ OUT | 50  | ₹250  | 12/12/2025
[RED] 1600/200 ⬅️ OUT | 100 | ₹500  | 13/12/2025
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
SEED TOTAL            | 100 | ₹500  | - (GREEN)
SELL TOTAL            | 200 | ₹1000 | - (ORANGE)
⬅️ OUTGOING TOTAL     | 150 | ₹750  | - (RED)
NET TOTAL (IN - OUT)  | 150 | ₹750  | -
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## Benefits

1. **Complete Inventory Tracking** - See both incoming and outgoing in one place
2. **Visual Clarity** - Color coding makes it easy to distinguish entry types
3. **Accurate Accounting** - Net totals show actual inventory after withdrawals
4. **Audit Trail** - Full history of all transactions (in and out)
5. **Balance Calculation** - Rent calculated correctly based on net inventory

## Files Modified

- `/templates/rent_management.html`
  - Added gate passes fetch
  - Added outgoing entries to customer trucks
  - Updated display with color coding
  - Added outgoing total row
  - Changed grand total to "NET TOTAL (IN - OUT)"

## Testing

To verify:
1. Go to Account Management page (`/rent-management`)
2. Find customer with completed gate passes
3. Click "Show Trucks"
4. Verify:
   - ✅ Outgoing entries shown in RED
   - ✅ Arrow indicators (➡️ IN, ⬅️ OUT)
   - ✅ Outgoing total row in RED
   - ✅ Net total = Incoming - Outgoing

---

**Status:** ✅ Completed and Deployed
**Date:** December 14, 2025
**Impact:** High - Complete inventory visibility for accountants
