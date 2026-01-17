"""
Local Test for Storage Recommendation Model
Uses exported JSON data from production database
"""

import json
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG

def load_exported_data():
    """Load exported JSON data"""
    # Load inventory
    with open('inventory_sample.json', 'r') as f:
        content = f.read().strip()
        if content:
            inventory = json.loads(content)
        else:
            inventory = []

    # Load customer patterns
    with open('customer_patterns.json', 'r') as f:
        content = f.read().strip()
        if content:
            patterns = json.loads(content)
        else:
            patterns = []

    return inventory, patterns


def test_with_exported_data():
    """Test recommendation model with exported production data"""
    print("=" * 70)
    print("Storage Recommendation Model - Local Test with Production Data")
    print("=" * 70)

    # Load data
    inventory, customer_data = load_exported_data()
    print(f"\nLoaded {len(inventory)} inventory records")
    print(f"Loaded {len(customer_data)} customer pattern records")

    # Create model
    model = StorageRecommendationModel()

    # Convert inventory to model format
    for item in inventory:
        key = f"{item['room_no']}-{item['floor']}-{item['gate_no']}"
        model.inventory_data[key] = {
            'room_no': item['room_no'],
            'floor': item['floor'],
            'gate_no': item['gate_no'],
            'items': item['items'] or 0,
            'customer_ids': item['customer_ids'] or [],
            'thock_numbers': []
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
                'primary_room': None,
                'primary_floor': None,
            }

        customer_patterns[cid]['distribution'].append({
            'room_no': item['room_no'],
            'floor': item['floor'],
            'items': item['items'],
        })
        customer_patterns[cid]['total_items'] += item['items']

        if customer_patterns[cid]['primary_room'] is None:
            customer_patterns[cid]['primary_room'] = item['room_no']
            customer_patterns[cid]['primary_floor'] = item['floor']

    # Print summary
    print("\n--- Inventory Summary ---")
    room_summary = {}
    for key, data in model.inventory_data.items():
        room = data['room_no']
        if room not in room_summary:
            room_summary[room] = {'gatars': 0, 'items': 0}
        room_summary[room]['gatars'] += 1
        room_summary[room]['items'] += data['items']

    for room in sorted(room_summary.keys()):
        print(f"  Room {room}: {room_summary[room]['gatars']} gatars, {room_summary[room]['items']} items")

    # Find top customers
    print("\n--- Top Customers by Items ---")
    sorted_customers = sorted(customer_patterns.items(), key=lambda x: x[1]['total_items'], reverse=True)[:5]
    for cid, data in sorted_customers:
        print(f"  {data['name']} (ID: {cid}): {data['total_items']} items")
        print(f"    Primary: Room {data['primary_room']}, Floor {data['primary_floor']}")

    # Test recommendations with top customer
    if sorted_customers:
        test_cid = sorted_customers[0][0]
        test_name = sorted_customers[0][1]['name']
        model.customer_patterns[test_cid] = {
            'total_items': customer_patterns[test_cid]['total_items'],
            'total_thocks': len(customer_patterns[test_cid]['distribution']),
            'distribution': customer_patterns[test_cid]['distribution'],
            'primary_room': customer_patterns[test_cid]['primary_room'],
            'primary_floor': customer_patterns[test_cid]['primary_floor'],
        }

        print(f"\n{'=' * 70}")
        print(f"TESTING RECOMMENDATIONS FOR: {test_name} (ID: {test_cid})")
        print(f"{'=' * 70}")

        # Test with different quantities
        test_cases = [
            (10, 'seed', 'SMALL quantity - should be NEAR gallery'),
            (30, 'seed', 'MEDIUM quantity - can be 1-2 cols from gallery'),
            (80, 'seed', 'LARGE quantity - can go DEEPER'),
        ]

        for qty, category, desc in test_cases:
            print(f"\n{'─' * 70}")
            print(f"TEST: {qty} bags ({category}) - {desc}")
            print(f"{'─' * 70}")

            recommendations = model.recommend(test_cid, qty, category)

            for i, rec in enumerate(recommendations[:3], 1):
                print(f"\n  #{i} Room {rec.room_no}, Floor {rec.floor}, Gatar {rec.gatar_no}")
                print(f"      Score: {rec.score}/100")
                print(f"      Gallery Distance: {rec.distance_from_gallery} columns")
                print(f"      Reasons: {', '.join(rec.reasons)}")
                print(f"      Score Breakdown:")
                for k, v in rec.score_breakdown.items():
                    print(f"        - {k}: {v}")

        # Validate recommendations
        print(f"\n{'=' * 70}")
        print("VALIDATION")
        print(f"{'=' * 70}")

        # Check if small quantities are near gallery
        small_recs = model.recommend(test_cid, 10, 'seed')
        if small_recs and small_recs[0].distance_from_gallery <= 1:
            print("[PASS] Small quantity (10 bags) placed near gallery")
        else:
            print(f"[WARN] Small quantity not near gallery - distance: {small_recs[0].distance_from_gallery if small_recs else 'N/A'}")

        # Check if large quantities can go deeper
        large_recs = model.recommend(test_cid, 100, 'seed')
        if large_recs:
            print(f"[INFO] Large quantity (100 bags) suggested at distance: {large_recs[0].distance_from_gallery}")

        # Check customer clustering
        if small_recs and small_recs[0].room_no == customer_patterns[test_cid]['primary_room']:
            print("[PASS] Recommendation keeps customer items clustered")
        else:
            print("[INFO] Recommendation suggests different room (may be due to capacity)")


if __name__ == "__main__":
    test_with_exported_data()
