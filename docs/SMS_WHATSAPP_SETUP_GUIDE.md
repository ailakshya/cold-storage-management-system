# SMS & WhatsApp Setup Guide for Cold Storage

This guide covers complete setup for:
1. DLT Registration (for cheap SMS at ₹0.17/msg)
2. Fast2SMS Configuration
3. WhatsApp Business API Setup (AiSensy/Interakt)
4. Message Template Creation

---

## Cost Comparison

| Channel | Cost/msg | Monthly (1200 customers × 30 days) |
|---------|----------|-----------------------------------|
| SMS Quick Route | ₹5.00 | ₹180,000 |
| SMS DLT Route | ₹0.17 | ₹6,120 |
| WhatsApp | ₹0.08 | ₹2,880 |
| **WhatsApp + SMS Fallback** | ~₹0.10 | ~₹3,600 |

**Recommended: WhatsApp-first with SMS fallback = 98% cost savings!**

---

## Part 1: DLT Registration (Required for Cheap SMS)

### What is DLT?
DLT (Distributed Ledger Technology) is TRAI's mandate for all commercial SMS in India. Without DLT registration, you can only use expensive "Quick" route (₹5/SMS).

### Step 1: Choose DLT Operator
Register on any ONE of these portals:

| Operator | Portal URL | Recommended |
|----------|-----------|-------------|
| Jio | https://trueconnect.jio.com | ✅ Fast approval |
| Airtel | https://www.airtel.in/business/commercial-communication | Good |
| Vodafone-Idea | https://www.vilpower.in | Good |
| BSNL | https://www.ucc-bsnl.co.in | Slow |

**Recommended: Jio TrueConnect** (fastest approval, good support)

### Step 2: Register as Principal Entity (PE)

1. Go to https://trueconnect.jio.com
2. Click "Register" → "Principal Entity"
3. Fill business details:
   - **Entity Type**: Private Limited / Proprietorship / Partnership
   - **Business Name**: Your cold storage name
   - **Business PAN**: Company PAN card
   - **GST Number**: Your GSTIN
   - **Authorized Person**: Owner/Director name
   - **Mobile**: For OTP verification
   - **Email**: Business email

4. Upload documents:
   - PAN Card (Business)
   - GST Certificate
   - Certificate of Incorporation / Partnership Deed
   - Authorized signatory ID proof
   - Letter of Authorization (if not owner)

5. Pay registration fee: ₹5,900 (one-time, valid for 1 year)

6. Wait for approval: **2-7 business days**

### Step 3: After Approval - Get Entity ID

Once approved, you'll receive:
- **Principal Entity ID (PEID)**: 19-digit number (e.g., 1234567890123456789)
- Login credentials for DLT portal

**Save this Entity ID - needed for Fast2SMS configuration!**

### Step 4: Register Sender ID (Header)

1. Login to DLT portal
2. Go to "Header Registration"
3. Register your 6-character Sender ID:
   - Example: `COLDST` or `CSMSPL` (your brand)
   - Must be 6 alphabetic characters
   - Should represent your business

4. Header Type: Select "Transactional" for OTP/alerts
5. Wait for approval: 1-3 days

### Step 5: Register Message Templates

1. Go to "Content Template Registration"
2. Create templates for each message type:

#### Template 1: OTP Template
```
Template Name: Cold Storage OTP
Template Type: Transactional (Service Implicit)
Content:
Dear Customer, Your OTP for Cold Storage login is {#var#}. Valid for 10 minutes. Do not share with anyone. - COLDST
```

#### Template 2: Payment Received
```
Template Name: Payment Confirmation
Template Type: Transactional (Service Implicit)
Content:
Dear {#var#}, payment of Rs.{#var#} received. Remaining balance: Rs.{#var#}. Thank you! - Cold Storage
```

#### Template 3: Item In Notification
```
Template Name: Item Storage Confirmation
Template Type: Transactional (Service Implicit)
Content:
Dear {#var#}, {#var#} items received at Cold Storage. Thock: {#var#}. Total stored: {#var#}. Thank you! - COLDST
```

#### Template 4: Item Out Notification
```
Template Name: Item Pickup Confirmation
Template Type: Transactional (Service Implicit)
Content:
Dear {#var#}, {#var#} items picked up from Cold Storage. Gate Pass: {#var#}. Remaining: {#var#} items. Thank you! - COLDST
```

#### Template 5: Payment Reminder
```
Template Name: Payment Reminder
Template Type: Transactional (Service Explicit)
Content:
Dear {#var#}, your pending balance at Cold Storage is Rs.{#var#}. Please clear the dues at your earliest. Thank you! - COLDST
```

**Note**: `{#var#}` is the variable placeholder in DLT templates.

3. Wait for template approval: 1-3 days
4. Note down **Template IDs** for each approved template

---

## Part 2: Fast2SMS Configuration

### Step 1: Create Fast2SMS Account

1. Go to https://www.fast2sms.com
2. Sign up with email/mobile
3. Complete KYC verification

### Step 2: Add DLT Details

1. Login to Fast2SMS
2. Go to Settings → DLT Settings
3. Enter:
   - **Entity ID**: Your 19-digit PEID from DLT portal
   - **Sender ID**: Your registered 6-char header (e.g., COLDST)

4. Add Templates:
   - Go to "DLT Templates"
   - Add each template with its Template ID from DLT portal

### Step 3: Get API Key

1. Go to "Dev API" section
2. Copy your **API Authorization Key**
3. This is your `FAST2SMS_API_KEY`

### Step 4: Configure in Cold Storage System

Go to **System Settings → SMS Configuration**:

| Setting | Value |
|---------|-------|
| SMS Route | DLT (₹0.17/SMS) |
| Sender ID | COLDST (your registered header) |
| DLT Entity ID | Your 19-digit PEID |
| Cost per SMS | 0.17 |

### Step 5: Recharge Fast2SMS

| Recharge Amount | Per SMS Cost | Total SMS |
|-----------------|--------------|-----------|
| ₹1,000 | ₹0.20 | 5,000 |
| ₹5,000 | ₹0.18 | 27,777 |
| ₹10,000 | ₹0.17 | 58,823 |
| ₹25,000+ | ₹0.16 | 156,250+ |

**Recommended: ₹10,000+ for best rates**

---

## Part 3: WhatsApp Business API Setup

### Option A: AiSensy (Cheapest - ₹0.06-0.08/msg)

#### Step 1: Sign Up
1. Go to https://aisensy.com
2. Click "Start Free Trial"
3. Enter business details

#### Step 2: Connect WhatsApp Number
1. You need a NEW phone number (not your personal WhatsApp)
2. Can be a landline or mobile number
3. AiSensy will help verify it with Meta

#### Step 3: Business Verification
1. Submit business documents:
   - Business PAN
   - GST Certificate
   - Business address proof
2. Meta will verify (takes 2-7 days)

#### Step 4: Create Message Templates
WhatsApp requires pre-approved templates for business-initiated messages.

**Template 1: Payment Reminder**
```
Name: payment_reminder
Category: UTILITY
Language: English

Header: Payment Reminder
Body: Dear {{1}}, your pending balance at Cold Storage is Rs.{{2}}. Please clear the dues at your earliest convenience.
Footer: Cold Storage Management
Buttons: [Call Us] [Pay Now]
```

**Template 2: Item Notification**
```
Name: item_notification
Category: UTILITY
Language: English

Body: Dear {{1}}, {{2}} items have been {{3}} at Cold Storage.
Reference: {{4}}
Current stock: {{5}} items
Footer: Thank you for choosing us!
```

**Template 3: Payment Confirmation**
```
Name: payment_received
Category: UTILITY
Language: English

Body: Dear {{1}}, we have received your payment of Rs.{{2}}.
Remaining balance: Rs.{{3}}
Thank you for your payment!
```

#### Step 5: Get API Key
1. Go to AiSensy Dashboard → Settings → API
2. Copy the API Key
3. Enter in Cold Storage System Settings

#### AiSensy Pricing
| Plan | Monthly Fee | Per Message |
|------|-------------|-------------|
| Starter | Free | ₹0.08 |
| Growth | ₹999 | ₹0.06 |
| Pro | ₹2,499 | ₹0.05 |

---

### Option B: Interakt (Easy Setup - ₹0.08-0.10/msg)

#### Step 1: Sign Up
1. Go to https://www.interakt.shop
2. Click "Start Free Trial"
3. Enter business details

#### Step 2: Connect WhatsApp
1. Follow guided setup
2. Connect your WhatsApp Business number
3. Complete Meta verification

#### Step 3: Create Templates
Similar to AiSensy, create utility templates for:
- Payment reminders
- Item notifications
- Payment confirmations

#### Step 4: Get API Key
1. Go to Settings → Developer Settings
2. Copy API Key and API Secret
3. Configure in Cold Storage system

#### Interakt Pricing
| Plan | Monthly Fee | Per Message |
|------|-------------|-------------|
| Starter | ₹799 | ₹0.10 |
| Growth | ₹1,999 | ₹0.08 |
| Advanced | ₹4,999 | ₹0.06 |

---

## Part 4: Configure in Cold Storage System

### SMS Settings (System Settings Page)

```
SMS Configuration:
├── SMS Route: DLT (₹0.17/SMS)
├── Sender ID: COLDST
├── DLT Entity ID: 1234567890123456789
└── Cost per SMS: 0.17

WhatsApp Configuration:
├── Enable WhatsApp: ✅ ON
├── Provider: AiSensy
├── API Key: your_aisensy_api_key
└── Cost per Message: 0.08
```

### Enable Notifications

```
Automatic SMS Notifications:
├── Item In SMS: ✅ ON
├── Item Out SMS: ✅ ON
├── Payment Received SMS: ✅ ON
├── Payment Reminder SMS: ✅ ON
└── Promotional SMS: ✅ ON
```

---

## Part 5: Timeline & Checklist

### Week 1: DLT Registration
- [ ] Choose DLT operator (Jio recommended)
- [ ] Gather documents (PAN, GST, COI)
- [ ] Register as Principal Entity
- [ ] Pay ₹5,900 registration fee
- [ ] Wait for approval (2-7 days)

### Week 2: DLT Templates & Headers
- [ ] Register Sender ID (Header)
- [ ] Create all message templates
- [ ] Wait for template approval (1-3 days)
- [ ] Note down Entity ID, Header, Template IDs

### Week 2-3: Fast2SMS Setup
- [ ] Create Fast2SMS account
- [ ] Complete KYC
- [ ] Add DLT details
- [ ] Add templates
- [ ] Recharge account (₹10,000+ recommended)
- [ ] Configure in Cold Storage system
- [ ] Test SMS sending

### Week 2-3: WhatsApp Setup (Parallel)
- [ ] Sign up for AiSensy/Interakt
- [ ] Get new phone number for WhatsApp
- [ ] Submit business documents
- [ ] Wait for Meta verification (2-7 days)
- [ ] Create message templates
- [ ] Wait for template approval (1-2 days)
- [ ] Get API key
- [ ] Configure in Cold Storage system
- [ ] Test WhatsApp sending

### Week 4: Go Live
- [ ] Enable WhatsApp-first in settings
- [ ] Test with real customers
- [ ] Monitor delivery rates
- [ ] Adjust templates if needed

---

## Troubleshooting

### SMS Issues

**Problem**: SMS not delivered
- Check DLT template approval status
- Verify Entity ID is correct
- Check Fast2SMS balance

**Problem**: Still charging ₹5/SMS
- Route not set to "DLT" in settings
- DLT details not configured in Fast2SMS

### WhatsApp Issues

**Problem**: WhatsApp not sending
- Check if number is on WhatsApp
- Verify template is approved
- Check API key is correct

**Problem**: Template rejected
- Avoid promotional language in utility templates
- Include opt-out option for marketing
- Follow Meta's template guidelines

---

## Support Contacts

| Service | Support |
|---------|---------|
| Fast2SMS | support@fast2sms.com |
| Jio DLT | truconnect.support@jio.com |
| AiSensy | support@aisensy.com |
| Interakt | support@interakt.shop |

---

## Cost Calculator

For 1200 customers, 1 message/day:

| Scenario | Monthly Cost |
|----------|-------------|
| SMS Quick (current) | ₹180,000 |
| SMS DLT only | ₹6,120 |
| WhatsApp only | ₹2,880 |
| WhatsApp + 20% SMS fallback | ₹3,456 |

**Annual Savings with WhatsApp: ₹2,11,000+**
