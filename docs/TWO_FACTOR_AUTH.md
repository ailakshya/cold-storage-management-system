# Two-Factor Authentication (2FA) Documentation

Complete guide for TOTP-based Two-Factor Authentication in the Cold Storage Management System.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The system supports **TOTP (Time-based One-Time Password)** two-factor authentication for admin accounts, providing an additional layer of security beyond password authentication.

**Key Features:**
- TOTP-based (compatible with Google Authenticator, Authy, etc.)
- Backup codes for recovery
- Per-user enable/disable
- Failed attempt tracking
- Admin-only (currently)

---

## Supported Roles

**Currently:** Admin users only  
**Future:** Can be extended to Employee and Accountant roles

---

## Setup Process

### For Admins

**Step 1: Enable 2FA**

**Endpoint:** `POST /api/totp/setup`

**Response:**
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code": "data:image/png;base64,iVBORw0KG...",
  "backup_codes": [
    "12345678",
    "23456789",
    "34567890",
    "45678901",
    "56789012"
  ]
}
```

**Step 2: Scan QR Code**
- Open authenticator app (Google Authenticator, Authy, Microsoft Authenticator)
- Scan QR code
- App generates 6-digit codes every 30 seconds

**Step 3: Verify Setup**

**Endpoint:** `POST /api/totp/verify-setup`

```json
{
  "code": "123456"
}
```

**Response:**
```json
{
  "success": true,
  "message": "2FA enabled successfully"
}
```

**Step 4: Save Backup Codes**
- Store backup codes securely
- Use if authenticator app unavailable
- Each code single-use only

---

## Login with 2FA

### Regular Login Flow

**Step 1: Username/Password**

```http
POST /auth/login
{
  "email": "admin@cold.com",
  "password": "password123"
}
```

**Response (2FA Enabled):**
```json
{
  "requires_2fa": true,
  "temp_token": "temp_token_here"
}
```

**Step 2: Verify 2FA Code**

```http
POST /auth/verify-2fa
{
  "temp_token": "temp_token_here",
  "code": "123456"
}
```

**Response:**
```json
{
  "success": true,
  "token": "jwt_token_here",
  "user": {
    "id": 1,
    "email": "admin@cold.com",
    "role": "admin"
  }
}
```

### Using Backup Code

If authenticator app unavailable:

```http
POST /auth/verify-2fa
{
  "temp_token": "temp_token_here",
  "backup_code": "12345678"
}
```

**Note:** Backup code is consumed after use

---

## Managing 2FA

### Check 2FA Status

**Endpoint:** `GET /api/totp/status`

**Response:**
```json
{
  "enabled": true,
  "backup_codes_remaining": 4,
  "created_at": "2026-01-15T10:30:00Z"
}
```

### Disable 2FA

**Endpoint:** `POST /api/totp/disable`

**Requires:** Current password verification

```json
{
  "password": "current_password"
}
```

**Response:**
```json
{
  "success": true,
  "message": "2FA disabled successfully"
}
```

**Security:** TOTP secret and backup codes deleted

### Regenerate Backup Codes

**Endpoint:** `POST /api/totp/regenerate-backup-codes`

**Password required**

**Response:**
```json
{
  "backup_codes": [
    "87654321",
    "76543210",
    "65432109",
    "54321098",
    "43210987"
  ]
}
```

**Note:** Old backup codes invalidated

---

## Security Features

### Attempt Tracking

**Failed Attempts:**
- Tracked in `totp_verification_attempts`
- Max 5 failed attempts per hour
- Temporary lockout after limit
- Admin notification on suspicious activity

**Logged Information:**
- User ID
- Success/failure
- IP address
- Timestamp
- Code type (TOTP vs backup)

### Time-Based Validation

**TOTP Specifications:**
- Algorithm: SHA-1
- Digits: 6
- Time step: 30 seconds
- Window: ±1 time step (allows clock drift)

**Code Validity:**
- Code valid for 30-second window
- Previous/next window accepted (90 seconds total)
- Prevents replay attacks

---

## Recovery Procedures

### Lost Authenticator App

**Option 1: Use Backup Code**
1. Login with username/password
2. Enter backup code instead of TOTP
3. Login successfully
4. Setup new authenticator
5. Generate new backup codes

**Option 2: Admin Reset**
1. Contact another admin
2. Admin disables your 2FA
3. Login without 2FA
4. Setup 2FA again

**Option 3: Database Reset** (Emergency)
```sql
-- Admin access to database required
DELETE FROM totp_secrets WHERE user_id = X;
DELETE FROM totp_backup_codes WHERE user_id = X;
DELETE FROM totp_verification_attempts WHERE user_id = X;
```

### Lost Backup Codes

**If 2FA Still Working:**
1. Login normally
2. Navigate to 2FA settings
3. Regenerate backup codes
4. Save new codes securely

**If 2FA Lost:**
- Contact admin for reset
- Or use database reset (emergency)

---

## Best Practices

### For Users

**Setup:**
- Use reputable authenticator app
- Save backup codes offline (not in cloud)
- Test 2FA before logging out
- Keep backup codes in secure location

**Usage:**
- Don't share authenticator app
- Set authenticator app password/PIN
- Regularly backup authenticator data
- Update backup codes if compromised

**Security:**
- Use different 2FA for personal accounts
- Never share TOTP codes
- Be aware of phishing attempts
- Report suspicious activity

### For Admins

**Deployment:**
- Enable 2FA for all admin accounts
- Provide clear setup instructions
- Test recovery procedures
- Document backup code storage policy

**Monitoring:**
- Review failed attempt logs
- Set up alerts for anomalies
- Regular security audits
- Update users on best practices

**Support:**
- Have reset procedure documented
- Designate recovery admins
- Test recovery regularly
- Maintain emergency access

---

## Troubleshooting

### Code Not Accepted

**Check:**
- Time synchronized on device
- Correct user account
- Code not expired (30 seconds)
- Not using old code

**Solutions:**
- Sync device time with internet
- Wait for next code
- Check authenticator app account
- Ensure correct email

### Cannot Enable 2FA

**Possible Issues:**
- User not admin
- 2FA already enabled
- Server time misconfigured

**Solutions:**
- Verify admin role
- Check current status
- Contact system administrator

### Locked Out

**If Failed Attempts:**
- Wait 1 hour (auto-unlock)
- Or contact admin for manual unlock
- Or use backup code

---

## Database Schema

### totp_secrets

```sql
CREATE TABLE totp_secrets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    secret TEXT NOT NULL,
    enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### totp_backup_codes

```sql
CREATE TABLE totp_backup_codes (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    code TEXT NOT NULL,
    used BOOLEAN DEFAULT false,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### totp_verification_attempts

```sql
CREATE TABLE totp_verification_attempts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    success BOOLEAN,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## Configuration

### Environment Variables

```env
# TOTP Settings (optional - uses defaults)
TOTP_ISSUER=ColdStorage
TOTP_DIGITS=6
TOTP_PERIOD=30
```

### System Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `2fa_enabled_for_admins` | `true` | Require 2FA for all admins |
| `2fa_max_attempts` | `5` | Max failed attempts per hour |
| `2fa_backup_codes_count` | `5` | Number of backup codes |

---

## API Reference

[See API_DOCUMENTATION.md for complete 2FA API reference](API_DOCUMENTATION.md#two-factor-authentication-apis)

---

**Support:** Contact system administrator for 2FA assistance
