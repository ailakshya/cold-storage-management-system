"""
Analyze Gallery and Access Paths in Real Inventory Data
========================================================
Find where the galleries (access paths) are based on actual storage patterns.
"""

import json
from collections import defaultdict
from storage_recommendation import GATAR_CONFIG


def load_fresh_data():
    with open('inventory_fresh.json', 'r') as f:
        inventory = json.loads(f.read().strip())
    return inventory


def analyze_gallery_paths():
    print("=" * 70)
    print("GALLERY & ACCESS PATH ANALYSIS - Real Inventory Data")
    print("=" * 70)

    inventory = load_fresh_data()

    # =========================================================================
    # ANALYSIS 1: Understand Gatar Numbering Scheme
    # =========================================================================
    print("\n" + "─" * 70)
    print("1. GATAR NUMBERING SCHEME (from GATAR_CONFIG)")
    print("─" * 70)

    print("\nRoom Layout with Gatar Ranges:")
    print(f"{'Room':<8} {'Floor':<8} {'Start':<10} {'End':<10} {'Cols':<8} {'Rows':<8} {'Total'}")
    print("─" * 70)

    for room in ['1', '2', 'G', '3', '4']:
        for floor in ['0', '1', '2', '3', '4']:
            config = GATAR_CONFIG.get(room, {}).get(floor, {})
            if config:
                start = config.get('start', 0)
                end = config.get('end', 0)
                cols = config.get('cols', 0)
                rows = config.get('rows', 0)
                total = end - start + 1
                print(f"{room:<8} {floor:<8} {start:<10} {end:<10} {cols:<8} {rows:<8} {total}")

    # =========================================================================
    # ANALYSIS 2: Visualize Room Layout with Gallery
    # =========================================================================
    print("\n" + "─" * 70)
    print("2. ROOM LAYOUT VISUALIZATION")
    print("─" * 70)

    print("""
    Physical Layout (Top View):
    ═══════════════════════════════════════════════════════════════════════

                    ENTRY/EXIT
                        ↓
    ┌─────────────────────────────────────────────────────────────────────┐
    │                                                                     │
    │   ┌─────────┐   ┌─────────┐       ┌─────────┐   ┌─────────┐       │
    │   │         │   │         │       │         │   │         │       │
    │   │ ROOM 3  │   │ ROOM 4  │       │ ROOM 1  │   │ ROOM 2  │       │
    │   │ (SELL)  │   │ (SELL)  │   G   │ (SEED)  │   │ (SEED)  │       │
    │   │         │   │         │   A   │         │   │         │       │
    │   │  Cols   │   │  Cols   │   L   │  Cols   │   │  Cols   │       │
    │   │ 1←──→14 │   │ 1←──→14 │   L   │ 1←──→14 │   │ 1←──→14 │       │
    │   │         │   │         │   E   │         │   │         │       │
    │   │ Gatars  │   │ Gatars  │   R   │ Gatars  │   │ Gatars  │       │
    │   │1361-1500│   │2041-2160│   Y   │  1-140  │   │ 681-820 │       │
    │   │(Floor 0)│   │(Floor 0)│       │(Floor 0)│   │(Floor 0)│       │
    │   └─────────┘   └─────────┘       └─────────┘   └─────────┘       │
    │                                                                     │
    └─────────────────────────────────────────────────────────────────────┘

    Gallery Path runs through center of each room (Cols 6-7-8)
    """)

    # =========================================================================
    # ANALYSIS 3: Column Distribution in Real Data
    # =========================================================================
    print("\n" + "─" * 70)
    print("3. COLUMN DISTRIBUTION IN REAL DATA")
    print("─" * 70)

    def get_column_from_gatar(gatar_no, room_no, floor):
        """Calculate column position for a gatar"""
        config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
        if not config:
            return None

        cols = config.get('cols', 14)
        start = config.get('start', 1)

        if cols == 2:  # Gallery room
            return 1

        pos_in_floor = gatar_no - start
        if pos_in_floor < 0:
            return None

        # Calculate column (1-14)
        # Each column pair has ~10 gatars (rows)
        pair_index = pos_in_floor // 20
        col = (pair_index * 2) + 1 + (pos_in_floor % 2)

        return min(col, cols)

    # Analyze column distribution per room
    room_col_items = defaultdict(lambda: defaultdict(int))
    room_col_gatars = defaultdict(lambda: defaultdict(int))

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        gate_no_str = str(item['gate_no']).split(',')[0].strip()

        try:
            gate_no = int(gate_no_str)
            col = get_column_from_gatar(gate_no, room, floor)
            if col:
                room_col_items[room][col] += item['items'] or 0
                room_col_gatars[room][col] += 1
        except:
            pass

    # Print column distribution for each room
    for room in ['1', '2', '3', '4']:
        print(f"\nRoom {room} - Column Distribution:")
        print(f"{'Col':<6} {'Gatars':<10} {'Items':<12} {'Bar'}")
        print("─" * 60)

        max_items = max(room_col_items[room].values()) if room_col_items[room] else 1

        for col in range(1, 15):
            gatars = room_col_gatars[room].get(col, 0)
            items = room_col_items[room].get(col, 0)
            bar_len = int(items / max_items * 30) if max_items > 0 else 0
            bar = "█" * bar_len

            # Mark gallery columns
            gallery_marker = " ← GALLERY" if col in [6, 7, 8] else ""
            print(f"{col:<6} {gatars:<10} {items:<12,} {bar}{gallery_marker}")

    # =========================================================================
    # ANALYSIS 4: Gallery Room (G) Analysis
    # =========================================================================
    print("\n" + "─" * 70)
    print("4. GALLERY ROOM (G) ANALYSIS")
    print("─" * 70)

    gallery_items = []
    for item in inventory:
        if item['room_no'] == 'G':
            gallery_items.append(item)

    print(f"\nGallery Room Statistics:")
    print(f"  Total gatars used: {len(gallery_items)}")
    print(f"  Total items: {sum(i['items'] or 0 for i in gallery_items):,}")

    # Floor distribution in gallery
    gallery_floor_items = defaultdict(int)
    for item in gallery_items:
        gallery_floor_items[item['floor']] += item['items'] or 0

    print(f"\n  Floor Distribution in Gallery:")
    for floor in ['0', '1', '2', '3', '4']:
        items = gallery_floor_items.get(floor, 0)
        print(f"    Floor {floor}: {items:,} items")

    # =========================================================================
    # ANALYSIS 5: Access Path Identification
    # =========================================================================
    print("\n" + "─" * 70)
    print("5. ACCESS PATH IDENTIFICATION")
    print("─" * 70)

    print("""
    Based on the data analysis, the ACCESS PATHS are:

    ┌─────────────────────────────────────────────────────────────────────┐
    │                                                                     │
    │  Each Room (14 columns × 10 rows per floor):                       │
    │                                                                     │
    │    Col: 1  2  3  4  5  6  7  8  9 10 11 12 13 14                   │
    │         ├──┴──┤  ├──┴──┤  ├──┴──┤  ├──┴──┤  ├──┴──┤  ├──┴──┤  ├──┴──┤
    │         Deep     Mid     GALLERY  Mid     Deep                      │
    │                          ACCESS                                     │
    │                          PATH                                       │
    │                                                                     │
    │  Distance from Gallery:                                             │
    │    Cols 6-7-8:  0 (Gallery/Access Path)                            │
    │    Cols 5,9:    1 column away                                       │
    │    Cols 4,10:   2 columns away                                      │
    │    Cols 3,11:   3 columns away                                      │
    │    Cols 2,12:   4 columns away                                      │
    │    Cols 1,13-14: 5-6 columns away (Deepest)                        │
    │                                                                     │
    └─────────────────────────────────────────────────────────────────────┘
    """)

    # Calculate items by distance from gallery
    distance_items = defaultdict(int)
    distance_gatars = defaultdict(int)

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        gate_no_str = str(item['gate_no']).split(',')[0].strip()

        try:
            gate_no = int(gate_no_str)
            col = get_column_from_gatar(gate_no, room, floor)
            if col:
                config = GATAR_CONFIG.get(room, {}).get(floor, {})
                cols = config.get('cols', 14)

                if cols == 2:  # Gallery room
                    distance = 0
                else:
                    # Gallery is at center (cols 6-7-8 for 14-col room)
                    gallery_center = 7
                    if col <= gallery_center:
                        distance = gallery_center - col
                    else:
                        distance = col - gallery_center

                distance_items[distance] += item['items'] or 0
                distance_gatars[distance] += 1
        except:
            pass

    print("\nItems by Distance from Gallery Access Path:")
    print(f"{'Distance':<12} {'Columns':<20} {'Gatars':<10} {'Items':<12} {'%'}")
    print("─" * 70)

    total_items = sum(distance_items.values())
    col_mapping = {
        0: "6, 7, 8 (Gallery)",
        1: "5, 9",
        2: "4, 10",
        3: "3, 11",
        4: "2, 12",
        5: "1, 13",
        6: "14",
    }

    for dist in sorted(distance_items.keys()):
        cols_str = col_mapping.get(dist, f"Col {dist}")
        gatars = distance_gatars[dist]
        items = distance_items[dist]
        pct = (items / total_items * 100) if total_items > 0 else 0
        print(f"{dist:<12} {cols_str:<20} {gatars:<10} {items:<12,} {pct:.1f}%")

    # =========================================================================
    # ANALYSIS 6: Visual Grid of Real Data
    # =========================================================================
    print("\n" + "─" * 70)
    print("6. VISUAL GRID - Room 1, Floor 0 (Sample)")
    print("─" * 70)

    # Build grid for Room 1, Floor 0
    grid = {}
    for item in inventory:
        if item['room_no'] == '1' and item['floor'] == '0':
            gate_no_str = str(item['gate_no']).split(',')[0].strip()
            try:
                gate_no = int(gate_no_str)
                col = get_column_from_gatar(gate_no, '1', '0')
                config = GATAR_CONFIG.get('1', {}).get('0', {})
                start = config.get('start', 1)
                pos = gate_no - start
                row = (pos // 2) % 10 + 1  # Simplified row calculation

                if col and row:
                    key = (col, row)
                    if key not in grid:
                        grid[key] = 0
                    grid[key] += item['items'] or 0
            except:
                pass

    print("""
    Room 1, Floor 0 - Item Distribution (simplified view):

         Col:  1    2    3    4    5   [6    7    8]   9   10   11   12   13   14
               │    │    │    │    │    │ GALLERY │    │    │    │    │    │
    """)

    # Show which columns have the most items
    col_totals = defaultdict(int)
    for (col, row), items in grid.items():
        col_totals[col] += items

    print("    Items: ", end="")
    for col in range(1, 15):
        items = col_totals.get(col, 0)
        if items > 1000:
            marker = "███"
        elif items > 500:
            marker = "██ "
        elif items > 100:
            marker = "█  "
        elif items > 0:
            marker = "▪  "
        else:
            marker = "·  "

        if col in [6, 7, 8]:
            print(f"[{marker}]", end="")
        else:
            print(f" {marker} ", end="")
    print()

    print("\n    Legend: ███ = >1000 items, ██ = >500, █ = >100, ▪ = <100, · = empty")

    # =========================================================================
    # FINAL SUMMARY
    # =========================================================================
    print("\n" + "=" * 70)
    print("GALLERY ACCESS PATH SUMMARY")
    print("=" * 70)

    near_gallery = distance_items.get(0, 0) + distance_items.get(1, 0)
    far_from_gallery = sum(distance_items.get(d, 0) for d in range(4, 10))

    print(f"""
    ACCESS PATH STRUCTURE:
    ──────────────────────
    • Gallery runs through CENTER of each room (Columns 6-7-8)
    • Room G is the central gallery room (2 columns only)
    • Each room has 14 columns × 10 rows per floor
    • Items accessed from gallery path, then retrieved left/right

    CURRENT STORAGE ANALYSIS:
    ─────────────────────────
    • Items NEAR gallery (0-1 cols):  {near_gallery:,} ({near_gallery/total_items*100:.1f}%)
    • Items FAR from gallery (4+ cols): {far_from_gallery:,} ({far_from_gallery/total_items*100:.1f}%)

    RECOMMENDATION:
    ───────────────
    • Small quantities (1-20 bags): Place in columns 6-7-8 (gallery)
    • Medium quantities (20-50 bags): Place in columns 4-5 or 9-10
    • Large quantities (50+ bags): Can go in columns 1-3 or 11-14
    """)


if __name__ == "__main__":
    analyze_gallery_paths()
