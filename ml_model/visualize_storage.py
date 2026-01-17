"""
ASCII Visualization of Storage Layout
=====================================
Shows how items are stored with gallery and pathways.
"""

import json
from collections import defaultdict
from storage_recommendation import GATAR_CONFIG


def load_data():
    with open('inventory_fresh.json', 'r') as f:
        return json.loads(f.read().strip())


def get_column_row(gatar_no, room_no, floor):
    """Calculate column and row position for a gatar"""
    config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
    if not config:
        return None, None

    cols = config.get('cols', 14)
    start = config.get('start', 1)

    pos = gatar_no - start
    if pos < 0:
        return None, None

    if cols == 2:  # Gallery room
        col = (pos % 2) + 1
        row = (pos // 2) + 1
    else:
        # Standard room: 14 cols x 10 rows
        col = (pos % cols) + 1
        row = (pos // cols) + 1

    return col, row


def visualize_all():
    print("=" * 80)
    print("ASCII VISUALIZATION - STORAGE LAYOUT WITH GALLERY & PATHWAYS")
    print("=" * 80)

    inventory = load_data()

    # Build grid data for each room/floor
    grids = defaultdict(lambda: defaultdict(int))

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        gate_str = str(item['gate_no']).split(',')[0].strip()

        try:
            gate_no = int(gate_str)
            col, row = get_column_row(gate_no, room, floor)
            if col and row:
                key = (room, floor)
                grids[key][(col, row)] += item['items'] or 0
        except:
            pass

    # ==========================================================================
    # FACILITY OVERVIEW
    # ==========================================================================
    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                        COLD STORAGE FACILITY - TOP VIEW                      ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║                              ┌─────────────┐                                 ║
    ║                              │   ENTRY     │                                 ║
    ║                              │   EXIT      │                                 ║
    ║                              └──────┬──────┘                                 ║
    ║                                     │                                        ║
    ║      ┌──────────────────────────────┼──────────────────────────────┐        ║
    ║      │                              │                              │        ║
    ║      │   ┌─────────┐  ┌─────────┐   │   ┌─────────┐  ┌─────────┐  │        ║
    ║      │   │         │  │         │   │   │         │  │         │  │        ║
    ║      │   │ ROOM 3  │  │ ROOM 4  │   G   │ ROOM 1  │  │ ROOM 2  │  │        ║
    ║      │   │  SELL   │  │  SELL   │   A   │  SEED   │  │  SEED   │  │        ║
    ║      │   │         │  │         │   L   │         │  │         │  │        ║
    ║      │   │ 31,447  │  │ 30,984  │   L   │ 31,254  │  │ 31,713  │  │        ║
    ║      │   │  items  │  │  items  │   E   │  items  │  │  items  │  │        ║
    ║      │   │         │  │         │   R   │         │  │         │  │        ║
    ║      │   │         │  │         │   Y   │         │  │         │  │        ║
    ║      │   └─────────┘  └─────────┘   │   └─────────┘  └─────────┘  │        ║
    ║      │                         8,247│                              │        ║
    ║      │                         items│                              │        ║
    ║      └──────────────────────────────┴──────────────────────────────┘        ║
    ║                                                                              ║
    ║      Total: 133,645 items across 5 floors (0-4)                             ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # ==========================================================================
    # ROOM LAYOUT EXPLANATION
    # ==========================================================================
    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                         ROOM LAYOUT - GALLERY ACCESS                         ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   Each Room has 14 columns × 10 rows per floor                              ║
    ║   Gallery (access path) runs through CENTER (columns 6-7-8)                 ║
    ║                                                                              ║
    ║   ┌─────────────────────────────────────────────────────────────────────┐   ║
    ║   │     DEEP          MID        GALLERY        MID          DEEP       │   ║
    ║   │   Col 1-3       Col 4-5     Col 6-7-8     Col 9-10    Col 11-14    │   ║
    ║   │                                                                     │   ║
    ║   │   ┌───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┬───┐       │   ║
    ║   │   │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │ 8 │ 9 │10 │11 │12 │13 │14 │ Row 1 │   ║
    ║   │   ├───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┤       │   ║
    ║   │   │   │   │   │   │   │ G │ A │ L │   │   │   │   │   │   │ Row 2 │   ║
    ║   │   ├───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┤       │   ║
    ║   │   │   │   │   │   │   │ L │ E │ R │   │   │   │   │   │   │ Row 3 │   ║
    ║   │   ├───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┤       │   ║
    ║   │   │   │   │   │   │   │ E │ R │ Y │   │   │   │   │   │   │ Row 4 │   ║
    ║   │   ├───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┤       │   ║
    ║   │   │   │   │   │   │   │ R │ Y │   │   │   │   │   │   │   │ Row 5 │   ║
    ║   │   ├───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┼───┤       │   ║
    ║   │   │   │   │   │   │   │ Y │   │   │   │   │   │   │   │   │...    │   ║
    ║   │   └───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┴───┘       │   ║
    ║   │       ↑               ↑       ↑       ↑               ↑           │   ║
    ║   │     DEEP            MID    GALLERY   MID            DEEP          │   ║
    ║   │   Large qty      Medium    Small    Medium       Large qty        │   ║
    ║   │    (50+)         (20-50)   (1-20)   (20-50)        (50+)          │   ║
    ║   └─────────────────────────────────────────────────────────────────────┘   ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # ==========================================================================
    # VISUALIZE EACH ROOM - FLOOR 0 (Sample)
    # ==========================================================================

    def get_density_char(items):
        """Return character based on item density"""
        if items == 0:
            return '·'
        elif items < 50:
            return '░'
        elif items < 100:
            return '▒'
        elif items < 200:
            return '▓'
        else:
            return '█'

    def print_room_grid(room, floor, grids):
        """Print ASCII grid for a room/floor"""
        key = (room, floor)
        grid = grids.get(key, {})

        config = GATAR_CONFIG.get(room, {}).get(floor, {})
        cols = config.get('cols', 14) if config else 14
        rows = 10

        if room == 'G':
            cols = 2
            rows = 15

        # Calculate totals
        total_items = sum(grid.values())

        print(f"\n    ROOM {room} - FLOOR {floor} ({total_items:,} items)")
        print("    " + "─" * (cols * 4 + 10))

        # Header with column numbers
        header = "    Col: "
        for c in range(1, cols + 1):
            if room != 'G' and c in [6, 7, 8]:
                header += f"[{c:2}]"  # Gallery columns
            else:
                header += f" {c:2} "
        print(header)

        # Gallery indicator
        if room != 'G':
            gallery_line = "         "
            for c in range(1, cols + 1):
                if c in [6, 7, 8]:
                    gallery_line += " ▼▼ "
                else:
                    gallery_line += "    "
            print(gallery_line)

        # Grid rows
        for r in range(1, min(rows + 1, 11)):  # Show max 10 rows
            row_str = f"    R{r:2}: "
            for c in range(1, cols + 1):
                items = grid.get((c, r), 0)
                char = get_density_char(items)

                if room != 'G' and c in [6, 7, 8]:
                    row_str += f"[{char}{char}]"
                else:
                    row_str += f" {char}{char} "

            # Row total
            row_total = sum(grid.get((c, r), 0) for c in range(1, cols + 1))
            row_str += f"  │ {row_total:,}"
            print(row_str)

        # Column totals
        print("    " + "─" * (cols * 4 + 10))
        total_line = "   Total:"
        for c in range(1, cols + 1):
            col_total = sum(grid.get((c, r), 0) for r in range(1, rows + 1))
            if col_total > 999:
                total_line += f"{col_total//1000}k  "
            else:
                total_line += f"{col_total:3} "
        print(total_line)

        print(f"\n    Legend: · empty  ░ <50  ▒ 50-100  ▓ 100-200  █ >200 items")
        print(f"    [##] = Gallery columns (preferred for small quantities)")

    # Print Room 1, Floor 0
    print("\n" + "=" * 80)
    print("DETAILED ROOM VIEWS - FLOOR 0 (Ground Floor)")
    print("=" * 80)

    for room in ['1', '2', '3', '4', 'G']:
        print_room_grid(room, '0', grids)

    # ==========================================================================
    # RECOMMENDATION SYSTEM - HOW IT WOULD PLACE ITEMS
    # ==========================================================================
    print("\n" + "=" * 80)
    print("HOW RECOMMENDATION SYSTEM PLACES ITEMS")
    print("=" * 80)

    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                    SMALL QUANTITY (1-20 bags) PLACEMENT                      ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   System places NEAR GALLERY for quick access:                              ║
    ║                                                                              ║
    ║   Col:  1    2    3    4    5   [6]  [7]  [8]   9   10   11   12   13   14  ║
    ║        ·    ·    ·    ·    ·   [██] [██] [██]   ·    ·    ·    ·    ·    ·  ║
    ║        ·    ·    ·    ·    ·   [██] [██] [██]   ·    ·    ·    ·    ·    ·  ║
    ║        ·    ·    ·    ·    ·   [██] [██] [██]   ·    ·    ·    ·    ·    ·  ║
    ║                                                                              ║
    ║   ✓ Quick retrieval - 0 walking distance                                    ║
    ║   ✓ Frequently picked up items easily accessible                            ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝

    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                   MEDIUM QUANTITY (20-50 bags) PLACEMENT                     ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   System places in MID COLUMNS (4-5, 9-10):                                 ║
    ║                                                                              ║
    ║   Col:  1    2    3   [4]  [5]   6    7    8   [9]  [10]  11   12   13   14 ║
    ║        ·    ·    ·   [▓▓] [▓▓]  ░░   ░░   ░░  [▓▓] [▓▓]   ·    ·    ·    · ║
    ║        ·    ·    ·   [▓▓] [▓▓]  ░░   ░░   ░░  [▓▓] [▓▓]   ·    ·    ·    · ║
    ║        ·    ·    ·   [▓▓] [▓▓]  ░░   ░░   ░░  [▓▓] [▓▓]   ·    ·    ·    · ║
    ║                                                                              ║
    ║   ✓ Moderate walking distance (2-4 meters)                                  ║
    ║   ✓ Good balance of access and space utilization                            ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝

    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                    LARGE QUANTITY (50+ bags) PLACEMENT                       ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   System places in DEEP COLUMNS (1-3, 11-14):                               ║
    ║                                                                              ║
    ║   Col: [1]  [2]  [3]   4    5    6    7    8    9   10  [11] [12] [13] [14] ║
    ║       [██] [██] [██]  ▓▓   ▓▓   ░░   ░░   ░░   ▓▓   ▓▓  [██] [██] [██] [██]║
    ║       [██] [██] [██]  ▓▓   ▓▓   ░░   ░░   ░░   ▓▓   ▓▓  [██] [██] [██] [██]║
    ║       [██] [██] [██]  ▓▓   ▓▓   ░░   ░░   ░░   ▓▓   ▓▓  [██] [██] [██] [██]║
    ║                                                                              ║
    ║   ✓ Longer walking distance (5-8 meters) but fewer trips                    ║
    ║   ✓ Stable long-term storage                                                ║
    ║   ✓ All items still directly accessible from walkway                        ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # ==========================================================================
    # FULL FACILITY - ALL FLOORS
    # ==========================================================================
    print("\n" + "=" * 80)
    print("FULL FACILITY SUMMARY - ALL FLOORS")
    print("=" * 80)

    print("""
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                           FLOOR 4 (Top - Hardest Access)                   │
    │  ┌─────────┐ ┌─────────┐ ┌───┐ ┌─────────┐ ┌─────────┐                    │
    │  │ Room 3  │ │ Room 4  │ │ G │ │ Room 1  │ │ Room 2  │  Total: 23,129    │
    │  │  4,673  │ │  5,021  │ │1.4k│ │  5,878  │ │  6,080  │  items            │
    │  └─────────┘ └─────────┘ └───┘ └─────────┘ └─────────┘                    │
    ├────────────────────────────────────────────────────────────────────────────┤
    │                           FLOOR 3                                          │
    │  ┌─────────┐ ┌─────────┐ ┌───┐ ┌─────────┐ ┌─────────┐                    │
    │  │ Room 3  │ │ Room 4  │ │ G │ │ Room 1  │ │ Room 2  │  Total: 27,529    │
    │  │  6,507  │ │  6,195  │ │1.7k│ │  6,424  │ │  6,656  │  items            │
    │  └─────────┘ └─────────┘ └───┘ └─────────┘ └─────────┘                    │
    ├────────────────────────────────────────────────────────────────────────────┤
    │                           FLOOR 2                                          │
    │  ┌─────────┐ ┌─────────┐ ┌───┐ ┌─────────┐ ┌─────────┐                    │
    │  │ Room 3  │ │ Room 4  │ │ G │ │ Room 1  │ │ Room 2  │  Total: 27,916    │
    │  │  6,647  │ │  6,432  │ │1.7k│ │  6,482  │ │  6,638  │  items            │
    │  └─────────┘ └─────────┘ └───┘ └─────────┘ └─────────┘                    │
    ├────────────────────────────────────────────────────────────────────────────┤
    │                           FLOOR 1                                          │
    │  ┌─────────┐ ┌─────────┐ ┌───┐ ┌─────────┐ ┌─────────┐                    │
    │  │ Room 3  │ │ Room 4  │ │ G │ │ Room 1  │ │ Room 2  │  Total: 27,134    │
    │  │  6,668  │ │  6,456  │ │1.4k│ │  6,186  │ │  6,406  │  items            │
    │  └─────────┘ └─────────┘ └───┘ └─────────┘ └─────────┘                    │
    ├────────────────────────────────────────────────────────────────────────────┤
    │                           FLOOR 0 (Ground - Easiest Access)                │
    │  ┌─────────┐ ┌─────────┐ ┌───┐ ┌─────────┐ ┌─────────┐                    │
    │  │ Room 3  │ │ Room 4  │ │ G │ │ Room 1  │ │ Room 2  │  Total: 27,937    │
    │  │  6,952  │ │  6,880  │ │1.9k│ │  6,284  │ │  5,933  │  items            │
    │  └─────────┘ └─────────┘ └───┘ └─────────┘ └─────────┘                    │
    └────────────────────────────────────────────────────────────────────────────┘

                              GALLERY runs through center
                                    ↑
                         ═══════════════════════════
                         All floors connected via Gallery
    """)

    # ==========================================================================
    # ACCESS PATH VISUALIZATION
    # ==========================================================================
    print("\n" + "=" * 80)
    print("ACCESS PATH - HOW WORKERS RETRIEVE ITEMS")
    print("=" * 80)

    print("""
    ┌────────────────────────────────────────────────────────────────────────────┐
    │                                                                            │
    │   ENTRY ──→ MAIN GALLERY ──→ ROOM ──→ ROW WALKWAY ──→ GATAR              │
    │                                                                            │
    │   Step 1: Enter facility                                                   │
    │           ↓                                                                │
    │   Step 2: Walk through Main Gallery (center of building)                  │
    │           ↓                                                                │
    │   Step 3: Enter desired Room (1, 2, 3, 4, or G)                           │
    │           ↓                                                                │
    │   Step 4: Go to correct Floor (0-4)                                       │
    │           ↓                                                                │
    │   Step 5: Walk along Row Walkway                                          │
    │           ↓                                                                │
    │   Step 6: Stop at Column, access Gatar directly                           │
    │                                                                            │
    │   ┌─────────────────────────────────────────────────────────────────────┐ │
    │   │                                                                     │ │
    │   │  MAIN        Row Walkway                                           │ │
    │   │ GALLERY ════╦════════════════════════════════════════════════      │ │
    │   │      │      ║  [G1] [G2] [G3] [G4] [G5] [G6] [G7]                  │ │
    │   │      │      ║   ↑    ↑    ↑    ↑    ↑    ↑    ↑                    │ │
    │   │      │      ║  All gatars accessible from walkway                  │ │
    │   │      │      ╠════════════════════════════════════════════════      │ │
    │   │      │      ║  [G8] [G9] [G10][G11][G12][G13][G14]                 │ │
    │   │      │      ╠════════════════════════════════════════════════      │ │
    │   │      │      ║   ... more rows ...                                  │ │
    │   │                                                                     │ │
    │   └─────────────────────────────────────────────────────────────────────┘ │
    │                                                                            │
    └────────────────────────────────────────────────────────────────────────────┘
    """)

    print("\n" + "=" * 80)
    print("VISUALIZATION COMPLETE")
    print("=" * 80)
    print("""
    Summary:
    ─────────────────────────────────────────────────────────
    • Total Items: 133,645 across 5 floors
    • Gallery: Center columns (6-7-8) - quick access
    • Deep: Edge columns (1-3, 11-14) - large quantities
    • All gatars accessible via row walkways
    • No item blocks another item
    """)


if __name__ == "__main__":
    visualize_all()
