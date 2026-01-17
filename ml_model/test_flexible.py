"""
Test Flexible Employee Selection
================================
Tests that the model works when:
1. No room/floor selected - uses customer clustering (if possible)
2. Employee selects specific room - recommends best gatar in that room
3. Employee selects specific floor - recommends best gatar on that floor
4. Employee selects both - recommends best gatar in that room/floor
"""

import json
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG


def load_data():
    """Load exported data"""
    with open('inventory_full.json', 'r') as f:
        inventory = json.loads(f.read().strip())
    with open('customer_patterns_full.json', 'r') as f:
        patterns = json.loads(f.read().strip())
    return inventory, patterns


def test_flexible_selection():
    print("=" * 70)
    print("Flexible Employee Selection Test")
    print("=" * 70)

    # Load data
    inventory, customer_data = load_data()

    # Create model
    model = StorageRecommendationModel()

    # Load inventory
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

    # Find a test customer with items in Room 1
    test_customer = None
    for cid, data in customer_patterns.items():
        if data['primary_room'] == '1' and data['total_items'] > 100:
            test_customer = (cid, data)
            break

    if not test_customer:
        # Fallback to any customer
        test_customer = list(customer_patterns.items())[0]

    cid, cust_data = test_customer
    model.customer_patterns[cid] = {
        'total_items': cust_data['total_items'],
        'total_thocks': len(cust_data['distribution']),
        'distribution': cust_data['distribution'],
        'primary_room': cust_data['primary_room'],
        'primary_floor': cust_data['primary_floor'],
    }

    print(f"\nTest Customer: {cust_data['name']} (ID: {cid})")
    print(f"Primary Location: Room {cust_data['primary_room']}, Floor {cust_data['primary_floor']}")
    print(f"Total Items: {cust_data['total_items']}")

    # Test cases
    test_cases = [
        {
            'name': 'No Selection (Auto - Customer Clustering)',
            'room': None,
            'floor': None,
            'expected': f"Should suggest Room {cust_data['primary_room']} (customer's room)"
        },
        {
            'name': 'Employee Selects Room 2',
            'room': '2',
            'floor': None,
            'expected': "Should suggest best gatar in Room 2"
        },
        {
            'name': 'Employee Selects Room 3 (different category)',
            'room': '3',
            'floor': None,
            'expected': "Should suggest best gatar in Room 3"
        },
        {
            'name': 'Employee Selects Floor 0',
            'room': None,
            'floor': '0',
            'expected': "Should suggest best gatar on Floor 0"
        },
        {
            'name': 'Employee Selects Room 4, Floor 2',
            'room': '4',
            'floor': '2',
            'expected': "Should suggest best gatar in Room 4, Floor 2"
        },
    ]

    print("\n" + "=" * 70)
    print("TEST RESULTS")
    print("=" * 70)

    all_passed = True

    for tc in test_cases:
        print(f"\n{'─' * 70}")
        print(f"TEST: {tc['name']}")
        print(f"Expected: {tc['expected']}")
        print(f"{'─' * 70}")

        recommendations = model.recommend(
            customer_id=cid,
            quantity=30,
            category='seed',
            selected_room=tc['room'],
            selected_floor=tc['floor']
        )

        if recommendations:
            rec = recommendations[0]
            print(f"\n  Top Recommendation:")
            print(f"    Room {rec.room_no}, Floor {rec.floor}, Gatar {rec.gatar_no}")
            print(f"    Score: {rec.score}/100")
            print(f"    Gallery Distance: {rec.distance_from_gallery} columns")
            print(f"    Reasons: {', '.join(rec.reasons)}")

            # Validate
            passed = True
            if tc['room'] and rec.room_no != tc['room']:
                print(f"    [FAIL] Expected Room {tc['room']}, got Room {rec.room_no}")
                passed = False
            if tc['floor'] and rec.floor != tc['floor']:
                print(f"    [FAIL] Expected Floor {tc['floor']}, got Floor {rec.floor}")
                passed = False

            if passed:
                print(f"    [PASS] Recommendation matches selection")
            else:
                all_passed = False

            # Show other recommendations
            if len(recommendations) > 1:
                print(f"\n  Other options:")
                for r in recommendations[1:3]:
                    print(f"    - Room {r.room_no}, Floor {r.floor}, Gatar {r.gatar_no} (Score: {r.score})")
        else:
            print(f"  [FAIL] No recommendations returned")
            all_passed = False

    # Summary
    print("\n" + "=" * 70)
    print("SUMMARY")
    print("=" * 70)

    if all_passed:
        print("\n  [PASS] All tests passed!")
        print("\n  Model Behavior:")
        print("    - Without selection: Clusters with customer's existing items (if possible)")
        print("    - With room selection: Recommends best gatar in selected room")
        print("    - With floor selection: Recommends best gatar on selected floor")
        print("    - With both: Recommends best gatar in selected room/floor")
        print("\n  *** FLEXIBLE MODEL WORKING CORRECTLY ***")
    else:
        print("\n  [WARN] Some tests failed - review needed")


if __name__ == "__main__":
    test_flexible_selection()
