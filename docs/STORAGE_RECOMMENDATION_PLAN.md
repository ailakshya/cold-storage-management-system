# AI/ML Storage Location Recommendation System - Plan

## Objective
Create an intelligent system that suggests optimal storage locations for items, making retrieval easy regardless of item count.

---

## Current Storage Structure

### Facility Capacity
- **Total Capacity:** 140,000 items
- **Current Stock:** 133,870 items (95.6% utilized)
- **Available Space:** ~6,130 items (4.4%)
- **Target with Recommendation System:** +5% efficiency = ~140,000 items (100%)

### Room Configuration

| Room | Category | Floors 0-3 | Floor 4 | Total Gatars | Current Items |
|------|----------|------------|---------|--------------|---------------|
| 1    | Seed     | 140 each   | 120     | 680          | 31,304        |
| 2    | Seed     | 140 each   | 120     | 680          | 31,737        |
| 3    | Sell     | 140 each   | 120     | 680          | 31,492        |
| 4    | Sell     | 140 each   | 120     | 680          | 31,090        |
| G    | Sell     | 28-30 each | 28      | 142          | 8,247         |
| **Total** |     |            |         | **2,862**    | **133,870**   |

### Gatar Ranges by Room/Floor

| Room | Floor 0 | Floor 1 | Floor 2 | Floor 3 | Floor 4 |
|------|---------|---------|---------|---------|---------|
| 1    | 1-140   | 141-280 | 281-420 | 421-560 | 561-680 |
| 2    | 681-820 | 821-960 | 961-1100| 1101-1240| 1241-1360|
| 3    | 1361-1500| 1501-1640| 1641-1780| 1781-1920| 1921-2040|
| 4    | 2041-2120 + 2869-2928 | 2121-2260| 2261-2400| 2401-2540| 2601-2720|
| G    | 2727-2756| 2757-2784| 2785-2812| 2813-2840| 2841-2868|

### Average Capacity per Gatar
- **Target:** ~49 items per gatar (140,000 ÷ 2,862)
- **Current avg:** ~47 items per gatar (133,870 ÷ 2,862)

**Note:** Seed and Sell items are loaded **simultaneously** - categories shown are preferences, not restrictions.

---

## Physical Constraints

### Item Weight
- **Weight per item:** 55-65 kg (avg ~60 kg)
- **Total weight at capacity:** ~8,520 tonnes (142,000 × 60 kg)

### Current Inventory by Room/Floor

| Room | Floor 0 | Floor 1 | Floor 2 | Floor 3 | Floor 4 | Total |
|------|---------|---------|---------|---------|---------|-------|
| 1    | 6,390   | 6,479   | 6,605   | 6,518   | 5,312   | 31,304 |
| 2    | 6,690   | 6,722   | 6,588   | 6,337   | 5,400   | 31,737 |
| 3    | 6,631   | 6,397   | 6,514   | 6,411   | 5,539   | 31,492 |
| 4    | 6,369   | 6,124   | 6,598   | 6,519   | 5,480   | 31,090 |
| G    | 1,884   | 1,420   | 1,717   | 1,749   | 1,477   | 8,247  |

### Why Floor 4 Has Fewer Items

**Floor 4 has 120 gatars vs 140 gatars on Floors 0-3** (fewer gatars, not less capacity per gatar)

| Floor | Gatars (Main Rooms) | Gatars (Gallery) | Max Items |
|-------|---------------------|------------------|-----------|
| 0     | 140                 | 30               | ~7,000    |
| 1     | 140                 | 28               | ~7,000    |
| 2     | 140                 | 28               | ~7,000    |
| 3     | 140                 | 28               | ~7,000    |
| 4     | **120**             | 28               | **~6,000** |

### Floor-Based Recommendations

| Floor | Gatars | Accessibility | Recommendation |
|-------|--------|---------------|----------------|
| 0     | 140    | Easiest       | Best for frequent retrieval items |
| 1     | 140    | Easy          | Good for any items |
| 2     | 140    | Medium        | Good for any items |
| 3     | 140    | Hard          | Better for long-term storage |
| 4     | 120    | Hardest       | Long-term storage, less space |

**Gallery (G):** 28-30 gatars per floor, ~1,500-1,900 items capacity per floor

### Physical Room Layout (from UI)

```
┌─────────────────────────────────────────────────────────┐
│                     BUILDING LAYOUT                      │
├─────────────────────────────────────────────────────────┤
│                                                         │
│    ┌──────────────┐         ┌──────────────┐           │
│    │   ROOM 2     │         │   ROOM 1     │           │
│    │  (Seed)      │         │  (Seed)      │           │
│    │  Green       │         │  Green       │           │
│    └──────────────┘         └──────────────┘           │
│                                                         │
│              ┌──────────────────────┐                   │
│              │      GALLERY         │                   │
│              │      (Sell)          │                   │
│              │      Orange          │                   │
│              └──────────────────────┘                   │
│                                                         │
│    ┌──────────────┐         ┌──────────────┐           │
│    │   ROOM 3     │         │   ROOM 4     │           │
│    │  (Sell)      │         │  (Sell)      │           │
│    │  Orange      │         │  Orange      │           │
│    └──────────────┘         └──────────────┘           │
│                                                         │
│                      [EXIT]                             │
└─────────────────────────────────────────────────────────┘
```

### Filling Pattern (Flexible - Not Strict)

**Important:** The filling is done by employees who can choose any location. The recommendation system only **suggests** optimal locations - it does NOT enforce rules.

**Typical Filling Patterns:**
1. **Rooms 2 + 4 together**, then **Rooms 1 + 3** (diagonal pattern)
2. **Floors can be filled simultaneously** across rooms
3. **Seed and Sell items loaded at the same time** (not separately)

### Weight Distribution Concept (CPU Cooler Method)

**For understanding only:** Like tightening CPU cooler screws in a diagonal/cross pattern - this concept explains why we should distribute weight evenly across the floor rather than concentrating in one area.

```
Concept Visualization:
┌─────────────────────────────────────────┐
│ [1]  .   .   .   .   .   .   .   .  [2] │  ← Start at opposite corners
│  .   .   .   .   .   .   .   .   .   .  │
│  .   .   .   .   .   .   .   .   .   .  │
│  .   .   .   .  [5]  .   .   .   .   .  │  ← Then fill center
│  .   .   .   .   .   .   .   .   .   .  │
│  .   .   .   .   .   .   .   .   .   .  │
│ [3]  .   .   .   .   .   .   .   .  [4] │  ← Then other corners
└─────────────────────────────────────────┘
```

**The algorithm uses this concept to RECOMMEND locations that:**
- Balance weight across the floor
- Don't overload any single section
- Consider building structure safety

### Weight Distribution Recommendations (Not Rules)

1. **Prefer even distribution** - Suggest locations that balance existing weight
2. **Room G for heavy/long-term** - Gallery has maximum load capacity
3. **Room 4 for lighter loads** - Minimum capacity, suggest when others are full
4. **Warn if section overloaded** - Show warning but don't block

### Stress Calculation

```go
// Calculate floor section stress
func calculateSectionStress(items int, avgWeight float64) float64 {
    totalWeight := float64(items) * avgWeight  // kg
    // Section is 1/4 of floor (30 gatars)
    // Max safe load per section varies by room
    return totalWeight
}

// Room load limits (kg per floor section)
var maxSectionLoad = map[string]float64{
    "G": 180000,  // 3000 items × 60kg (strongest)
    "1": 150000,  // 2500 items × 60kg
    "2": 150000,  // 2500 items × 60kg
    "3": 150000,  // 2500 items × 60kg
    "4": 120000,  // 2000 items × 60kg (weakest)
}
```

---

## Room Layout Structure

### Grid Layout (Per Floor)
Each room floor has a **12 columns × 10 rows** grid layout:
- **Columns:** 12 vertical sections
- **Rows:** 10 horizontal sections
- **Gatars per floor:** 120 gatars (12 × 10)

### Gatar Numbering by Room/Floor

| Room | Floor | Gatar Range | Total Gatars |
|------|-------|-------------|--------------|
| G    | 0-4   | 1-600       | 600          |
| 1    | 0-4   | 601-1200    | 600          |
| 2    | 0-4   | 1201-1800   | 600          |
| 3    | 0-4   | 1801-2400   | 600          |
| 4    | 0-4   | 2401-2720   | 600          |

### Gallery System (Pathways)

**Galleries are pathways within rooms used for:**
1. **Loading:** Path to fill rooms with items
2. **Unloading:** Path to retrieve items during gate pass pickup

**Gallery Gatars:** 2727-2868 (142 gatars across 5 floors)

| Floor | Gallery Gatars | Count |
|-------|----------------|-------|
| 0     | 2727-2754      | 28    |
| 1     | 2755-2783      | 29    |
| 2     | 2784-2812      | 29    |
| 3     | 2813-2840      | 28    |
| 4     | 2841-2868      | 28    |

**Gallery Impact on Accessibility:**
- Items **near galleries** = easier to access (don't need to move other items)
- Items **far from galleries** = harder to access (may require moving items)
- Galleries run through the middle/edges of room floors

### Visual Layout Reference
```
Room Layout (12×10 Grid with Gallery)
┌──────────────────────────────────────────────────┐
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │ ← Gallery (║)
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
│  [1] [2] [3] [4] [5] [6] ║ [7] [8] [9] [10][11][12] │
└──────────────────────────────────────────────────┘
```

**Layout files available at:** `lakshya@192.168.15.195:/home/lakshya/jupyter-/cold/room_layout_with_gatar_no/`

---

## Recommendation Algorithm Factors

### 1. Load Balance Score (25%) - SAFETY CRITICAL
*Ensures weight is distributed evenly to protect building structure*

**Section Balance Check:**
- Divide each floor into 4 quadrants (30 gatars each)
- Calculate weight in each quadrant
- Score based on balance across quadrants

| Condition | Score |
|-----------|-------|
| All quadrants within 20% of each other | 1.0 |
| Quadrants within 30% variance | 0.8 |
| Quadrants within 50% variance | 0.5 |
| One quadrant > 50% heavier | 0.2 |
| Adding here would exceed room load limit | 0.0 (SKIP) |

**Room Load Priority:**
- **Room G:** Prioritize for heavy/seed items (strongest structure)
- **Rooms 1-3:** Medium load, prefer sell items
- **Room 4:** Light load only, use last

**Diagonal Fill Pattern:**
- Prefer gatars that continue the CPU cooler pattern
- Score higher for positions that balance existing weight

### 2. Customer Clustering Score (25%)
- **Same location as existing items:** Score 1.0
- **Same room, different floor:** Score 0.7
- **Same floor, different room:** Score 0.4
- **New customer:** Use accessibility score

### 3. Gallery Proximity Score (20%)
*ALL retrievals happen through the gallery - items must be accessible from gallery path*

**Key Principle:** Place items so ANY item can be taken out easily from gallery

**Quantity-Based Placement:**

| Quantity | Recommended Location | Reason |
|----------|---------------------|--------|
| Small (1-20 bags) | **Near gallery** (cols 6-7) | Not worth moving items for few bags |
| Medium (21-50 bags) | **1-2 cols from gallery** (cols 4-5, 8-9) | Moderate effort acceptable |
| Large (50+ bags) | **Can go deeper** (cols 1-3, 10-12) | Worth the effort, taking many bags |

**Scoring by Position + Quantity:**

```
For SMALL quantities (1-20 bags):
- Adjacent to gallery (cols 6-7): Score 1.0
- 1 col away: Score 0.6
- 2+ cols away: Score 0.2 (avoid deep placement)

For MEDIUM quantities (21-50 bags):
- Adjacent to gallery: Score 1.0
- 1-2 cols away: Score 0.8
- 3+ cols away: Score 0.5

For LARGE quantities (50+ bags):
- Any location acceptable: Score 0.7-1.0
- Can fill deep spots: Score 0.8
```

**Visual: Gallery Access Pattern**
```
    DEEP ←──────────────── GALLERY ──────────────→ DEEP
┌─────────────────────────────────────────────────────────┐
│ [Large] [Large] [Med] [Med] [Small] ║ [Small] [Med] [Med] [Large] [Large] │
│   1       2      3     4      5    ║    7      8     9      10     11    │
│                              ←─────║─────→                               │
│                           DIRECT ACCESS                                  │
└─────────────────────────────────────────────────────────┘
```

**CRITICAL RULES:**
1. **NO moving ANY items** during retrieval
2. **Use ONLY pre-defined gallery paths** - no extra paths
3. All items accessible through existing gallery structure

**Pre-defined Gallery Path Layout:**
```
┌─────────────────────────────────────────────────────┐
│  [1] [2] [3] [4] [5] [6]  ║  [7] [8] [9] [10][11][12] │
│  Row 1                    ║                    Row 1 │
│  [1] [2] [3] [4] [5] [6]  ║  [7] [8] [9] [10][11][12] │
│  Row 2                    ║                    Row 2 │
│  ...                      ║                    ...   │
│  [1] [2] [3] [4] [5] [6]  ║  [7] [8] [9] [10][11][12] │
│  Row 10                   ║                   Row 10 │
└─────────────────────────────────────────────────────┘
                            ║
                      PRE-DEFINED
                      GALLERY PATH
                      (Fixed - cannot change)
```

**Placement Strategy using existing gallery:**
- **Small quantities** → Place in gatars adjacent to gallery path (cols 6-7)
- **Large quantities** → Can place in deeper gatars (cols 1-5, 8-12)
- **Retrieval** → Walk through existing gallery, access gatar directly

**Result:** All items accessible via pre-defined gallery paths, NO items moved

### 4. Floor Score (15%)
*Combines accessibility AND load capacity*

| Floor | Access | Load Capacity | Combined Score |
|-------|--------|---------------|----------------|
| 0     | Easiest | **Maximum** | 1.0 |
| 1     | Easy | High | 0.90 |
| 2     | Medium | High | 0.80 |
| 3     | Hard | Medium | 0.60 |
| 4     | Hardest | **Minimum** | 0.40 |

**Floor 4 gets lowest score** because:
- Hardest to access (top floor)
- Lowest load capacity (~5,400 vs ~6,500)

### 5. Capacity Score (10%)
- **< 50% utilized:** Score 1.0
- **50-75% utilized:** Score 0.5
- **75-95% utilized:** Score 0.25
- **> 95% utilized:** Skip location

### 6. Room Proximity Score (5%)
*Based on actual building layout - exit at bottom*

- **Room 3:** Score 1.0 - Closest to exit (bottom left)
- **Room 4:** Score 1.0 - Closest to exit (bottom right)
- **Gallery (G):** Score 0.80 - Center of building
- **Room 1:** Score 0.60 - Farthest from exit (top right)
- **Room 2:** Score 0.60 - Farthest from exit (top left)

---

## Scoring Formula

```
Final Score = (Customer Clustering × 0.30) +
              (Gallery Proximity × 0.25) +
              (Floor Accessibility × 0.20) +
              (Capacity × 0.15) +
              (Room Proximity × 0.10)
```

**Example Calculation:**
- Customer has items in Room 1, Floor 0
- New recommendation for same location, near gallery:
  - Clustering: 1.0 × 0.30 = 0.30
  - Gallery: 1.0 × 0.25 = 0.25
  - Floor: 1.0 × 0.20 = 0.20
  - Capacity (60%): 0.5 × 0.15 = 0.075
  - Room: 0.9 × 0.10 = 0.09
  - **Final: 91.5 points**

---

## Gallery Proximity Calculation

### Determining Column from Gatar Number

Each floor has 120 gatars in a 12×10 grid. To find the column position:

```go
// Get column (1-12) from gatar number within a floor
func getColumnFromGatar(gatarInFloor int) int {
    // gatarInFloor is 1-120
    row := (gatarInFloor - 1) / 12  // 0-9
    col := (gatarInFloor - 1) % 12 + 1  // 1-12
    return col
}

// Calculate gallery proximity score
func getGalleryProximityScore(column int) float64 {
    // Gallery runs between columns 6 and 7
    distanceFromGallery := 0
    if column <= 6 {
        distanceFromGallery = 6 - column
    } else {
        distanceFromGallery = column - 7
    }

    switch distanceFromGallery {
    case 0: return 1.0   // Adjacent to gallery
    case 1: return 0.85
    case 2: return 0.70
    case 3: return 0.55
    default: return 0.40 // Far from gallery
    }
}
```

### Gatar Position Lookup Table

| Gatar Range (Floor 0) | Column | Gallery Score |
|-----------------------|--------|---------------|
| 1, 13, 25...         | 1      | 0.40          |
| 2, 14, 26...         | 2      | 0.40          |
| 3, 15, 27...         | 3      | 0.55          |
| 4, 16, 28...         | 4      | 0.70          |
| 5, 17, 29...         | 5      | 0.85          |
| 6, 18, 30...         | 6      | 1.00          |
| 7, 19, 31...         | 7      | 1.00          |
| 8, 20, 32...         | 8      | 0.85          |
| 9, 21, 33...         | 9      | 0.70          |
| 10, 22, 34...        | 10     | 0.55          |
| 11, 23, 35...        | 11     | 0.40          |
| 12, 24, 36...        | 12     | 0.40          |

---

## Implementation Phases

### Phase 1: Backend Service
**Files to create:**
- `internal/services/storage_recommendation_service.go`
- `internal/handlers/storage_recommendation_handler.go`

**API Endpoint:**
```
GET /api/storage/recommendations?customer_id=123&phone=9917585586&category=seed&quantity=50
```

**Response:**
```json
{
  "recommendations": [
    {
      "room_no": "1",
      "floor": "0",
      "suggested_gatars": [6, 7, 18, 19],
      "score": 91.5,
      "score_breakdown": {
        "customer_clustering": 30.0,
        "gallery_proximity": 25.0,
        "floor_accessibility": 20.0,
        "capacity": 7.5,
        "room_proximity": 9.0
      },
      "reason": "Customer has existing items here; Near gallery for easy retrieval",
      "current_items": 6390,
      "capacity": 8000,
      "utilization": 79.8,
      "available_near_gallery": 24
    },
    {
      "room_no": "1",
      "floor": "1",
      "suggested_gatars": [6, 7, 18, 19],
      "score": 78.2,
      "reason": "Same room as main storage; Good gallery access",
      "current_items": 6100,
      "capacity": 8000,
      "utilization": 76.3,
      "available_near_gallery": 18
    }
  ],
  "customer_pattern": {
    "total_items": 3546,
    "total_thocks": 34,
    "primary_room": "1",
    "primary_floor": "0",
    "distribution": [
      {"room": "1", "floor": "0", "items": 2100, "gatars_used": [5, 6, 7, 17, 18, 19]},
      {"room": "1", "floor": "1", "items": 1446, "gatars_used": [6, 7, 8]}
    ]
  },
  "gallery_info": {
    "room_1_floor_0": {
      "gallery_columns": [6, 7],
      "occupied_near_gallery": 42,
      "available_near_gallery": 24
    }
  }
}
```

### Phase 2: Router Integration
**File:** `internal/http/router.go`
- Add route under `/api/storage/recommendations`
- Require authentication (employee/admin)

### Phase 3: UI Integration
**File:** `templates/entry_room.html`

**Changes:**
1. Add "Get Recommendation" button near room/floor selection
2. Show recommended locations with scores
3. Allow one-click selection of recommended location
4. Show customer's existing storage pattern

**UI Mockup:**
```
┌──────────────────────────────────────────────────────────────────┐
│ 📍 AI Recommended Storage Locations                               │
├──────────────────────────────────────────────────────────────────┤
│ ⭐ Room 1, Floor 0 - Score: 91.5                                 │
│    ├─ "Customer has items here; Near gallery for easy retrieval" │
│    ├─ Suggested Gatars: 6, 7, 18, 19 (gallery adjacent)          │
│    ├─ Utilization: 79.8% | Available near gallery: 24            │
│    └─ [Select This Location]                                     │
├──────────────────────────────────────────────────────────────────┤
│ ⭐ Room 1, Floor 1 - Score: 78.2                                 │
│    ├─ "Same room as main storage; Good gallery access"           │
│    ├─ Suggested Gatars: 6, 7 (gallery adjacent)                  │
│    ├─ Utilization: 76.3% | Available near gallery: 18            │
│    └─ [Select This Location]                                     │
├──────────────────────────────────────────────────────────────────┤
│ ⭐ Room G, Floor 0 - Score: 72.1                                 │
│    ├─ "Closest to exit; Good space available"                    │
│    ├─ Suggested Gatars: 5, 6, 7, 8 (near gallery)                │
│    ├─ Utilization: 65.0% | Available near gallery: 32            │
│    └─ [Select This Location]                                     │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│ 📊 Customer Storage Pattern                                       │
├──────────────────────────────────────────────────────────────────┤
│ Total Items: 3,546 | Total Thocks: 34                            │
│ Primary Location: Room 1, Floor 0                                │
├──────────────────────────────────────────────────────────────────┤
│ Distribution:                                                    │
│   Room 1, Floor 0: 2,100 items (Gatars: 5, 6, 7, 17, 18, 19)     │
│   Room 1, Floor 1: 1,446 items (Gatars: 6, 7, 8)                 │
└──────────────────────────────────────────────────────────────────┘
```

### Phase 4: Advanced ML Features (Future)

1. **Retrieval Pattern Learning**
   - Track gate pass pickups
   - Learn which customers pick up frequently
   - Prioritize accessible locations for frequent retrievers

2. **Seasonal Patterns**
   - Seed vs Sell items have different retrieval times
   - Optimize based on expected retrieval date

3. **FIFO Optimization**
   - Older items should be more accessible
   - Suggest locations that don't block older items

4. **Batch Optimization**
   - When customer picks up, suggest consolidating remaining items
   - Reduce fragmentation

---

## Database Queries Needed

### 1. Customer Storage Pattern
```sql
SELECT room_no, floor, SUM(quantity) as items, COUNT(DISTINCT thock_number) as thocks
FROM room_entries re
JOIN entries e ON re.thock_number = e.thock_number
WHERE e.customer_id = $1
GROUP BY room_no, floor
ORDER BY items DESC;
```

### 2. Current Utilization by Room/Floor
```sql
SELECT room_no, floor, SUM(quantity) as items
FROM room_entries
GROUP BY room_no, floor;
```

### 3. Gatar-Level Occupancy (for Gallery Proximity)
```sql
SELECT
    room_no,
    floor,
    gate_no as gatar_no,
    SUM(quantity) as items,
    COUNT(DISTINCT thock_number) as thocks,
    -- Calculate column position (1-12) from gatar
    ((CAST(gate_no AS INT) - 1) % 12) + 1 as column_position
FROM room_entries
WHERE gate_no IS NOT NULL AND gate_no != ''
GROUP BY room_no, floor, gate_no
ORDER BY room_no, floor, gate_no;
```

### 4. Available Gatars Near Gallery
```sql
WITH occupied_gatars AS (
    SELECT DISTINCT room_no, floor, gate_no
    FROM room_entries
    WHERE gate_no IS NOT NULL AND gate_no != ''
),
all_gatars AS (
    SELECT room_no, floor, gatar_no
    FROM (SELECT DISTINCT room_no, floor FROM room_entries) rf
    CROSS JOIN generate_series(1, 120) as gatar_no
)
SELECT
    ag.room_no,
    ag.floor,
    ag.gatar_no,
    ((ag.gatar_no - 1) % 12) + 1 as column_position,
    CASE
        WHEN ((ag.gatar_no - 1) % 12) + 1 IN (6, 7) THEN 'adjacent'
        WHEN ((ag.gatar_no - 1) % 12) + 1 IN (5, 8) THEN 'near'
        ELSE 'far'
    END as gallery_proximity
FROM all_gatars ag
LEFT JOIN occupied_gatars og
    ON ag.room_no = og.room_no
    AND ag.floor = og.floor
    AND ag.gatar_no::text = og.gate_no
WHERE og.gate_no IS NULL  -- Only unoccupied gatars
ORDER BY ag.room_no, ag.floor,
    CASE WHEN ((ag.gatar_no - 1) % 12) + 1 IN (6, 7) THEN 0 ELSE 1 END;
```

### 5. Retrieval Frequency (Future)
```sql
SELECT e.customer_id, COUNT(gp.id) as pickup_count
FROM gate_passes gp
JOIN entries e ON gp.thock_number = e.thock_number
WHERE gp.status = 'completed'
  AND gp.completed_at > NOW() - INTERVAL '90 days'
GROUP BY e.customer_id
ORDER BY pickup_count DESC;
```

---

## Files to Modify/Create

| File | Action | Purpose |
|------|--------|---------|
| `internal/services/storage_recommendation_service.go` | Create | Core algorithm |
| `internal/handlers/storage_recommendation_handler.go` | Create | API handler |
| `internal/http/router.go` | Modify | Add route |
| `cmd/server/main.go` | Modify | Initialize service |
| `templates/entry_room.html` | Modify | Add UI |

---

## Estimated Effort

| Phase | Tasks | Complexity |
|-------|-------|------------|
| Phase 1 | Backend service | Medium |
| Phase 2 | Router integration | Low |
| Phase 3 | UI integration | Medium |
| Phase 4 | ML features | High (future) |

---

## Success Metrics

1. **Retrieval Time Reduction** - Measure average time to locate items
2. **Customer Clustering** - % of customer items in same room
3. **Space Utilization** - Even distribution across locations
4. **User Adoption** - % of entries using recommendations

---

## Notes

- Algorithm weights can be adjusted based on real-world feedback
- Start with rule-based scoring, can add ML later
- Consider caching recommendations for performance
