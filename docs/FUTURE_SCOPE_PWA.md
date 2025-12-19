# Future Scope: Progressive Web App (PWA) with Offline Support

## Overview

Build a mobile-first PWA for the Cold Storage Management System with:
- Offline functionality (IndexedDB + Service Worker)
- Mobile tab navigation (bottom nav bar)
- All 4 portals: Admin, Employee, Accountant, Customer
- Background sync when back online

---

## Architecture Decision

**Approach: Enhanced MPA with PWA Layer**

Rather than building a separate SPA, add PWA capabilities to existing templates:
- Keeps existing 43 templates working
- Adds Service Worker for offline caching
- Adds IndexedDB for offline data storage
- Adds mobile bottom tab navigation
- Incremental rollout possible

---

## Phase 1: PWA Foundation (3-4 days)

### 1.1 Create Web App Manifest
**File:** `/static/manifest.json`

```json
{
    "name": "Cold Storage Management",
    "short_name": "Cold Storage",
    "start_url": "/dashboard",
    "display": "standalone",
    "background_color": "#FFF9E6",
    "theme_color": "#22c55e",
    "orientation": "portrait-primary",
    "icons": [
        {"src": "/static/icons/icon-192x192.png", "sizes": "192x192", "type": "image/png"},
        {"src": "/static/icons/icon-512x512.png", "sizes": "512x512", "type": "image/png"}
    ]
}
```

### 1.2 Create App Icons
**Folder:** `/static/icons/`
- icon-72x72.png, icon-96x96.png, icon-128x128.png
- icon-144x144.png, icon-152x152.png, icon-192x192.png
- icon-384x384.png, icon-512x512.png

---

## Phase 2: Mobile Tab Navigation (2-3 days)

### Bottom Tab Bar - Portal-specific tabs:

**Employee Portal:**
| Icon | Label | Route |
|------|-------|-------|
| bi-house | Home | /dashboard |
| bi-box-arrow-in-down | Entry | /entry-room |
| bi-box-arrow-right | Gate Pass | /gate-pass-entry |
| bi-receipt | Tickets | /unloading-tickets |
| bi-person | Account | /profile |

**Accountant Portal:**
| Icon | Label | Route |
|------|-------|-------|
| bi-house | Home | /accountant |
| bi-cash | Payments | /rent-management |
| bi-receipt | Receipt | /payment-receipt |
| bi-search | Search | /item-search |

**Customer Portal:**
| Icon | Label | Route |
|------|-------|-------|
| bi-house | Home | /customer/dashboard |
| bi-box | My Items | /customer/items |
| bi-card-checklist | Gate Pass | /customer/gate-pass |

**Admin Portal:**
| Icon | Label | Route |
|------|-------|-------|
| bi-house | Home | /admin |
| bi-graph-up | Reports | /admin/reports |
| bi-gear | Settings | /system-settings |
| bi-people | Users | /employees |

---

## Phase 3: Service Worker (4-5 days)

**File:** `/static/js/sw.js`

**Caching Strategies:**
| Resource | Strategy | Reason |
|----------|----------|--------|
| CSS, Fonts, Icons | Cache First | Rarely change |
| HTML Pages | Network First | May have dynamic content |
| API GET | Stale While Revalidate | Fresh preferred, cached ok |
| API POST/PUT | Queue for Sync | Must sync when online |

---

## Phase 4: IndexedDB for Offline Data (4-5 days)

**File:** `/static/js/pwa/offline-db.js`

```javascript
const DB_STORES = {
    syncQueue: { keyPath: 'id', autoIncrement: true },
    customers: { keyPath: 'id' },
    entries: { keyPath: 'id' },
    roomEntries: { keyPath: 'id' },
    gatePasses: { keyPath: 'id' },
    rentPayments: { keyPath: 'id' },
    settings: { keyPath: 'key' },
    session: { keyPath: 'key' }
};
```

---

## Phase 5: Offline UI Components (3-4 days)

### Network Status Indicator
Shows floating indicator:
- **Offline:** Red badge "Offline Mode" + pending count
- **Syncing:** Yellow badge "Syncing X items..."
- **Online:** Hidden

---

## Phase 6: Mobile Responsive Fixes (5-6 days)

**High Priority Templates:**
1. `entry_room.html` - Change `grid-cols-2` to `grid-cols-1 lg:grid-cols-2`
2. `gate_pass_entry.html` - Same grid fix
3. `unloading_tickets.html` - Fix 10-column grid

**Global Fixes:**
- Reduce padding: `p-4 md:p-6 lg:p-8`
- Responsive text: `text-xl md:text-2xl lg:text-3xl`
- Touch-friendly buttons: min-height 44px

---

## Phase 7: Backend API Changes (2-3 days)

### Add ClientID Support for Idempotency
```go
type CreateEntryRequest struct {
    // ... existing fields ...
    ClientID string `json:"_clientId,omitempty"`
}
```

---

## File Structure

```
static/
├── manifest.json
├── icons/
│   └── icon-*.png
├── js/
│   ├── sw.js
│   └── pwa/
│       ├── sw-register.js
│       ├── offline-db.js
│       ├── sync-manager.js
│       ├── network-status.js
│       ├── mobile-tabs.js
│       └── tab-config.js
└── css/
    └── mobile.css

templates/
└── partials/
    ├── pwa-head.html
    └── mobile-tabs.html
```

---

## Estimated Timeline

| Phase | Duration |
|-------|----------|
| 1. PWA Foundation | 3-4 days |
| 2. Mobile Tabs | 2-3 days |
| 3. Service Worker | 4-5 days |
| 4. IndexedDB | 4-5 days |
| 5. Offline UI | 3-4 days |
| 6. Mobile Fixes | 5-6 days |
| 7. Backend API | 2-3 days |
| 8. Testing | 3-4 days |

**Total: 5-6 weeks**

---

## Environment Notes

**Development (localhost):**
- Service Workers work on `localhost` without HTTPS
- Can test all PWA features locally

**Production:**
- PWA requires HTTPS (already have on K3s)

---

## Success Criteria

1. App can be installed on phone home screen
2. Can create entries without internet
3. Can issue gate passes offline
4. Data syncs automatically when online
5. Bottom tabs work on all portals
6. All screens usable on 5" phone
7. Lighthouse PWA score > 90
