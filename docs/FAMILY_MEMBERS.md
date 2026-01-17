# Family Members Documentation

Complete guide for the family member management system.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The Family Members feature allows customers to link family members who can act on their behalf for gate passes, payments, and entry viewing.

**Key Features:**
- Link multiple family members to customer account
- Family members can request gate passes
- Family members can make payments
- Relation tracking (son, daughter, brother, etc.)
- Separate phone number for each member

---

## Use Cases

### Common Scenarios

**1. Elderly Customer**
- Customer is elderly and cannot visit facility
- Son handles all gate passes and payments
- Son linked as family member

**2. Busy Farmer**
- Farmer stores produce but is busy in fields
- Brother manages pickups and payments
- Brother linked as family member

**3. Joint Family**
- Multiple family members share stored goods
- Each can independently request pickups
- All linked to same customer account

---

## Features

### 1. Add Family Member

**Endpoint:** `POST /api/customers/:customerId/family-members`

**Request:**
```json
{
  "name": "रमेश कुमार",
  "phone": "8888888888",
  "relation": "son",
  "notes": "Eldest son, handles all transactions"
}
```

**Response:**
```json
{
  "id": 15,
  "customer_id": 5,
  "name": "रमेश कुमार",
  "phone": "8888888888",
  "relation": "son",
  "notes": "Eldest son, handles all transactions",
  "created_at": "2026-01-17T10:30:00Z"
}
```

**Relation Types:**
- `son` - Son (बेटा)
- `daughter` - Daughter (बेटी)
- `brother` - Brother (भाई)
- `sister` - Sister (बहन)
- `father` - Father (पिता)
- `mother` - Mother (माता)
- `wife` - Wife (पत्नी)
- `husband` - Husband (पति)
- `other` - Other relation

### 2. List Family Members

**Endpoint:** `GET /api/customers/:customerId/family-members`

**Response:**
```json
{
  "family_members": [
    {
      "id": 15,
      "name": "रमेश कुमार",
      "phone": "8888888888",
      "relation": "son",
      "total_gate_passes": 12,
      "total_payments": 5,
      "last_activity": "2026-01-15T14:20:00Z"
    }
  ]
}
```

### 3. Update Family Member

**Endpoint:** `PUT /api/family-members/:id`

**Request:**
```json
{
  "name": "रमेश कुमार शर्मा",
  "phone": "8888888888",
  "notes": "Updated information"
}
```

### 4. Delete Family Member

**Endpoint:** `DELETE /api/family-members/:id`

**Note:** Deleting a family member does NOT delete their historical gate passes or payments.

---

## Integration with Other Features

### Gate Passes

When a family member requests a gate pass:

**Request:**
```json
{
  "entry_id": 123,
  "requested_quantity": 100,
  "family_member_id": 15,
  "notes": "Pickup for sale"
}
```

**Benefits:**
- Tracks who requested the gate pass
- Customer sees family member name on gate pass
- Audit trail maintained

**Gate Pass Display:**
```
Gate Pass #456
Entry: #123
Customer: राज कुमार
Requested By: रमेश कुमार (son)
Quantity: 100 quintals
```

### Payments

Family members can make payments on behalf of customer:

**Request:**
```json
{
  "entry_id": 123,
  "amount_paid": 2000,
  "family_member_id": 15,
  "payment_method": "cash"
}
```

**Receipt Shows:**
```
Receipt #789
Customer: राज कुमार
Paid By: रमेश कुमार (son)
Amount: ₹2000
```

### Customer Portal

**Family Member Login:**
- Family members can login to customer portal using their own phone
- See only entries related to linked customer
- Can request gate passes
- Can make online payments
- Dashboard shows customer name

**Access Control:**
- Family members see same data as customer
- Cannot modify customer details
- Cannot add/remove other family members
- Can only act on behalf of customer

---

## Database Schema

### family_members Table

```sql
CREATE TABLE family_members (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    relation TEXT,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_family_members_customer_id ON family_members(customer_id);
CREATE INDEX idx_family_members_phone ON family_members(phone);
```

**Relation to Other Tables:**
- `gate_passes.family_member_id` - Links gate pass to family member
- `rent_payments.family_member_id` - Links payment to family member

---

## Workflows

### Adding Family Member

**Admin/Accountant Workflow:**
1. Open customer profile
2. Click "Add Family Member"
3. Enter family member details
4. Save
5. Family member can now act on behalf

**Validation:**
- Phone number must be unique across family members
- Phone cannot be same as customer phone
- Relation type must be valid

### Family Member Gate Pass Request

**Customer Portal:**
1. Family member logs in with their phone
2. Sees customer's entries
3. Requests gate pass
4. System records family member ID
5. Admin sees "Requested by [Family Member Name]"
6. Approves normally
7. Gate pass shows family member info

### Payment by Family Member

**Main System:**
1. Employee/Accountant processes payment
2. Selects "Family Member" option
3. Chooses family member from dropdown
4. Processes payment normally
5. Receipt shows family member name

---

## Reports & Analytics

### Family Member Activity

**Endpoint:** `GET /api/family-members/:id/activity`

**Response:**
```json
{
  "family_member": {
    "id": 15,
    "name": "रमेश कुमार",
    "relation": "son"
  },
  "statistics": {
    "total_gate_passes": 12,
    "total_payments": 5,
    "total_amount_paid": 25000,
    "first_activity": "2025-08-15T10:00:00Z",
    "last_activity": "2026-01-15T14:20:00Z"
  },
  "recent_activities": [
    {
      "type": "gate_pass",
      "date": "2026-01-15",
      "details": "Gate Pass #456 - 100 quintals"
    },
    {
      "type": "payment",
      "date": "2026-01-10",
      "details": "Payment ₹2000"
    }
  ]
}
```

### Customer Family Overview

Shows all family members and their activity for a customer.

---

## Best Practices

### For Admins

**Verification:**
- Verify family relationship before adding
- Confirm with customer
- Check ID proof if needed
- Document relation correctly

**Management:**
- Review family member activity periodically
- Remove inactive family members
- Update contact information
- Monitor for suspicious activity

### For Customers

**Security:**
- Only add trusted family members
- Inform admin if phone number changes
- Report unauthorized family member additions
- Keep track of family member activities

**Privacy:**
- Family members see all customer data
- Choose wisely who has access
- Can request removal anytime

---

## Security & Privacy

### Access Control

**Family Member Can:**
- ✅ View customer entries
- ✅ Request gate passes
- ✅ Make payments
- ✅ View transaction history
- ✅ Use customer portal

**Family Member Cannot:**
- ❌ Modify customer profile
- ❌ Add/remove other family members
- ❌ Delete entries
- ❌ Access admin functions
- ❌ View other customers' data

### Audit Trail

**Tracked Information:**
- Who added the family member
- When family member was added
- All gate passes requested by family member
- All payments made by family member
- Customer portal login activity

**Logs Location:**
- `customer_activity_logs` - Portal activity
- `admin_action_logs` - Family member additions/removals

---

## Example Scenarios

### Scenario 1: Son Handles Everything

**Setup:**
```json
{
  "customer": {
    "name": "राम प्रसाद",
    "phone": "9999999999"
  },
  "family_member": {
    "name": "सुरेश प्रसाद",
    "phone": "8888888888",
    "relation": "son"
  }
}
```

**Usage:**
- Father stores paddy
- Son handles all pickups (gate passes)
- Son makes all payments
- Father just owns the account
- All receipts show "Paid by: सुरेश प्रसाद (son)"

### Scenario 2: Multiple Family Members

**Setup:**
```json
{
  "customer": {
    "name": "विकास शर्मा",
    "phone": "9999999999"
  },
  "family_members": [
    {
      "name": "राहुल शर्मा",
      "phone": "8888888888",
      "relation": "son"
    },
    {
      "name": "अमित शर्मा",
      "phone": "7777777777",
      "relation": "brother"
    }
  ]
}
```

**Usage:**
- Both son and brother can independently request gate passes
- Either can make payments
- Each has their own customer portal login
- All activity tracked separately

---

## Troubleshooting

### Cannot Add Family Member

**Issue:** Phone number already exists

**Solution:**
- Check if phone is used by another customer
- Check if phone is used by another family member
- Use different phone number

### Family Member Cannot Login to Portal

**Issue:** Phone not recognized

**Solution:**
- Verify phone number is correct
- Ensure family member record exists
- Check customer portal is enabled
- Verify OTP delivery

### Gate Pass Not Showing Family Member

**Issue:** Gate pass created without family member ID

**Solution:**
- Ensure family_member_id passed in request
- Re-create gate pass with family member
- Or edit gate pass to add family member

---

## API Reference

[See API_DOCUMENTATION.md for complete Family Members API reference](API_DOCUMENTATION.md#family-members-apis)

---

**Support:** Contact system administrator for family member management assistance
