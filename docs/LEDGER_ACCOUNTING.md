# Ledger & Accounting System Documentation

Complete guide for the double-entry ledger and accounting system.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The system uses a **double-entry ledger** system to track all financial transactions for customers, providing complete audit trails and accurate balance tracking.

**Key Features:**
- Double-entry accounting (debit/credit)
- Real-time customer balance tracking
- Complete transaction history
- Debtors list
- Manual ledger entries
- Audit trail for all transactions

---

## Ledger Basics

### Double-Entry Accounting

Every transaction has two sides:
- **Debit:** Amount owed (increases balance)
- **Credit:** Amount paid (decreases balance)

**Example:**
```
Entry created: Debit ₹5000 (customer owes facility)
Payment made: Credit ₹3000 (customer pays facility)
Balance: ₹2000 (still owed)
```

### Ledger Entry Types

| Type | Description | Debit | Credit |
|------|-------------|-------|--------|
| **entry_creation** | New entry created | ✅ | ❌ |
| **rent_payment** | Rent/storage fee paid | ❌ | ✅ |
| **gate_pass_charge** | Gate pass processing fee | ✅ | ❌ |
| **late_fee** | Late payment penalty | ✅ | ❌ |
| **discount** | Discount given | ❌ | ✅ |
| **refund** | Amount refunded | ❌ | ✅ |
| **adjustment** | Manual adjustment | Either | Either |

---

## Features

### 1. Customer Ledger

**Endpoint:** `GET /api/ledger/customer/:phone`

**Response:**
```json
{
  "customer": {
    "name": "राज कुमार",
    "phone": "9999999999",
    "current_balance": 5000
  },
  "ledger_entries": [
    {
      "id": 456,
      "date": "2026-01-17",
      "entry_type": "entry_creation",
      "description": "Entry #123 - Storage charges",
      "debit": 5000,
      "credit": 0,
      "balance": 5000,
      "reference_id": 123,
      "reference_type": "entry"
    },
    {
      "id": 457,
      "date": "2026-01-16",
      "entry_type": "rent_payment",
      "description": "Payment - Receipt #789",
      "debit": 0,
      "credit": 3000,
      "balance": 2000,
      "reference_id": 789,
      "reference_type": "payment"
    }
  ],
  "summary": {
    "total_debits": 5000,
    "total_credits": 3000,
    "net_balance": 2000
  }
}
```

### 2. Debtors List

**Endpoint:** `GET /api/ledger/debtors?min_balance=1000`

**Response:**
```json
{
  "debtors": [
    {
      "customer_id": 5,
      "customer_name": "राज कुमार",
      "customer_phone": "9999999999",
      "customer_village": "गांव",
      "balance": 5000,
      "days_overdue": 15,
      "last_payment_date": "2026-01-02",
      "entry_count": 3
    }
  ],
  "total_debtors": 45,
  "total_amount_owed": 225000
}
```

**Filters:**
- `min_balance` - Minimum balance to include
- `days_overdue` - Days since last payment
- `village` - Filter by village

### 3. Manual Ledger Entry

**For Adjustments, Corrections, Discounts:**

**Endpoint:** `POST /api/ledger/manual-entry`

**Request:**
```json
{
  "customer_phone": "9999999999",
  "entry_type": "discount",
  "description": "Discount for loyal customer",
  "debit": 0,
  "credit": 500,
  "notes": "10% discount on total"
}
```

**Requires:** Admin permission

**Use Cases:**
- Adjust incorrect charges
- Apply discounts
- Record manual payments
- Correct accounting errors
- Write-off bad debts

### 4. Balance Summary

**Endpoint:** `GET /api/ledger/balance/:phone`

**Quick balance check without full ledger.**

**Response:**
```json
{
  "customer_name": "राज कुमार",
  "current_balance": 5000,
  "last_transaction_date": "2026-01-17",
  "overdue_days": 15
}
```

---

## Automatic Ledger Entries

### Entry Creation

**When:** New entry created  
**Ledger Entry:**
```json
{
  "entry_type": "entry_creation",
  "description": "Entry #123",
  "debit": calculated_rent,
  "credit": 0
}
```

**Calculation:** Based on quantity, duration, rate

### Payment Processing

**When:** Payment received  
**Ledger Entry:**
```json
{
  "entry_type": "rent_payment",
  "description": "Payment - Receipt #789",
  "debit": 0,
  "credit": amount_paid
}
```

### Gate Pass Charges

**When:** Gate pass processed (if applicable)  
**Ledger Entry:**
```json
{
  "entry_type": "gate_pass_charge",
  "description": "Gate Pass #456 processing fee",
  "debit": gate_pass_fee,
  "credit": 0
}
```

---

## Database Schema

### ledger_entries Table

```sql
CREATE TABLE ledger_entries (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id),
    entry_type TEXT NOT NULL,
    description TEXT,
    debit NUMERIC(10,2) DEFAULT 0,
    credit NUMERIC(10,2) DEFAULT 0,
    balance NUMERIC(10,2),
    reference_id INTEGER,
    reference_type TEXT,
    notes TEXT,
    created_by INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ledger_customer ON ledger_entries(customer_id);
CREATE INDEX idx_ledger_date ON ledger_entries(created_at);
CREATE INDEX idx_ledger_type ON ledger_entries(entry_type);
```

**Balance Calculation:**
- Running balance maintained
- Updated on each transaction
- Verified periodically

---

## Reports

### 1. Customer Ledger Report

**Format:** PDF/CSV  
**Endpoint:** `GET /api/reports/ledger/:phone?format=pdf`

**Includes:**
- Customer details
- All ledger entries
- Transaction summary
- Current balance
- Payment history

### 2. Debtors Report

**Format:** PDF/CSV  
**Endpoint:** `GET /api/reports/debtors?format=pdf`

**Includes:**
- All customers with pending balance
- Sorted by balance (highest first)
- Days overdue
- Contact information
- Total outstanding amount

### 3. Daily Transaction Summary

**Endpoint:** `GET /api/reports/daily-transactions?date=2026-01-17`

**Includes:**
- All debits (charges)
- All credits (payments)
- Net change in receivables
- Number of transactions

---

## Balance Reconciliation

### Verification

**Check Customer Balance:**
```sql
SELECT 
    customer_id,
    SUM(debit) - SUM(credit) as calculated_balance
FROM ledger_entries
WHERE customer_id = X
GROUP BY customer_id;
```

**Compare with Stored Balance:**
```sql
SELECT balance FROM customers WHERE id = X;
```

**Should Match!**

### Reconciliation Report

**Endpoint:** `GET /api/ledger/reconciliation`

Checks all customer balances against ledger totals.

**Flags Discrepancies:**
- Balance mismatch
- Missing ledger entries
- Orphaned entries

---

## Workflows

### Payment Processing Workflow

**Employee/Accountant:**
1. Customer arrives with payment
2. Open entry or customer profile
3. Click "Record Payment"
4. Enter amount
5. System:
   - Creates payment record
   - Creates credit ledger entry
   - Updates customer balance
   - Generates receipt
6. Print/email receipt

### Manual Adjustment Workflow

**Admin Only:**
1. Navigate to customer ledger
2. Click "Add Manual Entry"
3. Select entry type
4. Enter debit OR credit amount
5. Add description and notes
6. Save
7. System:
   - Creates ledger entry
   - Updates balance
   - Logs admin action

### Debt Collection Workflow

**Accountant:**
1. Run debtors report
2. Filter by balance > ₹1000
3. Export to CSV
4. Send payment reminders (SMS)
5. Follow up with calls
6. Track payments
7. Update as paid

---

## Integration with Debt Requests

### Debt Request System

Employees can request debt approval for customers who want to defer payment.

**Endpoint:** `POST /api/debt/requests`

```json
{
  "customer_id": 5,
  "amount": 5000,
  "reason": "Customer will pay after harvest",
  "requested_by": 3
}
```

**Workflow:**
1. Employee creates debt request
2. Admin reviews and approves
3. If approved:
   - Ledger entry created as "debt_approved"
   - Customer balance updated
   - Customer can pickup without full payment
4. Payment due tracked in ledger

**Endpoint:** `GET /api/debt/requests?status=pending`

Shows all pending debt approval requests.

---

## Best Practices

### For Accountants

**Daily Tasks:**
- Record all payments promptly
- Verify receipt numbers sequential
- Check balance reconciliation
- Review debtors list

**Weekly Tasks:**
- Run debtors report
- Send payment reminders
- Follow up on overdue accounts
- Reconcile ledger totals

**Monthly Tasks:**
- Generate monthly statements
- Archive old ledger entries
- Review manual adjustments
- Bad debt write-offs (if needed)

### For Admins

**Monitoring:**
- Review manual ledger entries
- Check for anomalies
- Verify large adjustments
- Monitor employee actions

**Reporting:**
- Monthly financial summary
- Aging analysis (30/60/90 days)
- Collection efficiency
- Outstanding receivables trend

---

## Troubleshooting

### Balance Doesn't Match

**Check:**
1. Run reconciliation report
2. Verify all entries have customer_id
3. Check for duplicate entries
4. Review manual adjustments

**Fix:**
- Create adjustment entry
- Document reason
- Update balance manually (last resort)

### Missing Ledger Entry

**For Payment:**
- Check rent_payments table
- Verify payment was processed
- Create manual ledger entry if needed

**For Entry:**
- Check entries table
- Verify entry creation completed
- Add manual debit if needed

### Incorrect Balance

**Steps:**
1. Export full ledger for customer
2. Calculate manually
3. Compare with system
4. Identify discrepancy
5. Create adjustment entry
6. Document in notes

---

## Security & Audit

### Permissions

**View Ledger:**
- Employee: Own transactions only
- Accountant: All customers
- Admin: All customers + manual entries

**Manual Entries:**
- Admin only
- Logged in admin_action_logs
- Requires reason/notes

### Audit Trail

**Every ledger entry records:**
- Who created it (user_id)
- When created (timestamp)
- Reference to source transaction
- Description and notes
- Balance after transaction

**Immutable:** Ledger entries cannot be deleted, only reversed

---

## API Reference

[See API_DOCUMENTATION.md for complete Ledger API reference](API_DOCUMENTATION.md#ledger-system-apis)

---

**Support:** Contact accounting team for ledger questions
