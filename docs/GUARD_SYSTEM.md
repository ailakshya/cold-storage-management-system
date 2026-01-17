# Guard Register System Documentation

Complete guide for the Guard token-based truck registration system.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The Guard Register System is a token-based truck entry management system that allows guards to register incoming trucks, assign daily color-coded tokens, and categorize entries as Seed or Sell.

**Purpose:**
- Streamline truck registration at gate
- Daily token tracking
- Seed vs Sell categorization
- Integration with main entry system

---

## Key Concepts

### 1. Guard Role

**Access Level:** Limited to guard-specific functions
- Register truck entries with tokens
- View guard dashboard
- Process token-based entries
- No access to payments, reports, or admin functions

**Login:** Same authentication as other system users

### 2. Token System

**Daily Token Colors:**
- Each day assigned a unique color
- Guards issue sequential tokens
- Tokens tracked for accountability
- Lost/skipped tokens can be marked

**Token Format:**
- Sequential numbers (1, 2, 3, ...)
- Associated with daily color
- Tied to specific entry

**Example:**
- Date: 2026-01-17
- Color: Blue
- Tokens issued: 1, 2, 3, 5, 6 (token #4 skipped/lost)

### 3. Entry Types

**Seed Entries:**
- Paddy for storage
- Tracked separately
- Quantity in quintals

**Sell Entries:**
- Paddy for direct sale
- Different processing workflow
- Quantity in quintals

---

## Features

### 1. Token Color Management

**Set Daily Token Color:**

**Endpoint:** `PUT /api/token-color`

```json
{
  "date": "2026-01-17",
  "color": "Blue"
}
```

**Get Current Token Color:**

**Endpoint:** `GET /api/token-color?date=2026-01-17`

**Response:**
```json
{
  "date": "2026-01-17",
  "color": "Blue"
}
```

**Common Colors:** Red, Blue, Green, Yellow, Orange, Purple, Pink, Brown, Black, White

### 2. Guard Entry Creation

**Create Guard Entry:**

**Endpoint:** `POST /api/guard/entries`

```json
{
  "token_number": 5,
  "customer_name": "राज कुमार",
  "customer_phone": "9999999999",
  "customer_village": "गाँव का नाम",
  "truck_number": "UP16AB1234",
  "seed_quantity": 100,
  "sell_quantity": 0,
  "notes": "First load"
}
```

**Response:**
```json
{
  "id": 45,
  "token_number": 5,
  "token_color": "Blue",
  "customer_name": "राज कुमार",
  "entry_date": "2026-01-17",
  "status": "pending"
}
```

**Field Details:**
- `token_number`: Sequential number for the day
- `customer_name`: Can be Hindi or English
- `seed_quantity`: Quantity for storage (quintals)
- `sell_quantity`: Quantity for direct sale (quintals)
- At least one quantity must be > 0

### 3. Next Token Number

**Get Next Available Token:**

**Endpoint:** `GET /api/guard/next-token?date=2026-01-17`

**Response:**
```json
{
  "next_token": 6,
  "color": "Blue",
  "last_token": 5
}
```

**Auto-increment:** System suggests next sequential token

### 4. Skip/Lost Token

**Mark Token as Skipped:**

**Endpoint:** `POST /api/guard/tokens/skip`

```json
{
  "token_number": 4,
  "date": "2026-01-17",
  "reason": "Lost token"
}
```

**Use Cases:**
- Physical token lost
- Token damaged
- Token skipped accidentally
- Maintain sequential integrity

### 5. Guard Dashboard

**Endpoint:** `GET /api/guard/dashboard?date=2026-01-17`

**Response:**
```json
{
  "date": "2026-01-17",
  "token_color": "Blue",
  "total_entries": 8,
  "seed_entries": 5,
  "sell_entries": 3,
  "total_seed_quantity": 450,
  "total_sell_quantity": 200,
  "entries": [
    {
      "token_number": 1,
      "customer_name": "राज कुमार",
      "truck_number": "UP16AB1234",
      "seed_quantity": 100,
      "sell_quantity": 0,
      "status": "processed",
      "created_at": "2026-01-17T08:30:00Z"
    }
  ],
  "skipped_tokens": [4],
  "last_token": 8
}
```

---

## Workflow

### Daily Guard Workflow

**Morning Setup:**
1. Login to system as Guard
2. Set today's token color (if not set)
3. Prepare physical tokens with today's color

**Truck Arrival:**
1. Truck arrives at gate
2. Guard collects information
3. Issues physical token to driver
4. Creates guard entry in system
5. Driver proceeds with token

**Processing by Employees:**
1. Employee finds guard entry by token
2. Converts to full entry
3. Assigns rooms (if seed)
4. Processes payment/invoice
5. Token entry marked as processed

**End of Day:**
1. Review dashboard
2. Verify all tokens accounted for
3. Mark any lost tokens
4. Report to admin

### Integration with Main Entry System

**Guard Entry → Full Entry:**

Guard entries serve as pre-registration. Employees then:
1. Search by token number and color
2. Or search by customer phone
3. Find pending guard entry
4. Create full entry with room assignments
5. Guard entry status → "processed"

**Benefits:**
- Faster gate processing
- Accurate truck counting
- Better load tracking
- Reduced wait times

---

## Guard Dashboard Features

### Statistics

**Real-Time Metrics:**
- Total trucks registered today
- Seed vs Sell breakdown
- Total quantity by type
- Pending vs Processed count

**Historical View:**
- View any date's entries
- Compare day-over-day
- Monthly summaries

### Entry Management

**Search & Filter:**
- By token number
- By date
- By customer name/phone
- By  truck number
- By status (pending/processed)

**Export:**
- Daily entry summary (PDF)
- Token utilization report

---

## System Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `guard_register_enabled` | `true` | Enable guard register system |
| `require_token_color` | `true` | Require daily color to be set |
| `auto_increment_token` | `true` | Auto-suggest next token |

---

## Best Practices

### For Guards

**Token Management:**
- Set color first thing in morning
- Use sequential tokens (don't skip)
- Mark lost tokens immediately
- Physical tokens match system tokens

**Data Entry:**
- Verify phone numbers
- Use Hindi for local names
- Double-check truck numbers
- Note any special instructions

**Communication:**
- Inform employees of high volume days
- Report system issues immediately
- Coordinate with weighbridge

### For Admins

**Setup:**
- Train guards on system
- Provide physical tokens
- Color coding system
- Daily checklist

**Monitoring:**
- Review daily token usage
- Check for anomalies
- Verify all entries processed
- Regular audits

---

## Reports

### Daily Token Report

**Endpoint:** `GET /api/guard/reports/daily?date=2026-01-17`

**Includes:**
- All guard entries for date
- Token color
- Seed/Sell breakdown
- Skipped tokens
- Processing status

### Guard Performance Report

**Track:**
- Entries per guard per day
- Average processing time
- Error rates
- Token skip frequency

---

## Troubleshooting

### Token Number Issues

**Duplicate Token:**
- System prevents duplicate tokens per day
- If physical token duplicated, mark one as skipped

**Missing Token:**
- Check skipped tokens list
- Verify not processed
- Mark as skipped if truly lost

### Entry Processing

**Guard Entry Not Found:**
- Verify date and token number
- Check token color matches
- Ensure not already processed

**Cannot Process Guard Entry:**
- Verify employee has permissions
- Check system in correct operation mode
- Ensure all required fields filled

---

## API Reference

[See API_DOCUMENTATION.md for complete Guard API reference](API_DOCUMENTATION.md#guard-system-apis)

---

**Support:** Contact system administrator
