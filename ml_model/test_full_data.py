"""
Full Data Workflow Test - 100% of Production Data
==================================================
Tests the recommendation system on ALL customers with complete workflow simulation.
"""

import json
import time
import random
from collections import defaultdict
from dataclasses import dataclass
from typing import Optional, List, Dict
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG, WEIGHTS


@dataclass
class TestResult:
    customer_id: int
    customer_name: str
    quantity: int
    category: str
    selected_room: Optional[str]
    selected_floor: Optional[str]
    success: bool
    placement: Optional[dict]
    errors: List[str]
    time_ms: float


def load_all_data():
    """Load all production data"""
    with open('inventory_full.json', 'r') as f:
        inventory = json.loads(f.read().strip())
    with open('customer_patterns_full.json', 'r') as f:
        patterns = json.loads(f.read().strip())
    return inventory, patterns


def validate_placement(category: str, quantity: int, placement: dict,
                      selected_room: Optional[str], selected_floor: Optional[str]) -> List[str]:
    """Validate placement follows all business rules"""
    errors = []

    room = placement['room_no']
    floor = placement['floor']
    gatar = placement['gatar_no']

    # Rule 1: Category-room matching
    if category == 'seed':
        if room not in ['1', '2', 'G']:
            errors.append(f"Seed in sell room {room}")
    else:
        if room not in ['3', '4', 'G']:
            errors.append(f"Sell in seed room {room}")

    # Rule 2: Employee room override
    if selected_room and room != selected_room:
        errors.append(f"Room override ignored: wanted {selected_room}, got {room}")

    # Rule 3: Employee floor override
    if selected_floor and floor != selected_floor:
        errors.append(f"Floor override ignored: wanted {selected_floor}, got {floor}")

    # Rule 4: Valid floor
    if floor not in ['0', '1', '2', '3', '4']:
        errors.append(f"Invalid floor: {floor}")

    # Rule 5: Valid gatar for room/floor
    config = GATAR_CONFIG.get(room, {}).get(floor, {})
    if config:
        if not (config['start'] <= gatar <= config['end']):
            errors.append(f"Invalid gatar {gatar} for {room}/{floor}")
    else:
        errors.append(f"No config for {room}/{floor}")

    return errors


def run_full_data_test():
    """Test on 100% of production data"""
    print("=" * 70)
    print("FULL DATA WORKFLOW TEST - 100% of Production Data")
    print("=" * 70)

    # Load data
    inventory, customer_data = load_all_data()
    print(f"\nLoaded {len(inventory)} inventory records")
    print(f"Loaded {len(customer_data)} customer pattern records")

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

    # Build ALL customer patterns
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

    # Load ALL customer patterns into model
    for cid, data in customer_patterns.items():
        model.customer_patterns[cid] = {
            'total_items': data['total_items'],
            'total_thocks': len(data['distribution']),
            'distribution': data['distribution'],
            'primary_room': data['primary_room'],
            'primary_floor': data['primary_floor'],
        }

    total_customers = len(customer_patterns)
    print(f"\nTesting on ALL {total_customers} customers (100%)")

    # Test configurations
    test_configs = [
        # (quantity, category, room_override, floor_override, description)
        (10, 'seed', None, None, 'Small seed - auto'),
        (30, 'seed', None, None, 'Medium seed - auto'),
        (80, 'seed', None, None, 'Large seed - auto'),
        (25, 'sell', None, None, 'Medium sell - auto'),
        (50, 'sell', None, None, 'Large sell - auto'),
        (20, 'seed', '1', None, 'Seed - Room 1 override'),
        (20, 'seed', '2', None, 'Seed - Room 2 override'),
        (20, 'sell', '3', None, 'Sell - Room 3 override'),
        (20, 'sell', '4', None, 'Sell - Room 4 override'),
        (30, 'seed', None, '0', 'Seed - Floor 0 override'),
        (30, 'seed', None, '2', 'Seed - Floor 2 override'),
        (40, 'seed', '1', '1', 'Seed - Room 1, Floor 1'),
        (40, 'sell', '4', '0', 'Sell - Room 4, Floor 0'),
    ]

    # Results storage
    all_results: List[TestResult] = []

    # Metrics by test config
    config_metrics = defaultdict(lambda: {
        'total': 0, 'passed': 0, 'failed': 0,
        'times': [], 'errors': defaultdict(int)
    })

    # Overall metrics
    total_tests = 0
    total_passed = 0
    total_failed = 0
    all_times = []
    error_counts = defaultdict(int)

    print("\n" + "─" * 70)
    print("RUNNING TESTS ON ALL CUSTOMERS")
    print("─" * 70)

    customers_list = list(customer_patterns.items())

    for config_idx, (qty, cat, room, floor, desc) in enumerate(test_configs):
        print(f"\n[Config {config_idx + 1}/{len(test_configs)}] {desc}")
        print(f"  Quantity: {qty}, Category: {cat}, Room: {room or 'auto'}, Floor: {floor or 'auto'}")

        config_passed = 0
        config_failed = 0
        config_times = []

        for i, (cid, cust_data) in enumerate(customers_list):
            if i % 200 == 0 and i > 0:
                print(f"    Progress: {i}/{total_customers} customers...")

            start = time.perf_counter()

            try:
                recommendations = model.recommend(
                    customer_id=cid,
                    quantity=qty,
                    category=cat,
                    selected_room=room,
                    selected_floor=floor
                )

                elapsed = (time.perf_counter() - start) * 1000

                if recommendations:
                    rec = recommendations[0]
                    placement = {
                        'room_no': rec.room_no,
                        'floor': rec.floor,
                        'gatar_no': rec.gatar_no,
                        'score': rec.score,
                        'distance': rec.distance_from_gallery
                    }

                    errors = validate_placement(cat, qty, placement, room, floor)

                    result = TestResult(
                        customer_id=cid,
                        customer_name=cust_data['name'],
                        quantity=qty,
                        category=cat,
                        selected_room=room,
                        selected_floor=floor,
                        success=len(errors) == 0,
                        placement=placement,
                        errors=errors,
                        time_ms=elapsed
                    )

                    if result.success:
                        config_passed += 1
                        total_passed += 1
                    else:
                        config_failed += 1
                        total_failed += 1
                        for err in errors:
                            error_counts[err] += 1
                            config_metrics[desc]['errors'][err] += 1
                else:
                    result = TestResult(
                        customer_id=cid,
                        customer_name=cust_data['name'],
                        quantity=qty,
                        category=cat,
                        selected_room=room,
                        selected_floor=floor,
                        success=False,
                        placement=None,
                        errors=['No recommendations returned'],
                        time_ms=elapsed
                    )
                    config_failed += 1
                    total_failed += 1
                    error_counts['No recommendations'] += 1

                config_times.append(elapsed)
                all_times.append(elapsed)
                all_results.append(result)
                total_tests += 1

            except Exception as e:
                elapsed = (time.perf_counter() - start) * 1000
                result = TestResult(
                    customer_id=cid,
                    customer_name=cust_data['name'],
                    quantity=qty,
                    category=cat,
                    selected_room=room,
                    selected_floor=floor,
                    success=False,
                    placement=None,
                    errors=[f'Exception: {str(e)}'],
                    time_ms=elapsed
                )
                config_failed += 1
                total_failed += 1
                error_counts[f'Exception: {type(e).__name__}'] += 1
                all_results.append(result)
                total_tests += 1

        # Config summary
        config_pct = (config_passed / total_customers) * 100 if total_customers > 0 else 0
        avg_time = sum(config_times) / len(config_times) if config_times else 0

        config_metrics[desc]['total'] = total_customers
        config_metrics[desc]['passed'] = config_passed
        config_metrics[desc]['failed'] = config_failed
        config_metrics[desc]['times'] = config_times

        status = "PASS" if config_pct == 100 else "WARN" if config_pct >= 95 else "FAIL"
        print(f"  [{status}] {config_passed}/{total_customers} passed ({config_pct:.1f}%), avg time: {avg_time:.2f}ms")

    # Final Summary
    print("\n" + "=" * 70)
    print("FULL DATA TEST RESULTS")
    print("=" * 70)

    overall_pct = (total_passed / total_tests) * 100 if total_tests > 0 else 0
    avg_time = sum(all_times) / len(all_times) if all_times else 0
    min_time = min(all_times) if all_times else 0
    max_time = max(all_times) if all_times else 0

    print(f"\n  Total Tests:     {total_tests:,}")
    print(f"  Total Customers: {total_customers:,}")
    print(f"  Test Configs:    {len(test_configs)}")
    print(f"  ")
    print(f"  Passed:          {total_passed:,} ({overall_pct:.2f}%)")
    print(f"  Failed:          {total_failed:,} ({100-overall_pct:.2f}%)")

    print(f"\n  Response Times:")
    print(f"  ─────────────────────────────────")
    print(f"  Average:         {avg_time:.2f} ms")
    print(f"  Min:             {min_time:.2f} ms")
    print(f"  Max:             {max_time:.2f} ms")

    if all_times:
        sorted_times = sorted(all_times)
        p50 = sorted_times[int(len(sorted_times) * 0.50)]
        p95 = sorted_times[int(len(sorted_times) * 0.95)]
        p99 = sorted_times[int(len(sorted_times) * 0.99)]
        print(f"  50th percentile: {p50:.2f} ms")
        print(f"  95th percentile: {p95:.2f} ms")
        print(f"  99th percentile: {p99:.2f} ms")
        print(f"  ")
        print(f"  Throughput:      ~{1000/avg_time:.0f} requests/second")

    # Error breakdown
    if error_counts:
        print(f"\n  Error Breakdown:")
        print(f"  ─────────────────────────────────")
        for err, count in sorted(error_counts.items(), key=lambda x: -x[1]):
            print(f"    {err}: {count}")

    # Config breakdown
    print(f"\n  Results by Configuration:")
    print(f"  ─────────────────────────────────")
    for desc, metrics in config_metrics.items():
        pct = (metrics['passed'] / metrics['total']) * 100 if metrics['total'] > 0 else 0
        avg = sum(metrics['times']) / len(metrics['times']) if metrics['times'] else 0
        status = "PASS" if pct == 100 else "WARN" if pct >= 95 else "FAIL"
        print(f"  [{status}] {desc}: {metrics['passed']}/{metrics['total']} ({pct:.1f}%), {avg:.2f}ms")

    # Business rule validation
    print(f"\n" + "─" * 70)
    print("BUSINESS RULE VALIDATION")
    print("─" * 70)

    # Check category-room matching
    category_correct = sum(1 for r in all_results if r.success or
                          not any('seed' in e.lower() or 'sell' in e.lower() for e in r.errors))
    category_pct = (category_correct / total_tests) * 100 if total_tests > 0 else 0

    # Check employee overrides
    override_tests = [r for r in all_results if r.selected_room or r.selected_floor]
    override_correct = sum(1 for r in override_tests if r.success or
                          not any('override' in e.lower() for e in r.errors))
    override_pct = (override_correct / len(override_tests)) * 100 if override_tests else 100

    # Check gallery proximity for small quantities
    small_qty_results = [r for r in all_results if r.quantity <= 20 and r.placement]
    small_near_gallery = sum(1 for r in small_qty_results if r.placement['distance'] <= 1)
    small_pct = (small_near_gallery / len(small_qty_results)) * 100 if small_qty_results else 100

    print(f"\n  Category-Room Matching: {category_pct:.1f}% correct")
    print(f"  Employee Overrides:     {override_pct:.1f}% respected")
    print(f"  Small Qty Near Gallery: {small_pct:.1f}% within 1 column")

    # Final verdict
    print(f"\n" + "=" * 70)
    if overall_pct == 100:
        print("  *** ALL TESTS PASSED - SYSTEM READY FOR PRODUCTION ***")
    elif overall_pct >= 99:
        print("  *** TESTS MOSTLY PASSED - MINOR ISSUES TO REVIEW ***")
    elif overall_pct >= 95:
        print("  *** TESTS ACCEPTABLE - SOME ISSUES TO ADDRESS ***")
    else:
        print("  *** TESTS NEED ATTENTION - REVIEW FAILURES ***")
    print("=" * 70)

    return all_results, config_metrics


if __name__ == "__main__":
    run_full_data_test()
