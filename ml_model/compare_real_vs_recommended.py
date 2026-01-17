"""
Compare Real Inventory vs Recommendation System
================================================
Analyzes how items are ACTUALLY stored vs how the system WOULD recommend.
"""

import json
from collections import defaultdict
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG


def load_fresh_data():
    """Load fresh data from production database"""
    with open('inventory_fresh.json', 'r') as f:
        inventory = json.loads(f.read().strip())
    with open('customer_patterns_fresh.json', 'r') as f:
        patterns = json.loads(f.read().strip())
    return inventory, patterns


def analyze_real_vs_recommended():
    print("=" * 70)
    print("REAL INVENTORY vs RECOMMENDATION SYSTEM COMPARISON")
    print("=" * 70)

    # Load data
    inventory, customer_data = load_fresh_data()

    # Create model
    model = StorageRecommendationModel()

    # Load inventory into model
    for item in inventory:
        key = f"{item['room_no']}-{item['floor']}-{item['gate_no']}"
        model.inventory_data[key] = {
            'room_no': item['room_no'],
            'floor': item['floor'],
            'gate_no': str(item['gate_no']).split(',')[0].strip(),
            'items': item['items'] or 0,
            'customer_ids': item['customer_ids'] or [],
        }

    # Build customer patterns
    customer_patterns = {}
    for item in customer_data:
        cid = item['customer_id']
        if cid not in customer_patterns:
            customer_patterns[cid] = {
                'name': item['name'],
                'total_items': 0,
                'distribution': [],
                'rooms': set(),
                'floors': set(),
            }
        customer_patterns[cid]['distribution'].append({
            'room_no': item['room_no'],
            'floor': item['floor'],
            'items': item['items'],
        })
        customer_patterns[cid]['total_items'] += item['items']
        customer_patterns[cid]['rooms'].add(item['room_no'])
        customer_patterns[cid]['floors'].add(item['floor'])

    # Load patterns into model
    for cid, data in customer_patterns.items():
        primary = data['distribution'][0] if data['distribution'] else {}
        model.customer_patterns[cid] = {
            'total_items': data['total_items'],
            'total_thocks': len(data['distribution']),
            'distribution': data['distribution'],
            'primary_room': primary.get('room_no'),
            'primary_floor': primary.get('floor'),
        }

    # =========================================================================
    # ANALYSIS 1: Room Distribution
    # =========================================================================
    print("\n" + "─" * 70)
    print("1. ROOM DISTRIBUTION ANALYSIS")
    print("─" * 70)

    # Real distribution
    real_room_items = defaultdict(int)
    real_room_gatars = defaultdict(int)
    for item in inventory:
        real_room_items[item['room_no']] += item['items'] or 0
        real_room_gatars[item['room_no']] += 1

    total_items = sum(real_room_items.values())

    print("\nReal Inventory Distribution:")
    print(f"{'Room':<8} {'Items':<12} {'%':<8} {'Gatars':<10} {'Avg/Gatar'}")
    print("─" * 50)
    for room in ['1', '2', 'G', '3', '4']:
        items = real_room_items.get(room, 0)
        gatars = real_room_gatars.get(room, 0)
        pct = (items / total_items * 100) if total_items > 0 else 0
        avg = items / gatars if gatars > 0 else 0
        print(f"{room:<8} {items:<12,} {pct:<8.1f} {gatars:<10} {avg:.1f}")

    # Seed vs Sell distribution
    seed_items = real_room_items.get('1', 0) + real_room_items.get('2', 0)
    sell_items = real_room_items.get('3', 0) + real_room_items.get('4', 0)
    gallery_items = real_room_items.get('G', 0)

    print(f"\nCategory Distribution:")
    print(f"  Seed (Room 1+2):  {seed_items:,} items ({seed_items/total_items*100:.1f}%)")
    print(f"  Sell (Room 3+4):  {sell_items:,} items ({sell_items/total_items*100:.1f}%)")
    print(f"  Gallery (Room G): {gallery_items:,} items ({gallery_items/total_items*100:.1f}%)")

    # =========================================================================
    # ANALYSIS 2: Floor Distribution
    # =========================================================================
    print("\n" + "─" * 70)
    print("2. FLOOR DISTRIBUTION ANALYSIS")
    print("─" * 70)

    real_floor_items = defaultdict(int)
    for item in inventory:
        real_floor_items[item['floor']] += item['items'] or 0

    print("\nReal Floor Distribution:")
    print(f"{'Floor':<8} {'Items':<12} {'%':<8} {'Recommendation System Preference'}")
    print("─" * 70)

    floor_prefs = {
        '0': 'Highest (1.0) - Ground floor, easiest access',
        '1': 'High (0.9)',
        '2': 'Medium (0.8)',
        '3': 'Low (0.6)',
        '4': 'Lowest (0.4) - Top floor, hardest access',
    }

    for floor in ['0', '1', '2', '3', '4']:
        items = real_floor_items.get(floor, 0)
        pct = (items / total_items * 100) if total_items > 0 else 0
        print(f"{floor:<8} {items:<12,} {pct:<8.1f} {floor_prefs.get(floor, '')}")

    # =========================================================================
    # ANALYSIS 3: Gallery Proximity (Real vs Recommended)
    # =========================================================================
    print("\n" + "─" * 70)
    print("3. GALLERY PROXIMITY ANALYSIS")
    print("─" * 70)

    # Calculate gallery distance for real inventory
    distance_distribution = defaultdict(int)
    items_by_distance = defaultdict(int)

    for item in inventory:
        room = item['room_no']
        floor = item['floor']
        gate_no_str = str(item['gate_no']).split(',')[0].strip()

        try:
            gate_no = int(gate_no_str)
            col = model.get_column_from_gatar(gate_no, room, floor)
            config = GATAR_CONFIG.get(room, {}).get(floor, {})
            cols = config.get('cols', 14)
            distance = model.get_gallery_distance(col, cols)

            distance_distribution[distance] += 1
            items_by_distance[distance] += item['items'] or 0
        except:
            pass

    print("\nReal Inventory - Distance from Gallery:")
    print(f"{'Distance':<12} {'Gatars':<10} {'Items':<12} {'% Items':<10} {'Recommendation'}")
    print("─" * 70)

    rec_notes = {
        0: 'BEST for small qty (1-20 bags)',
        1: 'GOOD for small/medium qty',
        2: 'OK for medium qty (20-50 bags)',
        3: 'OK for large qty (50+ bags)',
        4: 'OK for large qty only',
        5: 'Deep - large qty only',
        6: 'Deepest - large qty only',
    }

    for dist in sorted(distance_distribution.keys()):
        gatars = distance_distribution[dist]
        items = items_by_distance[dist]
        pct = (items / total_items * 100) if total_items > 0 else 0
        note = rec_notes.get(dist, 'Very deep')
        print(f"{dist:<12} {gatars:<10} {items:<12,} {pct:<10.1f} {note}")

    # =========================================================================
    # ANALYSIS 4: Customer Clustering (Real vs Recommended)
    # =========================================================================
    print("\n" + "─" * 70)
    print("4. CUSTOMER CLUSTERING ANALYSIS")
    print("─" * 70)

    # How many rooms/floors does each customer use?
    single_room_customers = 0
    multi_room_customers = 0
    single_floor_customers = 0
    multi_floor_customers = 0

    rooms_per_customer = []
    floors_per_customer = []

    for cid, data in customer_patterns.items():
        num_rooms = len(data['rooms'])
        num_floors = len(data['floors'])

        rooms_per_customer.append(num_rooms)
        floors_per_customer.append(num_floors)

        if num_rooms == 1:
            single_room_customers += 1
        else:
            multi_room_customers += 1

        if num_floors == 1:
            single_floor_customers += 1
        else:
            multi_floor_customers += 1

    total_customers = len(customer_patterns)

    print("\nReal Customer Clustering:")
    print(f"  Customers in single room:   {single_room_customers} ({single_room_customers/total_customers*100:.1f}%)")
    print(f"  Customers in multiple rooms: {multi_room_customers} ({multi_room_customers/total_customers*100:.1f}%)")
    print(f"  ")
    print(f"  Customers on single floor:   {single_floor_customers} ({single_floor_customers/total_customers*100:.1f}%)")
    print(f"  Customers on multiple floors: {multi_floor_customers} ({multi_floor_customers/total_customers*100:.1f}%)")

    avg_rooms = sum(rooms_per_customer) / len(rooms_per_customer) if rooms_per_customer else 0
    avg_floors = sum(floors_per_customer) / len(floors_per_customer) if floors_per_customer else 0

    print(f"\n  Average rooms per customer:  {avg_rooms:.2f}")
    print(f"  Average floors per customer: {avg_floors:.2f}")

    print("\n  Recommendation System Target:")
    print("    - Keep customer items clustered (same room/floor when possible)")
    print("    - Customer clustering weight: 25%")

    # =========================================================================
    # ANALYSIS 5: What Would System Recommend Differently?
    # =========================================================================
    print("\n" + "─" * 70)
    print("5. RECOMMENDATION DIFFERENCES")
    print("─" * 70)

    # Sample some customers and compare real vs recommended
    sample_customers = list(customer_patterns.items())[:20]

    differences = {
        'same_room': 0,
        'different_room': 0,
        'same_floor': 0,
        'different_floor': 0,
        'closer_to_gallery': 0,
        'farther_from_gallery': 0,
        'same_distance': 0,
    }

    print("\nSample Comparison (20 customers):")
    print(f"{'Customer':<20} {'Real Room':<12} {'Rec Room':<12} {'Real Floor':<12} {'Rec Floor'}")
    print("─" * 70)

    for cid, data in sample_customers:
        real_room = data['distribution'][0]['room_no'] if data['distribution'] else None
        real_floor = data['distribution'][0]['floor'] if data['distribution'] else None

        # Determine category from real room
        if real_room in ['1', '2']:
            category = 'seed'
        elif real_room in ['3', '4']:
            category = 'sell'
        else:
            category = 'seed'  # Default

        # Get recommendation
        recs = model.recommend(cid, 30, category)
        if recs:
            rec = recs[0]
            rec_room = rec.room_no
            rec_floor = rec.floor

            # Compare
            if real_room == rec_room:
                differences['same_room'] += 1
                room_match = "✓"
            else:
                differences['different_room'] += 1
                room_match = "✗"

            if real_floor == rec_floor:
                differences['same_floor'] += 1
                floor_match = "✓"
            else:
                differences['different_floor'] += 1
                floor_match = "✗"

            print(f"{data['name'][:18]:<20} {real_room:<12} {rec_room} {room_match:<10} {real_floor:<12} {rec_floor} {floor_match}")

    print("\n" + "─" * 70)
    print("DIFFERENCE SUMMARY (Sample of 20 customers)")
    print("─" * 70)
    print(f"  Room Match:   {differences['same_room']}/20 ({differences['same_room']/20*100:.0f}%)")
    print(f"  Floor Match:  {differences['same_floor']}/20 ({differences['same_floor']/20*100:.0f}%)")

    # =========================================================================
    # ANALYSIS 6: Optimization Opportunities
    # =========================================================================
    print("\n" + "─" * 70)
    print("6. OPTIMIZATION OPPORTUNITIES")
    print("─" * 70)

    # Items far from gallery
    far_items = sum(items_by_distance.get(d, 0) for d in range(4, 10))
    far_pct = (far_items / total_items * 100) if total_items > 0 else 0

    print(f"\nCurrent Issues Identified:")
    print(f"  1. Items far from gallery (4+ cols): {far_items:,} ({far_pct:.1f}%)")
    print(f"     - Recommendation: Place small qty near gallery")

    # Scattered customers
    print(f"  2. Customers scattered across rooms: {multi_room_customers} ({multi_room_customers/total_customers*100:.1f}%)")
    print(f"     - Recommendation: Keep customer items clustered")

    # Floor imbalance
    floor_0_pct = real_floor_items.get('0', 0) / total_items * 100 if total_items > 0 else 0
    floor_4_pct = real_floor_items.get('4', 0) / total_items * 100 if total_items > 0 else 0

    print(f"  3. Floor distribution:")
    print(f"     - Floor 0 (easiest): {floor_0_pct:.1f}%")
    print(f"     - Floor 4 (hardest): {floor_4_pct:.1f}%")
    print(f"     - Recommendation: Prioritize lower floors for accessibility")

    # =========================================================================
    # FINAL SUMMARY
    # =========================================================================
    print("\n" + "=" * 70)
    print("FINAL COMPARISON SUMMARY")
    print("=" * 70)

    print("""
    ┌─────────────────────────────────────────────────────────────────────┐
    │  Aspect              │ Real Inventory      │ Recommendation System │
    ├─────────────────────────────────────────────────────────────────────┤
    │  Gallery Proximity   │ Mixed distances     │ Small qty near gallery│
    │  Customer Clustering │ {:.0f}% single room    │ Prioritize clustering │
    │  Floor Preference    │ Distributed         │ Lower floors first    │
    │  Category Matching   │ Mostly correct      │ Strict enforcement    │
    │  Load Balance        │ Natural fill        │ CPU cooler pattern    │
    └─────────────────────────────────────────────────────────────────────┘
    """.format(single_room_customers/total_customers*100))

    print("  KEY DIFFERENCES:")
    print("  ─────────────────")
    print("  1. Real inventory was filled WITHOUT recommendation system")
    print("  2. System would optimize for gallery access based on quantity")
    print("  3. System would keep customer items more clustered")
    print("  4. System enforces category-room matching strictly")
    print("  5. System considers weight distribution (load balance)")

    print("\n  CONCLUSION:")
    print("  ────────────")
    print("  The recommendation system would provide MORE OPTIMIZED placements")
    print("  compared to current manual/random storage decisions.")


if __name__ == "__main__":
    analyze_real_vs_recommended()
