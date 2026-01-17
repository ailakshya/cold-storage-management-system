"""
Resource Analysis for Storage Recommendation System
===================================================
Analyzes: Memory, CPU, Data, Time, Dependencies
"""

import sys
import time
import json
from storage_recommendation import StorageRecommendationModel, GATAR_CONFIG, WEIGHTS


def analyze_resources():
    print("=" * 70)
    print("RESOURCE ANALYSIS - Storage Recommendation System")
    print("=" * 70)

    # Load data
    with open('inventory_full.json', 'r') as f:
        inventory = json.loads(f.read().strip())
    with open('customer_patterns_full.json', 'r') as f:
        patterns = json.loads(f.read().strip())

    # 1. DATA RESOURCES
    print("\n" + "─" * 70)
    print("1. DATA RESOURCES (What data is needed)")
    print("─" * 70)

    print(f"""
    Database Queries Required:
    ─────────────────────────
    a) Inventory Data (room_entries table):
       - room_no, floor, gate_no, quantity
       - Joined with entries for customer_id
       - Records: {len(inventory):,}

    b) Customer Pattern (on-demand):
       - Customer's existing storage locations
       - Aggregated by room/floor
       - Records per customer: ~5-20

    Data Size:
    ──────────
    - Inventory JSON: {sys.getsizeof(json.dumps(inventory)) / 1024:.1f} KB
    - Customer Patterns JSON: {sys.getsizeof(json.dumps(patterns)) / 1024:.1f} KB
    - Total in-memory: ~{(sys.getsizeof(json.dumps(inventory)) + sys.getsizeof(json.dumps(patterns))) / 1024:.1f} KB
    """)

    # 2. MEMORY RESOURCES
    print("\n" + "─" * 70)
    print("2. MEMORY RESOURCES")
    print("─" * 70)

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

    model_size = sys.getsizeof(model.inventory_data)
    for v in model.inventory_data.values():
        model_size += sys.getsizeof(v)

    print(f"""
    Model Memory Usage:
    ───────────────────
    - Inventory dict: ~{model_size / 1024:.1f} KB
    - Per gatar entry: ~{model_size / len(model.inventory_data):.0f} bytes
    - Customer pattern cache: ~500 bytes per customer

    Total Estimated Memory:
    ───────────────────────
    - Base model: ~{model_size / 1024:.1f} KB
    - With 100 cached customers: ~{(model_size + 50000) / 1024:.1f} KB
    - Maximum (all customers): ~{(model_size + 927 * 500) / 1024:.1f} KB

    ✓ Very lightweight - fits easily in memory
    """)

    # 3. CPU / COMPUTATION RESOURCES
    print("\n" + "─" * 70)
    print("3. CPU / COMPUTATION RESOURCES")
    print("─" * 70)

    # Build customer pattern for testing
    customer_patterns = {}
    for item in patterns:
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

    # Load one customer for testing
    test_cid = list(customer_patterns.keys())[0]
    model.customer_patterns[test_cid] = {
        'total_items': customer_patterns[test_cid]['total_items'],
        'total_thocks': len(customer_patterns[test_cid]['distribution']),
        'distribution': customer_patterns[test_cid]['distribution'],
        'primary_room': customer_patterns[test_cid]['primary_room'],
        'primary_floor': customer_patterns[test_cid]['primary_floor'],
    }

    # Time single recommendation
    times = []
    for _ in range(100):
        start = time.perf_counter()
        model.recommend(test_cid, 30, 'seed')
        end = time.perf_counter()
        times.append((end - start) * 1000)

    avg_time = sum(times) / len(times)
    min_time = min(times)
    max_time = max(times)

    print(f"""
    Recommendation Time (100 runs):
    ───────────────────────────────
    - Average: {avg_time:.2f} ms
    - Min: {min_time:.2f} ms
    - Max: {max_time:.2f} ms

    Operations per Recommendation:
    ──────────────────────────────
    - Rooms checked: 5 (or 1 if employee selected)
    - Floors checked: 5 (or 1 if employee selected)
    - Max locations scored: 25
    - Gatars scanned per location: ~140

    Complexity:
    ───────────
    - Time: O(rooms × floors × gatars) = O(25 × 140) = O(3,500)
    - With employee selection: O(1 × 1 × 140) = O(140)

    ✓ Very fast - can handle 100+ requests/second
    """)

    # 4. ALGORITHM DETAILS
    print("\n" + "─" * 70)
    print("4. ALGORITHM DETAILS (No ML Libraries Required)")
    print("─" * 70)

    print(f"""
    Scoring Factors:
    ────────────────
    1. Load Balance:        {WEIGHTS['load_balance']*100:.0f}% - Weight distribution (CPU cooler pattern)
    2. Customer Clustering: {WEIGHTS['customer_clustering']*100:.0f}% - Keep items together (if possible)
    3. Gallery Proximity:   {WEIGHTS['gallery_proximity']*100:.0f}% - Distance from gallery path
    4. Floor Score:         {WEIGHTS['floor_score']*100:.0f}% - Accessibility
    5. Capacity:            {WEIGHTS['capacity']*100:.0f}% - Available space
    6. Room Proximity:      {WEIGHTS['room_proximity']*100:.0f}% - Distance to exit

    Algorithm Type:
    ───────────────
    - Rule-based scoring (NOT machine learning)
    - Weighted sum of factors
    - No training required
    - No external ML libraries needed

    Dependencies:
    ─────────────
    Python Standard Library Only:
    - json (data parsing)
    - math (sqrt for variance)
    - dataclasses (data structures)
    - typing (type hints)

    Optional (for DB connection):
    - psycopg2 (PostgreSQL driver)

    ✓ No TensorFlow, PyTorch, scikit-learn, etc.
    ✓ Pure Python - easy to port to Go
    """)

    # 5. PRODUCTION REQUIREMENTS
    print("\n" + "─" * 70)
    print("5. PRODUCTION REQUIREMENTS (Go Implementation)")
    print("─" * 70)

    print(f"""
    For Go Implementation:
    ──────────────────────
    - No external ML libraries needed
    - Uses existing PostgreSQL connection
    - In-memory caching (optional)

    Database Queries:
    ─────────────────
    1. Load inventory (once, cache):
       SELECT room_no, floor, gate_no, SUM(quantity)
       FROM room_entries GROUP BY room_no, floor, gate_no

    2. Customer pattern (per request):
       SELECT room_no, floor, SUM(quantity)
       FROM room_entries re JOIN entries e ...
       WHERE customer_id = $1 GROUP BY room_no, floor

    API Response Time:
    ──────────────────
    - Algorithm: ~{avg_time:.1f} ms
    - DB query: ~10-50 ms
    - Total: ~50-100 ms

    Scalability:
    ────────────
    - Memory: ~1-2 MB for full inventory cache
    - CPU: Minimal (simple math operations)
    - Concurrent requests: 100+ easily

    ✓ Suitable for production
    ✓ No GPU required
    ✓ No external services required
    """)

    # 6. SUMMARY
    print("\n" + "=" * 70)
    print("SUMMARY - Resource Requirements")
    print("=" * 70)

    print(f"""
    ┌─────────────────────────────────────────────────────────────────┐
    │  Resource          │  Requirement                              │
    ├─────────────────────────────────────────────────────────────────┤
    │  Memory            │  ~1-2 MB (with cache)                     │
    │  CPU               │  Minimal (no ML)                          │
    │  GPU               │  Not required                             │
    │  Response Time     │  ~50-100 ms                               │
    │  Database          │  2 queries per request                    │
    │  External Services │  None                                     │
    │  ML Libraries      │  None (rule-based)                        │
    │  Dependencies      │  Standard library only                    │
    └─────────────────────────────────────────────────────────────────┘

    This is a LIGHTWEIGHT rule-based recommendation system, NOT a
    heavy machine learning model. It can run on any server without
    special hardware.
    """)


if __name__ == "__main__":
    analyze_resources()
