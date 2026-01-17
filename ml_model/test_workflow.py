"""
End-to-End Workflow Test for Storage Recommendation System
==========================================================
Simulates the complete application workflow:
1. New entry arrives (thock created)
2. Employee opens room config page
3. System loads customer pattern
4. System generates recommendations
5. Employee selects room/floor (optional)
6. System provides best gatar within selection
7. Items are assigned to recommended location
8. Validate placement follows all business rules

This test simulates the room-config-1.html flow.
"""

import json
import time
import random
from dataclasses import dataclass
from typing import Optional, List, Dict
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG, WEIGHTS


@dataclass
class ThockEntry:
    """Simulates an entry from the entries table"""
    id: int
    thock_number: str
    customer_id: int
    customer_name: str
    expected_quantity: int
    thock_category: str  # 'seed' or 'sell'
    status: str


@dataclass
class WorkflowResult:
    """Result of a workflow test"""
    success: bool
    thock: ThockEntry
    recommendation: Optional[dict]
    employee_override: Optional[dict]
    final_placement: Optional[dict]
    validation_errors: List[str]
    time_ms: float


class WorkflowSimulator:
    """Simulates the complete application workflow"""

    def __init__(self):
        self.model = StorageRecommendationModel()
        self.inventory = {}
        self.customer_patterns = {}
        self.placements = []  # Track all placements made

    def load_data(self):
        """Load production data"""
        with open('inventory_full.json', 'r') as f:
            inventory = json.loads(f.read().strip())
        with open('customer_patterns_full.json', 'r') as f:
            patterns = json.loads(f.read().strip())

        # Load inventory into model
        for item in inventory:
            key = f"{item['room_no']}-{item['floor']}-{item['gate_no']}"
            self.model.inventory_data[key] = {
                'room_no': item['room_no'],
                'floor': item['floor'],
                'gate_no': str(item['gate_no']).split(',')[0].strip(),
                'items': item['items'] or 0,
                'customer_ids': item['customer_ids'] or [],
            }

        # Build customer patterns
        for item in patterns:
            cid = item['customer_id']
            if cid not in self.customer_patterns:
                self.customer_patterns[cid] = {
                    'name': item['name'],
                    'total_items': 0,
                    'distribution': [],
                    'primary_room': None,
                    'primary_floor': None,
                }
            self.customer_patterns[cid]['distribution'].append({
                'room_no': item['room_no'],
                'floor': item['floor'],
                'items': item['items'],
            })
            self.customer_patterns[cid]['total_items'] += item['items']
            if self.customer_patterns[cid]['primary_room'] is None:
                self.customer_patterns[cid]['primary_room'] = item['room_no']
                self.customer_patterns[cid]['primary_floor'] = item['floor']

        # Load patterns into model
        for cid, data in self.customer_patterns.items():
            self.model.customer_patterns[cid] = {
                'total_items': data['total_items'],
                'total_thocks': len(data['distribution']),
                'distribution': data['distribution'],
                'primary_room': data['primary_room'],
                'primary_floor': data['primary_floor'],
            }

        print(f"Loaded {len(self.model.inventory_data)} gatar records")
        print(f"Loaded {len(self.customer_patterns)} customer patterns")

    def simulate_new_entry(self, customer_id: int, quantity: int, category: str) -> ThockEntry:
        """
        Step 1: Simulate creation of a new thock entry
        (This happens when employee creates entry from entry page)
        """
        thock_prefix = '2025-' if category == 'seed' else '2025-S-'
        thock_number = f"{thock_prefix}{random.randint(1000, 9999)}"

        customer = self.customer_patterns.get(customer_id, {})
        return ThockEntry(
            id=random.randint(10000, 99999),
            thock_number=thock_number,
            customer_id=customer_id,
            customer_name=customer.get('name', f'Customer #{customer_id}'),
            expected_quantity=quantity,
            thock_category=category,
            status='active'
        )

    def simulate_room_config_page(self, thock: ThockEntry,
                                   selected_room: Optional[str] = None,
                                   selected_floor: Optional[str] = None) -> WorkflowResult:
        """
        Steps 2-7: Simulate the room-config-1.html workflow
        """
        start_time = time.perf_counter()
        validation_errors = []

        # Step 2: Employee opens room config page
        # Step 3: System loads customer pattern (already loaded in model)

        # Step 4: System generates recommendations
        recommendations = self.model.recommend(
            customer_id=thock.customer_id,
            quantity=thock.expected_quantity,
            category=thock.thock_category,
            selected_room=selected_room,
            selected_floor=selected_floor
        )

        if not recommendations:
            return WorkflowResult(
                success=False,
                thock=thock,
                recommendation=None,
                employee_override={'room': selected_room, 'floor': selected_floor} if selected_room or selected_floor else None,
                final_placement=None,
                validation_errors=['No recommendations returned'],
                time_ms=(time.perf_counter() - start_time) * 1000
            )

        # Step 5 & 6: Employee selection (if any) already passed to recommend()
        top_rec = recommendations[0]

        # Step 7: Create the placement record
        final_placement = {
            'room_no': top_rec.room_no,
            'floor': top_rec.floor,
            'gatar_no': top_rec.gatar_no,
            'quantity': thock.expected_quantity,
            'distance_from_gallery': top_rec.distance_from_gallery,
            'score': top_rec.score,
            'reasons': top_rec.reasons,
        }

        # Step 8: Validate placement
        validation_errors = self.validate_placement(thock, final_placement, selected_room, selected_floor)

        elapsed = (time.perf_counter() - start_time) * 1000

        return WorkflowResult(
            success=len(validation_errors) == 0,
            thock=thock,
            recommendation={
                'room_no': top_rec.room_no,
                'floor': top_rec.floor,
                'gatar_no': top_rec.gatar_no,
                'score': top_rec.score,
                'distance_from_gallery': top_rec.distance_from_gallery,
            },
            employee_override={'room': selected_room, 'floor': selected_floor} if selected_room or selected_floor else None,
            final_placement=final_placement,
            validation_errors=validation_errors,
            time_ms=elapsed
        )

    def validate_placement(self, thock: ThockEntry, placement: dict,
                          selected_room: Optional[str], selected_floor: Optional[str]) -> List[str]:
        """Validate that placement follows all business rules"""
        errors = []

        # Rule 1: Category must match room (Gallery accepts both)
        room = placement['room_no']
        if thock.thock_category == 'seed':
            if room not in ['1', '2', 'G']:
                errors.append(f"Seed item placed in sell room {room}")
        else:  # sell
            if room not in ['3', '4', 'G']:
                errors.append(f"Sell item placed in seed room {room}")

        # Rule 2: If employee selected room, placement must be in that room
        if selected_room and placement['room_no'] != selected_room:
            errors.append(f"Employee selected room {selected_room}, but placed in {placement['room_no']}")

        # Rule 3: If employee selected floor, placement must be on that floor
        if selected_floor and placement['floor'] != selected_floor:
            errors.append(f"Employee selected floor {selected_floor}, but placed on {placement['floor']}")

        # Rule 4: Small quantities should be near gallery (warning, not error)
        if thock.expected_quantity <= 20 and placement['distance_from_gallery'] > 2:
            # This is a warning, not a hard error
            pass

        # Rule 5: Floor must be valid
        if placement['floor'] not in ['0', '1', '2', '3', '4']:
            errors.append(f"Invalid floor: {placement['floor']}")

        # Rule 6: Gatar must be valid for the room (using global gatar numbering)
        try:
            gatar_num = int(placement['gatar_no'])
            room_config = GATAR_CONFIG.get(placement['room_no'], {})
            floor_config = room_config.get(placement['floor'], {})

            if floor_config:
                # Use global gatar numbering (start, end) from GATAR_CONFIG
                valid_start = floor_config.get('start', 1)
                valid_end = floor_config.get('end', 140)
                if not (valid_start <= gatar_num <= valid_end):
                    errors.append(f"Invalid gatar {gatar_num} for room {room}, floor {placement['floor']} (valid: {valid_start}-{valid_end})")
            else:
                errors.append(f"No config found for room {room}, floor {placement['floor']}")
        except ValueError:
            errors.append(f"Invalid gatar number: {placement['gatar_no']}")

        return errors


def run_workflow_tests():
    """Run comprehensive workflow tests"""
    print("=" * 70)
    print("END-TO-END WORKFLOW TEST")
    print("Storage Recommendation System - Full Application Flow")
    print("=" * 70)

    # Initialize simulator
    sim = WorkflowSimulator()
    sim.load_data()

    # Test scenarios that mirror real usage
    test_scenarios = [
        # Scenario 1: New customer, small quantity, no override
        {
            'name': 'New Customer - Small Qty - Auto Placement',
            'customer_id': list(sim.customer_patterns.keys())[0],
            'quantity': 15,
            'category': 'seed',
            'selected_room': None,
            'selected_floor': None,
        },
        # Scenario 2: Existing customer, medium quantity, no override (should cluster)
        {
            'name': 'Existing Customer - Medium Qty - Should Cluster',
            'customer_id': list(sim.customer_patterns.keys())[5],
            'quantity': 40,
            'category': 'seed',
            'selected_room': None,
            'selected_floor': None,
        },
        # Scenario 3: Large quantity, no override
        {
            'name': 'Large Qty - Can Go Deeper',
            'customer_id': list(sim.customer_patterns.keys())[10],
            'quantity': 100,
            'category': 'seed',
            'selected_room': None,
            'selected_floor': None,
        },
        # Scenario 4: Employee selects specific room
        {
            'name': 'Employee Override - Room 2 Only',
            'customer_id': list(sim.customer_patterns.keys())[15],
            'quantity': 30,
            'category': 'seed',
            'selected_room': '2',
            'selected_floor': None,
        },
        # Scenario 5: Employee selects specific floor
        {
            'name': 'Employee Override - Floor 1 Only',
            'customer_id': list(sim.customer_patterns.keys())[20],
            'quantity': 25,
            'category': 'seed',
            'selected_room': None,
            'selected_floor': '1',
        },
        # Scenario 6: Employee selects both room and floor
        {
            'name': 'Employee Override - Room 1, Floor 2',
            'customer_id': list(sim.customer_patterns.keys())[25],
            'quantity': 50,
            'category': 'seed',
            'selected_room': '1',
            'selected_floor': '2',
        },
        # Scenario 7: Sell category (different rooms)
        {
            'name': 'Sell Category - Auto Placement',
            'customer_id': list(sim.customer_patterns.keys())[30],
            'quantity': 35,
            'category': 'sell',
            'selected_room': None,
            'selected_floor': None,
        },
        # Scenario 8: Sell with employee override
        {
            'name': 'Sell Category - Room 4 Override',
            'customer_id': list(sim.customer_patterns.keys())[35],
            'quantity': 45,
            'category': 'sell',
            'selected_room': '4',
            'selected_floor': None,
        },
        # Scenario 9: Gallery placement (sell)
        {
            'name': 'Gallery Room - Sell Category',
            'customer_id': list(sim.customer_patterns.keys())[40],
            'quantity': 20,
            'category': 'sell',
            'selected_room': 'G',
            'selected_floor': None,
        },
        # Scenario 10: Very small quantity (should be near gallery)
        {
            'name': 'Very Small Qty - Near Gallery',
            'customer_id': list(sim.customer_patterns.keys())[45],
            'quantity': 5,
            'category': 'seed',
            'selected_room': None,
            'selected_floor': None,
        },
    ]

    # Run all tests
    results = []
    print("\n" + "─" * 70)
    print("RUNNING WORKFLOW SCENARIOS")
    print("─" * 70)

    for i, scenario in enumerate(test_scenarios, 1):
        print(f"\n[{i}/{len(test_scenarios)}] {scenario['name']}")

        # Step 1: Create thock entry
        thock = sim.simulate_new_entry(
            customer_id=scenario['customer_id'],
            quantity=scenario['quantity'],
            category=scenario['category']
        )
        print(f"    Created: {thock.thock_number} for {thock.customer_name}")
        print(f"    Quantity: {thock.expected_quantity}, Category: {thock.thock_category}")

        # Steps 2-7: Room config workflow
        result = sim.simulate_room_config_page(
            thock=thock,
            selected_room=scenario['selected_room'],
            selected_floor=scenario['selected_floor']
        )
        results.append(result)

        # Display result
        if result.success:
            print(f"    [PASS] Placed at Room {result.final_placement['room_no']}, "
                  f"Floor {result.final_placement['floor']}, "
                  f"Gatar {result.final_placement['gatar_no']}")
            print(f"    Score: {result.recommendation['score']}/100, "
                  f"Gallery Distance: {result.final_placement['distance_from_gallery']} cols")
            print(f"    Time: {result.time_ms:.2f} ms")
        else:
            print(f"    [FAIL] Validation errors: {', '.join(result.validation_errors)}")

    # Summary
    print("\n" + "=" * 70)
    print("WORKFLOW TEST SUMMARY")
    print("=" * 70)

    passed = sum(1 for r in results if r.success)
    failed = len(results) - passed
    avg_time = sum(r.time_ms for r in results) / len(results)

    print(f"\n  Total Tests: {len(results)}")
    print(f"  Passed: {passed}")
    print(f"  Failed: {failed}")
    print(f"  Success Rate: {(passed/len(results))*100:.1f}%")
    print(f"  Average Response Time: {avg_time:.2f} ms")

    # Validate business rules
    print("\n" + "─" * 70)
    print("BUSINESS RULE VALIDATION")
    print("─" * 70)

    # Check small quantities near gallery
    small_qty_results = [r for r in results if r.thock.expected_quantity <= 20]
    if small_qty_results:
        near_gallery = sum(1 for r in small_qty_results
                         if r.final_placement and r.final_placement['distance_from_gallery'] <= 1)
        pct = (near_gallery / len(small_qty_results)) * 100
        status = "[PASS]" if pct >= 70 else "[WARN]"
        print(f"  {status} Small quantities near gallery: {near_gallery}/{len(small_qty_results)} ({pct:.0f}%)")

    # Check employee overrides respected
    override_results = [r for r in results if r.employee_override and (r.employee_override.get('room') or r.employee_override.get('floor'))]
    if override_results:
        respected = sum(1 for r in override_results if r.success)
        pct = (respected / len(override_results)) * 100
        status = "[PASS]" if pct == 100 else "[FAIL]"
        print(f"  {status} Employee overrides respected: {respected}/{len(override_results)} ({pct:.0f}%)")

    # Check category-room matching
    category_match = sum(1 for r in results if r.success)
    pct = (category_match / len(results)) * 100
    status = "[PASS]" if pct == 100 else "[FAIL]"
    print(f"  {status} Category-room matching: {category_match}/{len(results)} ({pct:.0f}%)")

    # Overall verdict
    print("\n" + "=" * 70)
    if passed == len(results):
        print("  *** ALL WORKFLOW TESTS PASSED ***")
        print("  System ready for integration with Go backend")
    elif passed >= len(results) * 0.9:
        print("  *** WORKFLOW TESTS MOSTLY PASSED ***")
        print("  Minor issues to address before production")
    else:
        print("  *** WORKFLOW TESTS NEED ATTENTION ***")
        print("  Review failed scenarios before proceeding")
    print("=" * 70)

    return results


def run_load_test():
    """Simulate high-volume usage"""
    print("\n\n" + "=" * 70)
    print("LOAD TEST - Simulating Peak Usage")
    print("=" * 70)

    sim = WorkflowSimulator()
    sim.load_data()

    # Simulate 100 entries being processed
    num_entries = 100
    customers = list(sim.customer_patterns.keys())

    print(f"\nProcessing {num_entries} entries...")

    times = []
    successes = 0

    for i in range(num_entries):
        customer_id = random.choice(customers)
        quantity = random.choice([10, 20, 30, 50, 80, 100])
        category = random.choice(['seed', 'sell'])

        # 30% chance employee selects room/floor
        selected_room = None
        selected_floor = None
        if random.random() < 0.3:
            if category == 'seed':
                selected_room = random.choice(['1', '2', None])
            else:
                selected_room = random.choice(['3', '4', None])
            selected_floor = random.choice(['0', '1', '2', '3', None])

        thock = sim.simulate_new_entry(customer_id, quantity, category)
        result = sim.simulate_room_config_page(thock, selected_room, selected_floor)

        times.append(result.time_ms)
        if result.success:
            successes += 1

        if (i + 1) % 25 == 0:
            print(f"  Processed {i + 1}/{num_entries}...")

    # Statistics
    avg_time = sum(times) / len(times)
    min_time = min(times)
    max_time = max(times)
    p95_time = sorted(times)[int(len(times) * 0.95)]
    p99_time = sorted(times)[int(len(times) * 0.99)]

    print(f"\n  Results:")
    print(f"  ─────────────────────────────────")
    print(f"  Total Requests:     {num_entries}")
    print(f"  Successful:         {successes} ({(successes/num_entries)*100:.1f}%)")
    print(f"  ")
    print(f"  Response Times:")
    print(f"  ─────────────────────────────────")
    print(f"  Average:            {avg_time:.2f} ms")
    print(f"  Min:                {min_time:.2f} ms")
    print(f"  Max:                {max_time:.2f} ms")
    print(f"  95th percentile:    {p95_time:.2f} ms")
    print(f"  99th percentile:    {p99_time:.2f} ms")
    print(f"  ")
    print(f"  Throughput:         ~{1000/avg_time:.0f} requests/second")

    if avg_time < 50 and successes == num_entries:
        print(f"\n  *** LOAD TEST PASSED ***")
    else:
        print(f"\n  *** LOAD TEST NEEDS REVIEW ***")


def run_edge_case_tests():
    """Test edge cases and boundary conditions"""
    print("\n\n" + "=" * 70)
    print("EDGE CASE TESTS")
    print("=" * 70)

    sim = WorkflowSimulator()
    sim.load_data()

    edge_cases = [
        # New customer (not in patterns)
        {
            'name': 'New Customer (No History)',
            'customer_id': 999999,  # Non-existent
            'quantity': 30,
            'category': 'seed',
            'expected': 'Should still provide recommendation',
        },
        # Very large quantity
        {
            'name': 'Very Large Quantity (500 bags)',
            'customer_id': list(sim.customer_patterns.keys())[0],
            'quantity': 500,
            'category': 'seed',
            'expected': 'Should handle large quantities',
        },
        # Minimum quantity
        {
            'name': 'Minimum Quantity (1 bag)',
            'customer_id': list(sim.customer_patterns.keys())[1],
            'quantity': 1,
            'category': 'seed',
            'expected': 'Should place near gallery',
        },
        # Gallery room with seed (mixed use)
        {
            'name': 'Gallery Room with Seed',
            'customer_id': list(sim.customer_patterns.keys())[2],
            'quantity': 25,
            'category': 'seed',
            'selected_room': 'G',
            'expected': 'Gallery accepts both categories',
        },
        # Floor 4 (fewer gatars)
        {
            'name': 'Floor 4 (120 gatars only)',
            'customer_id': list(sim.customer_patterns.keys())[3],
            'quantity': 40,
            'category': 'seed',
            'selected_floor': '4',
            'expected': 'Should use valid gatars for floor 4',
        },
    ]

    print("\nRunning edge case tests...")

    for i, case in enumerate(edge_cases, 1):
        print(f"\n[{i}/{len(edge_cases)}] {case['name']}")
        print(f"    Expected: {case['expected']}")

        thock = sim.simulate_new_entry(
            case['customer_id'],
            case['quantity'],
            case['category']
        )

        result = sim.simulate_room_config_page(
            thock,
            case.get('selected_room'),
            case.get('selected_floor')
        )

        if result.success:
            print(f"    [PASS] Got valid recommendation")
            print(f"    Placement: Room {result.final_placement['room_no']}, "
                  f"Floor {result.final_placement['floor']}, "
                  f"Gatar {result.final_placement['gatar_no']}")
        else:
            if result.recommendation:
                print(f"    [WARN] Got recommendation with validation issues: {result.validation_errors}")
            else:
                print(f"    [FAIL] No recommendation: {result.validation_errors}")


if __name__ == "__main__":
    # Run all tests
    run_workflow_tests()
    run_load_test()
    run_edge_case_tests()
