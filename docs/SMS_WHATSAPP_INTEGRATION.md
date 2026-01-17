# SMS & WhatsApp Integration Documentation

Complete guide for SMS and WhatsApp messaging features.

**Version:** 1.5.173  
**Last Updated:** 2026-01-17

---

## Overview

The system integrates with SMS and WhatsApp providers for customer communication including payment reminders, boli notifications, and general alerts.

**Key Features:**
- Bulk SMS sending
- Payment reminder automation  
- Boli notifications (loading completion)
- Delivery tracking and logs
- Configurable templates
- Multi-provider support

---

## SMS Provider Setup

### Configuration

```env
SMS_API_KEY=your_api_key
SMS_API_URL=https://sms-provider.com/api
WHATSAPP_API_KEY=your_whatsapp_key
SMS_SENDER_ID=COLDSTORAGE
```

### Supported Providers

- **Fast2SMS**
- **Twilio**
- **MSG91**
- **Custom HTTP API**

---

## Features

### 1. Bulk SMS

**Endpoint:** `POST /api/sms/bulk`

**Request:**
```json
{
  "phones": ["9999999999", "8888888888"],
  "message": "Your payment of ₹5000 is due. Please pay at earliest.",
  "template_id": "payment_reminder"
}
```

**Response:**
```json
{
  "success": true,
  "total_sent": 2,
  "failed": 0,
  "message_ids": ["msg_123", "msg_124"]
}
```

**Use Cases:**
- Payment reminders
- Season change announcements
- Facility maintenance notices
- General notifications

### 2. Payment Reminders

**Endpoint:** `POST /api/sms/payment-reminders`

**Request:**
```json
{
  "min_balance": 1000,
  "days_overdue": 7
}
```

**Behavior:**
- Queries customers with balance ≥ min_balance
- Filters by days overdue
- Sends personalized SMS with amount
- Logs delivery status

**Message Template:**
```
Dear {{customer_name}},
Your payment of ₹{{amount}} is pending for {{days}} days.
Please visit us to clear dues.
- Gurukrupa Cold Storage
```

### 3. Boli Notifications

**Endpoint:** `POST /api/sms/boli-notification`

**Request:**
```json
{
  "entry_id": 123,
  "customer_phone": "9999999999"
}
```

**Automatically Sent When:**
- Loading (boli) is marked complete
- Gate pass approved
- Entry ready for pickup

**Message Template:**
```
Your goods (Entry #{{entry_id}}) are ready for pickup.
Token: {{token_number}}
Gate Pass: #{{gate_pass_id}}
```

### 4. Custom SMS

**Endpoint:** `POST /api/sms/send`

**Request:**
```json
{
  "phone": "9999999999",
  "message": "Your custom message here"
}
```

**Use Cases:**
- Individual customer communication
- Urgent notifications
- Custom alerts

---

## Message Templates

### Predefined Templates

#### Payment Reminder
```
key: payment_reminder
variables: customer_name, amount, days_overdue
```

#### Gate Pass Approved
```
key: gate_pass_approved
variables: customer_name, entry_id, quantity, gate_pass_id
```

#### Boli Complete
```
key: boli_complete
variables: customer_name, entry_id, token_number
```

#### Season Change
```
key: season_change
variables: old_season, new_season, effective_date
```

### Managing Templates

**Get Templates:** `GET /api/sms/templates`

**Update Template:** `PUT /api/sms/templates/:key`

```json
{
  "template": "Dear {{customer_name}}, your payment of ₹{{amount}} is due.",
  "variables": ["customer_name", "amount"]
}
```

---

## SMS Logs

### View Logs

**Endpoint:** `GET /api/sms/logs?limit=100&status=delivered`

**Response:**
```json
{
  "logs": [
    {
      "id": 456,
      "phone_number": "9999999999",
      "message": "Payment reminder sent",
      "status": "delivered",
      "provider": "fast2sms",
      "message_id": "msg_123",
      "sent_at": "2026-01-17T10:30:00Z",
      "delivered_at": "2026-01-17T10:30:15Z"
    }
  ],
  "total": 1250
}
```

**Statuses:**
- `pending` - Queued for sending
- `sent` - Sent to provider
- `delivered` - Delivered to recipient
- `failed` - Delivery failed
- `rejected` - Rejected by provider

### Statistics

**Endpoint:** `GET /api/sms/stats?period=30d`

```json
{
  "total_sent": 5420,
  "delivered": 5320,
  "failed": 100,
  "delivery_rate": 0.98,
  "total_cost": 542.0,
  "by_type": {
    "payment_reminder": 2500,
    "boli_notification": 1800,
    "gate_pass_approved": 820,
    "custom": 300
  }
}
```

---

## WhatsApp Integration

### Setup

**WhatsApp Business API** required (not personal WhatsApp)

**Configuration:**
```env
WHATSAPP_API_KEY=your_key
WHATSAPP_API_URL=https://whatsapp-provider.com/api
WHATSAPP_NUMBER=919999999999
```

### Send WhatsApp Message

**Endpoint:** `POST /api/sms/whatsapp`

**Request:**
```json
{
  "phone": "9999999999",
  "message": "Your goods are ready for pickup",
  "template_name": "boli_complete"
}
```

**Note:** WhatsApp requires pre-approved templates

### WhatsApp vs SMS

| Feature | WhatsApp | SMS |
|---------|----------|-----|
| Cost | Lower | Higher |
| Delivery Rate | Higher | Lower (DND) |
| Rich Media | Yes | No |
| Templates | Required | Optional |
| Setup | Complex | Simple |

**Recommendation:** Use WhatsApp for  important notifications, SMS as fallback

---

## Automation

### Auto Payment Reminders

**Cron Job:** Daily at 10 AM

**Logic:**
```javascript
// Pseudo-code
customers = getCustomersWithPendingBalance(minBalance: 1000)
for each customer:
  if daysOverdue >= 7:
    sendPaymentReminder(customer)
    log("Reminder sent to " + customer.phone)
```

**Configuration:**
```env
AUTO_PAYMENT_REMINDERS=true
PAYMENT_REMINDER_MIN_BALANCE=1000
PAYMENT_REMINDER_DAYS_OVERDUE=7
PAYMENT_REMINDER_TIME=10:00
```

### Auto Boli Notifications

**Trigger:** When entry marked as boli complete

**Automatic:** Yes (if enabled in settings)

**Setting:** `auto_boli_notifications=true`

---

## System Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `sms_enabled` | `true` | Enable SMS functionality |
| `whatsapp_enabled` | `false` | Enable WhatsApp (requires setup) |
| `auto_payment_reminders` | `true` | Auto send payment reminders |
| `auto_boli_notifications` | `true` | Auto send boli notifications |
| `sms_provider` | `fast2sms` | SMS provider (fast2sms/twilio/msg91) |
| `payment_reminder_days` | `7` | Days overdue for reminders |
| `payment_reminder_min_balance` | `1000` | Min balance for reminders |

---

## Cost Management

### Credits Tracking

**Check Balance:** `GET /api/sms/credits`

```json
{
  "provider": "fast2sms",
  "credits_remaining": 5000,
  "credits_used_this_month": 542,
  "estimated_cost": "₹542"
}
```

### Usage Limits

**Set Monthly Limit:**
```json
{
  "max_monthly_messages": 10000,
  "alert_threshold": 8000
}
```

**Alert:** Notification when threshold reached

---

## Best Practices

### Message Content

**Do:**
- Keep messages concise
- Include business name
- Personalize with customer name
- Include call-to-action

**Don't:**
- Use promotional language (may trigger DND)
- Send late night (after 9 PM)
- Spam customers
- Use special characters excessively

### Delivery Optimization

- Send during business hours (9 AM - 6 PM)
- Avoid weekends for non-urgent messages
- Use WhatsApp for higher delivery rate
- Maintain DND compliance

### Template Management

- Keep templates up to date
- Test templates before bulk send
- Get approval for WhatsApp templates
- Monitor delivery rates by template

---

## Troubleshooting

### Messages Not Sending

**Check:**
- SMS provider credentials
- API balance/credits
- Network connectivity
- Phone number format (+91xxxxxxxxxx)

**Common Issues:**
- Invalid API key
- Insufficient credits
- DND-registered numbers
- Rate limiting

### Low Delivery Rates

**Causes:**
- DND-registered numbers (50-60% in India)
- Invalid phone numbers
- Promotional content flagged
- Network issues

**Solutions:**
- Use WhatsApp as alternative
- Verify phone numbers
- Use transactional templates
- Contact provider support

### WhatsApp Template Rejection

**Reasons:**
- Policy violations
- Promotional content
- Incorrect format

**Solution:**
- Review WhatsApp Business Policy
- Use approved template format
- Resubmit with corrections

---

## Compliance

### TRAI DND Regulations

**India:**
- Respect DND preferences
- Use transactional templates
- Obtain customer consent
- Maintain opt-out mechanism

**Best Practice:**
- Mark all messages as transactional
- Include opt-out instructions
- Maintain do-not-call list

---

**Support:** Contact system administrator  
**SMS Provider Support:** Contact provider directly for delivery issues
