# Customer Portal Documentation

Complete guide for the customer-facing web portal.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The Customer Portal is a dedicated web application running on **port 8081** that allows customers to view their entries, request gate passes, make online payments, and access their transaction history.

**Key Features:**
- Separate authentication system (OTP-based or password)
- Mobile-responsive interface
- Hindi transliteration support
- Razorpay payment integration
- Family member delegation
- Real-time entry tracking

---

## Access Information

**Portal URL:** `http://your-domain.com:8081` or `http://192.168.15.200:8081`

**Authentication Methods:**
1. **OTP Login** (Default) - One-time password sent via SMS
2. **Password Login** (Optional) - Traditional password-based login

**Configuration:** System setting `customer_login_method` controls which method is active

---

## Features

### 1. Dashboard

**Endpoint:** `GET /api/dashboard`

**Information Displayed:**
- Current entries with quantities and locations
- Active gate passes
- Recent payments
- Account balance
- Family members

**Example Response:**
```json
{
  "customer": {
    "name": "राज कुमार",
    "phone": "9999999999",
    "village": "गाँव नाम",
    "total_balance": 5000
  },
  "entries": [
    {
      "id": 123,
      "truck_number": "UP16AB1234",
      "expected_quantity": 500,
      "current_quantity": 480,
      "entry_date": "2026-01-15",
      "rooms": ["Room 1 - Floor 2 - Gate 45-50"]
    }
  ],
  "active_gate_passes": [],
  "recent_payments": [],
  "family_members": []
}
```

### 2. Gate Pass Request

Customers can request gate passes to withdraw goods.

**Endpoint:** `POST /api/gate-pass-requests`

**Request Body:**
```json
{
  "entry_id": 123,
  "requested_quantity": 100,
  "notes": "Pickup for sale",
  "family_member_id": null
}
```

**Workflow:**
1. Customer selects entry
2. Specifies quantity to withdraw
3. Optionally assigns to family member
4. Submits request
5. Admin approves in main system
6. Customer receives notification
7. Pickup at facility with approved gate pass

**Restrictions:**
- Only available in **Unloading Mode**
- Cannot exceed remaining quantity
- Pending requests block new requests for same entry

### 3. Online Payments

**Razorpay Integration:**

**Create Payment Order:**
```http
POST /api/payment/create-order
Content-Type: application/json

{
  "amount": 5000,
  "currency": "INR"
}
```

**Response:**
```json
{
  "order_id": "order_MNop123456",
  "amount": 5000,
  "currency": "INR",
  "razorpay_key": "rzp_live_xxxxx"
}
```

**Verify Payment:**
```http
POST /api/payment/verify
Content-Type: application/json

{
  "razorpay_order_id": "order_MNop123456",
  "razorpay_payment_id": "pay_MNop789012",
  "razorpay_signature": "signature_hash"
}
```

**Payment Flow:**
1. Customer views balance
2. Clicks "Pay Online"
3. Razorpay checkout opens
4. Customer completes payment
5. Webhook verifies payment
6. Balance updated automatically
7. Receipt generated

### 4. Transaction History

**Endpoint:** `GET /api/payment/transactions`

View all online payment transactions with status.

**Transaction Statuses:**
- `created` - Order created
- `authorized` - Payment authorized
- `captured` - Payment successful
- `failed` - Payment failed
- `refunded` - Payment refunded

### 5. Family Members

Customers can link family members who can:
- Make gate pass requests on their behalf
- Make payments
- View entries (tied to family member)

**Linked via Main System:** Family members are added by admin/accountant

---

## Authentication

### OTP Login

**Step 1: Request OTP**
```http
POST /auth/send-otp
Content-Type: application/json

{
  "phone": "9999999999"
}
```

**Response:**
```json
{
  "success": true,
  "message": "OTP sent successfully",
  "expires_in": 300
}
```

**Step 2: Verify OTP**
```http
POST /auth/verify-otp
Content-Type: application/json

{
  "phone": "9999999999",
  "otp": "123456"
}
```

**Response:**
```json
{
  "success": true,
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "customer": {
    "id": 5,
    "name": "राज कुमार",
    "phone": "9999999999"
  }
}
```

**OTP Details:**
- 6-digit numeric code
- Valid for 5 minutes
- SMS delivery via configured provider
- Maximum 3 attempts per phone per hour (rate limited)

### Password Login

**Endpoint:** `POST /auth/login`

```json
{
  "phone": "9999999999",
  "password": "customer_password"
}
```

**Password Management:**
- Set by admin in main system
- No self-service password reset
- Contact admin to change password

### Session Management

**JWT Token:**
- Stored in `localStorage`
- Automatically sent with API requests
- Separate from main system tokens
- Expires after 7 days of inactivity

**Logout:** `POST /auth/logout` - Clears session

---

## Hindi Transliteration

The portal supports real-time Hindi transliteration for Indian language users.

**How it Works:**
1. Customer types in English (e.g., "raj kumar")
2. System detects Hindi-compatible fields
3. Transliterates to Devanagari (राज कुमार)
4. Displays suggestions

**Translation API:**
```http
GET /api/translate?text=raj%20kumar&lang=hi

Response:
{
  "transliterated": "राज कुमार",
  "suggestions": ["राज कुमार", "राजकुमार"]
}
```

**Powered By:** Google Transliteration API

---

## Security

**Rate Limiting:**
- OTP requests: 3 per hour per phone
- Login attempts: 5 per hour per phone
- API requests: 100 per minute per customer

**Activity Logging:**
- All customer actions logged in `customer_activity_logs`
- IP address tracking
- Suspicious activity alerts

**Data Access:**
- Customers only see their own data
- No cross-customer data leakage
- Family members see restricted data

---

## Configuration

### System Settings

| Setting Key | Default | Description |
|-------------|---------|-------------|
| `customer_login_method` | `otp` | Login method: `otp` or `password` |
| `customer_portal_enabled` | `true` | Enable/disable portal |
| `razorpay_enabled` | `false` | Enable online payments |
| `otp_expiry_minutes` | `5` | OTP validity duration |

### Environment Variables

```env
# Customer Portal
CUSTOMER_PORTAL_PORT=8081
CUSTOMER_JWT_SECRET=your-customer-jwt-secret

# Razorpay
RAZORPAY_KEY_ID=rzp_live_xxxxx
RAZORPAY_KEY_SECRET=your_key_secret
RAZORPAY_WEBHOOK_SECRET=webhook_secret

# SMS Provider
SMS_API_KEY=your_sms_api_key
SMS_API_URL=https://sms-provider.com/api
```

---

## User Guide

### For Customers

**First Time Login:**
1. Visit customer portal URL
2. Enter your registered phone number
3. Receive OTP via SMS
4. Enter OTP and login
5. View dashboard

**Requesting Gate Pass:**
1. Login to portal
2. Click "Request Gate Pass"
3. Select entry
4. Enter quantity to withdraw
5. Add notes (optional)
6. Submit request
7. Wait for admin approval
8. Receive notification when approved

**Making Online Payment:**
1. Login to portal
2. View current balance
3. Click "Pay Online"
4. Enter amount (or pay full balance)
5. Complete Razorpay payment
6. Receive confirmation
7. Balance updated immediately

### For Administrators

**Managing Customer Portal Access:**
1. Enable customer in main system
2. Set phone number (required for OTP)
3. Optionally set password
4. Customer can now login

**Approving Gate Pass Requests:**
1. Check gate pass queue in main system
2. Review request details
3. Approve or reject
4. Customer notified automatically

**Monitoring Activity:**
1. View customer activity logs
2. Check login attempts
3. Review payment transactions
4. Monitor for suspicious activity

---

## Troubleshooting

### Customer Cannot Login

**OTP Not Received:**
- Check SMS provider logs
- Verify phone number is correct
- Check rate limiting (3 OTP/hour)
- Verify SMS provider credentials

**Password Login Not Working:**
- Ensure `customer_login_method` = `password`
- Verify password set in main system
- Check customer account active

### Payment Issues

**Razorpay Payment Failing:**
- Verify Razorpay credentials
- Check webhook configuration
- Review Razorpay dashboard for errors
- Ensure webhook secret matches

**Payment Not Reflecting:**
- Check webhook delivery
- Review `online_transactions` table
- Check Razorpay signature verification
- Manual reconciliation may be needed

### Gate Pass Request Issues

**Cannot Submit Request:**
- Verify system in Unloading Mode
- Check if pending request exists
- Verify sufficient quantity remaining
- Ensure entry not deleted

---

## API Reference

[See API_DOCUMENTATION.md for complete API reference](API_DOCUMENTATION.md#customer-portal-apis)

---

**Support:** Contact system administrator for assistance
