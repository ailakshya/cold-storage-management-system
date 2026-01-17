"""
IN STOCK OVERVIEW - Terminal Version
=====================================
Displays inventory data like the web UI's "In Stock Overview" page.
Shows all rooms, all floors, with capacity and occupancy stats.
"""

import json
from collections import defaultdict
from storage_recommendation import GATAR_CONFIG


def load_data():
    with open('inventory_fresh.json', 'r') as f:
        return json.loads(f.read().strip())


def get_density_char(qty):
    """Return density indicator based on quantity"""
    if qty == 0:
        return '·'
    elif qty < 50:
        return '░'
    elif qty < 100:
        return '▒'
    elif qty < 150:
        return '▓'
    else:
        return '█'


def show_stock_overview():
    inventory = load_data()

    # Calculate totals
    total_items = sum(i['items'] or 0 for i in inventory)

    # Build room/floor stats
    room_floor_stats = defaultdict(lambda: defaultdict(lambda: {'items': 0, 'gatars': set()}))

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        room_floor_stats[room][floor]['items'] += item['items'] or 0
        gate_str = str(item['gate_no']).split(',')[0].strip()
        try:
            room_floor_stats[room][floor]['gatars'].add(int(gate_str))
        except:
            pass

    # Calculate total capacity (all gatars across all rooms/floors)
    total_gatars = 0
    occupied_gatars = 0
    for room in GATAR_CONFIG:
        for floor in GATAR_CONFIG[room]:
            config = GATAR_CONFIG[room][floor]
            floor_gatars = config.get('end', 0) - config.get('start', 0) + 1
            total_gatars += floor_gatars
            occupied_gatars += len(room_floor_stats[room][floor]['gatars'])

    empty_gatars = total_gatars - occupied_gatars
    occupancy_pct = (occupied_gatars / total_gatars * 100) if total_gatars else 0

    # Capacity (assuming 140,000 bags total capacity like in the web UI)
    TOTAL_CAPACITY = 140000
    capacity_pct = (total_items / TOTAL_CAPACITY * 100)

    # Print header
    print("\n" + "═" * 100)
    print("                           📦 IN STOCK OVERVIEW")
    print("                    Visual floor plan of storage occupancy")
    print("═" * 100)

    # Summary Cards
    print(f"""
    ┌────────────────────┬────────────────────┬────────────────────┬────────────────────┐
    │   📦 TOTAL BAGS    │   ▓▓ OCCUPIED      │   ·· EMPTY         │   📊 OCCUPANCY     │
    ├────────────────────┼────────────────────┼────────────────────┼────────────────────┤
    │     {total_items:>10,}     │     {occupied_gatars:>10,}     │     {empty_gatars:>10,}     │     {occupancy_pct:>9.1f}%     │
    └────────────────────┴────────────────────┴────────────────────┴────────────────────┘
    """)

    # Storage Capacity Bar
    bar_width = 60
    filled = int(capacity_pct / 100 * bar_width)
    bar = "█" * filled + "░" * (bar_width - filled)

    print(f"""
    ┌──────────────────────────────────────────────────────────────────────────────────────┐
    │  🏭 STORAGE CAPACITY                                          Total: 1,40,000 bags  │
    ├──────────────────────────────────────────────────────────────────────────────────────┤
    │                                                                                      │
    │   [{bar}]  {capacity_pct:5.1f}%                     │
    │    0              35K              70K             105K             140K             │
    │                                                                                      │
    └──────────────────────────────────────────────────────────────────────────────────────┘
    """)

    # Room Definitions
    room_info = {
        '1': {'name': 'Room 1', 'type': 'Seed', 'color': '🟢'},
        '2': {'name': 'Room 2', 'type': 'Seed', 'color': '🟢'},
        '3': {'name': 'Room 3', 'type': 'Sell', 'color': '🟠'},
        '4': {'name': 'Room 4', 'type': 'Sell', 'color': '🟠'},
        'G': {'name': 'Gallery', 'type': 'Mix', 'color': '🟣'}
    }

    # Room Summary Cards (like web UI)
    print("\n    ═══════════════════════════════════════════════════════════════════════════════════")
    print("                                    ROOM SUMMARY")
    print("    ═══════════════════════════════════════════════════════════════════════════════════\n")

    # Print rooms side by side
    rooms_order = ['1', '2', 'G', '3', '4']

    # First row: Room headers
    for room in rooms_order:
        info = room_info[room]
        room_items = sum(room_floor_stats[room][f]['items'] for f in ['0','1','2','3','4'])
        room_gatars_used = sum(len(room_floor_stats[room][f]['gatars']) for f in ['0','1','2','3','4'])
        room_total_gatars = sum(
            GATAR_CONFIG.get(room, {}).get(f, {}).get('end', 0) - GATAR_CONFIG.get(room, {}).get(f, {}).get('start', -1) + 1
            for f in ['0','1','2','3','4'] if GATAR_CONFIG.get(room, {}).get(f, {})
        )
        print(f"    ┌──────────────────┐", end="  ")
    print()

    for room in rooms_order:
        info = room_info[room]
        print(f"    │ {info['color']} {info['name']:<8} ({info['type'][:4]}) │", end="  ")
    print()

    for room in rooms_order:
        print(f"    ├──────────────────┤", end="  ")
    print()

    # Items row
    for room in rooms_order:
        room_items = sum(room_floor_stats[room][f]['items'] for f in ['0','1','2','3','4'])
        print(f"    │ {room_items:>14,} │", end="  ")
    print()

    # Gatars row
    for room in rooms_order:
        room_gatars_used = sum(len(room_floor_stats[room][f]['gatars']) for f in ['0','1','2','3','4'])
        room_total_gatars = sum(
            GATAR_CONFIG.get(room, {}).get(f, {}).get('end', 0) - GATAR_CONFIG.get(room, {}).get(f, {}).get('start', -1) + 1
            for f in ['0','1','2','3','4'] if GATAR_CONFIG.get(room, {}).get(f, {})
        )
        print(f"    │   {room_gatars_used:>4}/{room_total_gatars:<4} gatars │", end="  ")
    print()

    for room in rooms_order:
        print(f"    ├──────────────────┤", end="  ")
    print()

    # Floor rows (4, 3, 2, 1, 0 from top to bottom like web UI)
    for floor in ['4', '3', '2', '1', '0']:
        for room in rooms_order:
            config = GATAR_CONFIG.get(room, {}).get(floor, {})
            if not config:
                print(f"    │      ---         │", end="  ")
                continue

            floor_items = room_floor_stats[room][floor]['items']
            floor_gatars_used = len(room_floor_stats[room][floor]['gatars'])
            floor_total = config.get('end', 0) - config.get('start', 0) + 1
            pct = (floor_gatars_used / floor_total * 100) if floor_total else 0

            # Mini progress bar (8 chars)
            bar_filled = int(pct / 100 * 8)
            bar = "█" * bar_filled + "░" * (8 - bar_filled)

            print(f"    │F{floor} {bar} {pct:>3.0f}%│", end="  ")
        print()

    for room in rooms_order:
        print(f"    └──────────────────┘", end="  ")
    print()

    # Detailed Floor View for Each Room
    print("\n\n" + "═" * 100)
    print("                              DETAILED FLOOR VIEWS")
    print("═" * 100)

    # Build gatar grids
    gatar_data = defaultdict(lambda: {'items': 0, 'customers': set()})
    for item in inventory:
        gate_str = str(item['gate_no']).split(',')[0].strip()
        try:
            gate_no = int(gate_str)
            gatar_data[gate_no]['items'] += item['items'] or 0
            for cid in (item['customer_ids'] or []):
                gatar_data[gate_no]['customers'].add(cid)
        except:
            pass

    # Show each room/floor
    for room in ['1', '2', '3', '4', 'G']:
        info = room_info[room]
        room_items = sum(room_floor_stats[room][f]['items'] for f in ['0','1','2','3','4'])

        print(f"\n\n    {'━' * 90}")
        print(f"    {info['color']} {info['name']} ({info['type']}) - Total: {room_items:,} items")
        print(f"    {'━' * 90}")

        for floor in ['0', '1', '2', '3', '4']:
            config = GATAR_CONFIG.get(room, {}).get(floor, {})
            if not config:
                continue

            start = config.get('start', 0)
            end = config.get('end', 0)
            cols = config.get('cols', 14)
            rows = config.get('rows', 10)

            floor_items = room_floor_stats[room][floor]['items']
            floor_gatars = len(room_floor_stats[room][floor]['gatars'])
            floor_total = end - start + 1
            pct = (floor_gatars / floor_total * 100) if floor_total else 0

            access_label = ["Ground (Easy)", "Floor 1", "Floor 2", "Floor 3", "Top (Hard)"][int(floor)]

            print(f"\n      ┌─ Floor {floor} ({access_label}) ─ Gatar {start}-{end} ─ {floor_items:,} items ({floor_gatars}/{floor_total} gatars, {pct:.0f}%) ─┐")

            if room == 'G':
                # Gallery: 2 columns
                print("      │  C1  C2                                                                        │")
                for r in range(rows):
                    row_str = f"      │ R{r+1:02d}"
                    for c in range(cols):
                        gatar_no = start + (r * cols) + c
                        if gatar_no <= end:
                            qty = gatar_data[gatar_no]['items']
                            char = get_density_char(qty)
                            row_str += f"  {char} "
                    row_str += " " * (80 - len(row_str)) + "│"
                    print(row_str)
            else:
                # Regular rooms: 14 columns (or 12 for floor 4)
                header = "      │      "
                for c in range(1, cols + 1):
                    header += f"C{c:02d} "
                header += " " * (86 - len(header)) + "│"
                print(header)

                for r in range(rows):
                    row_str = f"      │ R{r+1:02d}  "
                    for c in range(cols):
                        # Calculate gatar number based on column-pair layout
                        pair_idx = c // 2
                        is_left = (c % 2 == 0)
                        base = start + (pair_idx * 20)
                        if is_left:
                            gatar_no = base + (r * 2)
                        else:
                            gatar_no = base + (r * 2) + 1

                        if gatar_no <= end:
                            qty = gatar_data[gatar_no]['items']
                            char = get_density_char(qty)
                            row_str += f" {char}  "
                        else:
                            row_str += "    "
                    row_str += "│"
                    print(row_str)

            print(f"      └{'─' * 84}┘")

    # Legend
    print(f"""

    ════════════════════════════════════════════════════════════════════════════════════════════
                                           LEGEND
    ════════════════════════════════════════════════════════════════════════════════════════════

        ·  = Empty (0 items)
        ░  = Light (1-49 items)
        ▒  = Medium (50-99 items)
        ▓  = Heavy (100-149 items)
        █  = Full (150+ items)

        🟢 = Seed Rooms (1, 2)
        🟠 = Sell Rooms (3, 4)
        🟣 = Gallery (Mixed)

    ════════════════════════════════════════════════════════════════════════════════════════════
    """)

    # Summary Statistics
    print(f"""
    ┌────────────────────────────────────────────────────────────────────────────────────────┐
    │                               QUICK STATISTICS                                          │
    ├────────────────────────────────────────────────────────────────────────────────────────┤
    │                                                                                        │
    │   Total Items in Storage:     {total_items:>10,}                                              │
    │   Total Gatars Occupied:      {occupied_gatars:>10,}                                              │
    │   Total Gatars Empty:         {empty_gatars:>10,}                                              │
    │   Overall Occupancy:          {occupancy_pct:>10.1f}%                                             │
    │   Capacity Used:              {capacity_pct:>10.1f}%                                             │
    │                                                                                        │
    │   Seed Rooms (1, 2):          {sum(room_floor_stats['1'][f]['items'] + room_floor_stats['2'][f]['items'] for f in ['0','1','2','3','4']):>10,} items                                    │
    │   Sell Rooms (3, 4):          {sum(room_floor_stats['3'][f]['items'] + room_floor_stats['4'][f]['items'] for f in ['0','1','2','3','4']):>10,} items                                    │
    │   Gallery (G):                {sum(room_floor_stats['G'][f]['items'] for f in ['0','1','2','3','4']):>10,} items                                    │
    │                                                                                        │
    └────────────────────────────────────────────────────────────────────────────────────────┘
    """)


if __name__ == "__main__":
    show_stock_overview()
