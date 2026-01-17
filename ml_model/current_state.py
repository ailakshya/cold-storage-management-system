"""
CURRENT STATE - How Items Are Actually Stored Right Now
=======================================================
Based on fresh data from production database.
"""

import json
from collections import defaultdict
from storage_recommendation import GATAR_CONFIG


def load_data():
    with open('inventory_fresh.json', 'r') as f:
        return json.loads(f.read().strip())

def load_customers():
    with open('customer_patterns_fresh.json', 'r') as f:
        return json.loads(f.read().strip())


def get_column_row(gatar_no, room_no, floor):
    config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
    if not config:
        return None, None

    cols = config.get('cols', 14)
    start = config.get('start', 1)

    pos = gatar_no - start
    if pos < 0:
        return None, None

    if cols == 2:
        col = (pos % 2) + 1
        row = (pos // 2) + 1
    else:
        col = (pos % cols) + 1
        row = (pos // cols) + 1

    return col, row


def get_density_char(items):
    if items == 0:
        return '  '
    elif items < 30:
        return '░░'
    elif items < 60:
        return '▒▒'
    elif items < 100:
        return '▓▓'
    elif items < 150:
        return '██'
    else:
        return '▓█'


def show_current_state():
    print("=" * 80)
    print("           CURRENT STATE - REAL INVENTORY FROM DATABASE")
    print("=" * 80)

    inventory = load_data()
    customers = load_customers()

    # Build grids
    grids = defaultdict(lambda: defaultdict(lambda: {'items': 0, 'customers': set()}))

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        gate_str = str(item['gate_no']).split(',')[0].strip()

        try:
            gate_no = int(gate_str)
            col, row = get_column_row(gate_no, room, floor)
            if col and row:
                key = (room, floor)
                grids[key][(col, row)]['items'] += item['items'] or 0
                for cid in (item['customer_ids'] or []):
                    grids[key][(col, row)]['customers'].add(cid)
        except:
            pass

    # Summary stats
    total_items = sum(item['items'] or 0 for item in inventory)
    total_gatars = len(inventory)

    # Customer stats
    customer_rooms = defaultdict(set)
    customer_floors = defaultdict(set)
    customer_items = defaultdict(int)

    for item in customers:
        cid = item['customer_id']
        customer_rooms[cid].add(item['room_no'])
        customer_floors[cid].add(item['floor'])
        customer_items[cid] += item['items']

    print(f"""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                    CURRENT INVENTORY STATUS                                  ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   Total Items:     {total_items:>10,}                                          ║
    ║   Total Gatars:    {total_gatars:>10,}                                          ║
    ║   Total Customers: {len(customer_items):>10,}                                          ║
    ║   Avg Items/Gatar: {total_items//total_gatars if total_gatars else 0:>10}                                          ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # Room summary
    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                         ROOM SUMMARY                                         ║
    ╠══════════════════════════════════════════════════════════════════════════════╣""")

    room_stats = defaultdict(lambda: {'items': 0, 'gatars': 0})
    for item in inventory:
        room_stats[item['room_no']]['items'] += item['items'] or 0
        room_stats[item['room_no']]['gatars'] += 1

    for room in ['1', '2', 'G', '3', '4']:
        stats = room_stats[room]
        category = "SEED" if room in ['1', '2'] else ("GALLERY" if room == 'G' else "SELL")
        bar_len = int(stats['items'] / 1000)
        bar = '█' * min(bar_len, 30)
        print(f"    ║   Room {room} ({category:7}): {stats['items']:>6,} items, {stats['gatars']:>4} gatars  {bar}")

    print("""    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # Floor summary
    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                         FLOOR SUMMARY                                        ║
    ╠══════════════════════════════════════════════════════════════════════════════╣""")

    floor_stats = defaultdict(int)
    for item in inventory:
        floor_stats[item['floor']] += item['items'] or 0

    for floor in ['0', '1', '2', '3', '4']:
        items = floor_stats[floor]
        access = ["GROUND (Easiest)", "EASY", "MEDIUM", "HARD", "TOP (Hardest)"][int(floor)]
        bar_len = int(items / 1000)
        bar = '█' * min(bar_len, 25)
        print(f"    ║   Floor {floor} ({access:15}): {items:>6,} items  {bar}")

    print("""    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # Print each room/floor grid
    print("\n" + "=" * 80)
    print("              DETAILED ROOM GRIDS - ALL FLOORS")
    print("=" * 80)

    for floor in ['0', '1', '2', '3', '4']:
        print(f"\n{'═' * 80}")
        print(f"                              FLOOR {floor}")
        if floor == '0':
            print("                         (Ground - Easiest Access)")
        elif floor == '4':
            print("                         (Top - Hardest Access)")
        print(f"{'═' * 80}")

        # Print all rooms side by side for this floor
        print("""
        ┌─────────────────────┐     ┌─────────────────────┐
        │      ROOM 1         │     │      ROOM 2         │
        │      (SEED)         │     │      (SEED)         │
        └─────────────────────┘     └─────────────────────┘""")

        # Room 1 and 2 grids
        for room_pair in [('1', '2'), ('3', '4')]:
            if room_pair == ('3', '4'):
                print("""
        ┌─────────────────────┐     ┌─────────────────────┐
        │      ROOM 3         │     │      ROOM 4         │
        │      (SELL)         │     │      (SELL)         │
        └─────────────────────┘     └─────────────────────┘""")

            room1, room2 = room_pair
            grid1 = grids.get((room1, floor), {})
            grid2 = grids.get((room2, floor), {})

            # Calculate room totals
            total1 = sum(g['items'] for g in grid1.values())
            total2 = sum(g['items'] for g in grid2.values())

            print(f"        Room {room1}: {total1:,} items              Room {room2}: {total2:,} items")
            print()

            # Column headers
            header1 = "        Col:"
            header2 = "     Col:"
            for c in range(1, 15):
                header1 += f"{c:>3}"
                header2 += f"{c:>3}"
            print(header1 + header2)

            # Rows
            for r in range(1, 11):
                row_str1 = f"        R{r:>2}:"
                row_str2 = f"     R{r:>2}:"

                for c in range(1, 15):
                    items1 = grid1.get((c, r), {}).get('items', 0)
                    char1 = get_density_char(items1)
                    row_str1 += f" {char1[0]}"

                for c in range(1, 15):
                    items2 = grid2.get((c, r), {}).get('items', 0)
                    char2 = get_density_char(items2)
                    row_str2 += f" {char2[0]}"

                print(row_str1 + row_str2)

            print()

        # Gallery for this floor
        grid_g = grids.get(('G', floor), {})
        total_g = sum(g['items'] for g in grid_g.values())
        print(f"""
        ┌───────────────────────────────────────────────────┐
        │              GALLERY (Room G) - {total_g:,} items           │
        └───────────────────────────────────────────────────┘""")

        print("        Col:  1   2")
        for r in range(1, 16):
            items = grid_g.get((1, r), {}).get('items', 0)
            items2 = grid_g.get((2, r), {}).get('items', 0)
            char1 = get_density_char(items)
            char2 = get_density_char(items2)
            print(f"        R{r:>2}: {char1} {char2}")

    # Legend
    print(f"""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                              LEGEND                                          ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   ·· = Empty (0 items)                                                      ║
    ║   ░░ = Light (<30 items)                                                    ║
    ║   ▒▒ = Medium (30-60 items)                                                 ║
    ║   ▓▓ = Heavy (60-100 items)                                                 ║
    ║   ██ = Very Heavy (100-150 items)                                           ║
    ║   ▓█ = Maximum (>150 items)                                                 ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # Customer distribution stats
    single_room = sum(1 for cid in customer_rooms if len(customer_rooms[cid]) == 1)
    multi_room = sum(1 for cid in customer_rooms if len(customer_rooms[cid]) > 1)
    single_floor = sum(1 for cid in customer_floors if len(customer_floors[cid]) == 1)
    multi_floor = sum(1 for cid in customer_floors if len(customer_floors[cid]) > 1)

    print(f"""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                     CUSTOMER DISTRIBUTION STATS                              ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   Customers in SINGLE room:    {single_room:>4} ({single_room*100//len(customer_rooms):>2}%)                               ║
    ║   Customers in MULTIPLE rooms: {multi_room:>4} ({multi_room*100//len(customer_rooms):>2}%)  ← Scattered                  ║
    ║                                                                              ║
    ║   Customers on SINGLE floor:    {single_floor:>4} ({single_floor*100//len(customer_floors):>2}%)                               ║
    ║   Customers on MULTIPLE floors: {multi_floor:>4} ({multi_floor*100//len(customer_floors):>2}%)  ← Scattered                  ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    # Top 10 customers by items
    sorted_customers = sorted(customer_items.items(), key=lambda x: -x[1])[:10]

    # Get customer names
    customer_names = {}
    for item in customers:
        customer_names[item['customer_id']] = item['name']

    print(f"""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                     TOP 10 CUSTOMERS BY ITEMS                                ║
    ╠══════════════════════════════════════════════════════════════════════════════╣""")

    for i, (cid, items) in enumerate(sorted_customers, 1):
        name = customer_names.get(cid, f"Customer {cid}")[:20]
        rooms = len(customer_rooms[cid])
        floors = len(customer_floors[cid])
        bar_len = int(items / 100)
        bar = '█' * min(bar_len, 20)
        print(f"    ║   {i:>2}. {name:<20} {items:>5} items  R:{rooms} F:{floors}  {bar}")

    print("""    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)

    print("""
    ╔══════════════════════════════════════════════════════════════════════════════╗
    ║                         CURRENT STATE SUMMARY                                ║
    ╠══════════════════════════════════════════════════════════════════════════════╣
    ║                                                                              ║
    ║   This is how items are CURRENTLY stored in the facility.                   ║
    ║                                                                              ║
    ║   Waiting for REAL gallery and room entry locations to:                     ║
    ║   • Update column numbering                                                  ║
    ║   • Fix gallery position (currently assumed center)                         ║
    ║   • Adjust access path calculations                                          ║
    ║                                                                              ║
    ╚══════════════════════════════════════════════════════════════════════════════╝
    """)


if __name__ == "__main__":
    show_current_state()
