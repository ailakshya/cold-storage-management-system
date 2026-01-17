"""
Storage Location Recommendation Model
=====================================
Recommends optimal storage locations based on:
1. Customer clustering (keep items together)
2. Gallery proximity (based on quantity - small near gallery, large can go deep)
3. Floor accessibility
4. Current capacity utilization
5. Room proximity to exit

Rules:
- NO moving any items during retrieval
- Use ONLY pre-defined gallery paths
- Small quantities near gallery, large quantities can go deeper
"""

try:
    import psycopg2
except ImportError:
    psycopg2 = None  # Will use mock data

from dataclasses import dataclass
from typing import List, Dict, Optional
import math

# Database connection
DB_CONFIG = {
    'host': '192.168.15.120',
    'database': 'cold_db',
    'user': 'cold_user',
    'password': 'cold_password'  # Update with actual password
}

# Gatar configuration per room/floor
GATAR_CONFIG = {
    '1': {
        '0': {'start': 1, 'end': 140, 'cols': 14, 'rows': 10},
        '1': {'start': 141, 'end': 280, 'cols': 14, 'rows': 10},
        '2': {'start': 281, 'end': 420, 'cols': 14, 'rows': 10},
        '3': {'start': 421, 'end': 560, 'cols': 14, 'rows': 10},
        '4': {'start': 561, 'end': 680, 'cols': 12, 'rows': 10},
    },
    '2': {
        '0': {'start': 681, 'end': 820, 'cols': 14, 'rows': 10},
        '1': {'start': 821, 'end': 960, 'cols': 14, 'rows': 10},
        '2': {'start': 961, 'end': 1100, 'cols': 14, 'rows': 10},
        '3': {'start': 1101, 'end': 1240, 'cols': 14, 'rows': 10},
        '4': {'start': 1241, 'end': 1360, 'cols': 12, 'rows': 10},
    },
    '3': {
        '0': {'start': 1361, 'end': 1500, 'cols': 14, 'rows': 10},
        '1': {'start': 1501, 'end': 1640, 'cols': 14, 'rows': 10},
        '2': {'start': 1641, 'end': 1780, 'cols': 14, 'rows': 10},
        '3': {'start': 1781, 'end': 1920, 'cols': 14, 'rows': 10},
        '4': {'start': 1921, 'end': 2040, 'cols': 12, 'rows': 10},
    },
    '4': {
        '0': {'start': 2041, 'end': 2160, 'cols': 14, 'rows': 10},
        '1': {'start': 2121, 'end': 2260, 'cols': 14, 'rows': 10},
        '2': {'start': 2261, 'end': 2400, 'cols': 14, 'rows': 10},
        '3': {'start': 2401, 'end': 2540, 'cols': 14, 'rows': 10},
        '4': {'start': 2601, 'end': 2720, 'cols': 12, 'rows': 10},
    },
    'G': {
        '0': {'start': 2727, 'end': 2756, 'cols': 2, 'rows': 15},
        '1': {'start': 2757, 'end': 2784, 'cols': 2, 'rows': 14},
        '2': {'start': 2785, 'end': 2812, 'cols': 2, 'rows': 14},
        '3': {'start': 2813, 'end': 2840, 'cols': 2, 'rows': 14},
        '4': {'start': 2841, 'end': 2868, 'cols': 2, 'rows': 14},
    }
}

# Algorithm weights
WEIGHTS = {
    'load_balance': 0.25,
    'customer_clustering': 0.25,
    'gallery_proximity': 0.20,
    'floor_score': 0.15,
    'capacity': 0.10,
    'room_proximity': 0.05,
}

# Facility capacity
TOTAL_CAPACITY = 140000
TOTAL_GATARS = 2862


@dataclass
class GatarInfo:
    """Information about a single gatar"""
    gatar_no: int
    room_no: str
    floor: str
    column: int  # 1-14 (or 1-12 for floor 4, 1-2 for gallery)
    row: int     # 1-10 (or 1-15 for gallery)
    current_items: int
    customer_ids: List[int]
    thock_numbers: List[str]


@dataclass
class Recommendation:
    """Storage location recommendation"""
    room_no: str
    floor: str
    gatar_no: int
    score: float
    reasons: List[str]
    score_breakdown: Dict[str, float]
    current_items: int
    distance_from_gallery: int


class StorageRecommendationModel:
    """AI/ML model for storage location recommendations"""

    def __init__(self, db_conn=None):
        self.conn = db_conn
        self.inventory_data = {}
        self.customer_patterns = {}

    def connect_db(self):
        """Connect to database"""
        if not self.conn:
            self.conn = psycopg2.connect(**DB_CONFIG)
        return self.conn

    def load_inventory_data(self):
        """Load current inventory from database"""
        cursor = self.conn.cursor()

        # Get all room entries with quantities per gatar
        cursor.execute("""
            SELECT
                room_no,
                floor,
                gate_no,
                SUM(quantity) as total_items,
                array_agg(DISTINCT e.customer_id) as customer_ids,
                array_agg(DISTINCT re.thock_number) as thock_numbers
            FROM room_entries re
            JOIN entries e ON re.thock_number = e.thock_number
            WHERE room_no IS NOT NULL AND gate_no IS NOT NULL
            GROUP BY room_no, floor, gate_no
            ORDER BY room_no, floor, gate_no
        """)

        self.inventory_data = {}
        for row in cursor.fetchall():
            room_no, floor, gate_no, items, customer_ids, thock_numbers = row
            key = f"{room_no}-{floor}-{gate_no}"
            self.inventory_data[key] = {
                'room_no': room_no,
                'floor': floor,
                'gate_no': gate_no,
                'items': items or 0,
                'customer_ids': customer_ids or [],
                'thock_numbers': thock_numbers or []
            }

        cursor.close()
        print(f"Loaded {len(self.inventory_data)} gatar records")
        return self.inventory_data

    def load_customer_patterns(self, customer_id: int) -> Dict:
        """Load storage pattern for a specific customer"""
        cursor = self.conn.cursor()

        cursor.execute("""
            SELECT
                room_no,
                floor,
                SUM(quantity) as items,
                COUNT(DISTINCT thock_number) as thocks,
                array_agg(DISTINCT gate_no) as gatars
            FROM room_entries re
            JOIN entries e ON re.thock_number = e.thock_number
            WHERE e.customer_id = %s
            GROUP BY room_no, floor
            ORDER BY items DESC
        """, (customer_id,))

        pattern = {
            'total_items': 0,
            'total_thocks': 0,
            'distribution': [],
            'primary_room': None,
            'primary_floor': None,
        }

        for row in cursor.fetchall():
            room_no, floor, items, thocks, gatars = row
            pattern['distribution'].append({
                'room_no': room_no,
                'floor': floor,
                'items': items,
                'thocks': thocks,
                'gatars': gatars
            })
            pattern['total_items'] += items
            pattern['total_thocks'] += thocks

            if pattern['primary_room'] is None:
                pattern['primary_room'] = room_no
                pattern['primary_floor'] = floor

        cursor.close()
        self.customer_patterns[customer_id] = pattern
        return pattern

    def get_column_from_gatar(self, gatar_no: int, room_no: str, floor: str) -> int:
        """Calculate column position (distance from gallery) for a gatar"""
        config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
        if not config:
            return 7  # Default middle

        cols = config.get('cols', 14)
        start = config.get('start', 1)

        # Gallery rooms have only 2 columns
        if cols == 2:
            return 1  # Always near gallery

        # Calculate position within floor
        pos_in_floor = gatar_no - start
        if pos_in_floor < 0:
            return 7

        # Column pairs with 20 gatars each
        pair_index = pos_in_floor // 20
        col = (pair_index * 2) + 1 + (pos_in_floor % 2)

        return min(col, cols)

    def get_gallery_distance(self, column: int, total_cols: int = 14) -> int:
        """Calculate distance from gallery (columns 6-7 for 14-col, center for others)"""
        if total_cols == 2:
            return 0  # Gallery room

        gallery_center = total_cols // 2  # 7 for 14-col

        if column <= gallery_center:
            return gallery_center - column
        else:
            return column - gallery_center - 1

    def calculate_gallery_proximity_score(self, column: int, quantity: int, total_cols: int = 14) -> float:
        """
        Calculate gallery proximity score based on quantity
        - Small quantities (1-20) should be near gallery
        - Large quantities (50+) can go deeper
        """
        distance = self.get_gallery_distance(column, total_cols)

        if total_cols == 2:  # Gallery room
            return 1.0

        # Small quantity - must be near gallery
        if quantity <= 20:
            if distance == 0:
                return 1.0
            elif distance == 1:
                return 0.6
            elif distance == 2:
                return 0.3
            else:
                return 0.1  # Penalize deep placement for small quantities

        # Medium quantity
        elif quantity <= 50:
            if distance <= 1:
                return 1.0
            elif distance <= 2:
                return 0.8
            elif distance <= 3:
                return 0.6
            else:
                return 0.4

        # Large quantity - can go anywhere
        else:
            if distance <= 1:
                return 1.0
            elif distance <= 2:
                return 0.9
            elif distance <= 3:
                return 0.8
            else:
                return 0.7  # Deep spots OK for large quantities

    def calculate_customer_clustering_score(self, room_no: str, floor: str, customer_id: int) -> float:
        """Score based on keeping customer items together"""
        pattern = self.customer_patterns.get(customer_id)
        if not pattern or not pattern['distribution']:
            return 0.5  # New customer, neutral score

        # Same location as primary storage
        if room_no == pattern['primary_room'] and floor == pattern['primary_floor']:
            return 1.0

        # Same room, different floor
        if room_no == pattern['primary_room']:
            return 0.7

        # Same floor, different room
        for dist in pattern['distribution']:
            if dist['floor'] == floor:
                return 0.4

        # Different room and floor
        return 0.2

    def calculate_floor_score(self, floor: str) -> float:
        """Score based on floor accessibility"""
        floor_scores = {
            '0': 1.0,   # Ground - easiest
            '1': 0.9,
            '2': 0.8,
            '3': 0.6,
            '4': 0.4,   # Top - hardest, also fewer gatars
        }
        return floor_scores.get(floor, 0.5)

    def calculate_room_proximity_score(self, room_no: str) -> float:
        """Score based on room proximity to exit"""
        # Based on building layout - exit at bottom
        room_scores = {
            '3': 1.0,   # Closest to exit
            '4': 1.0,   # Closest to exit
            'G': 0.8,   # Center
            '1': 0.6,   # Farthest
            '2': 0.6,   # Farthest
        }
        return room_scores.get(room_no, 0.5)

    def calculate_capacity_score(self, room_no: str, floor: str) -> float:
        """Score based on current capacity utilization"""
        config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
        if not config:
            return 0.5

        total_gatars = config['end'] - config['start'] + 1

        # Count occupied gatars
        occupied = 0
        total_items = 0
        for key, data in self.inventory_data.items():
            if data['room_no'] == room_no and data['floor'] == floor:
                occupied += 1
                total_items += data['items']

        utilization = occupied / total_gatars if total_gatars > 0 else 1.0

        if utilization < 0.5:
            return 1.0
        elif utilization < 0.75:
            return 0.5
        elif utilization < 0.95:
            return 0.25
        else:
            return 0.0  # Skip full locations

    def calculate_load_balance_score(self, room_no: str, floor: str, quantity: int) -> float:
        """Score based on weight distribution (CPU cooler pattern)"""
        config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
        if not config:
            return 0.5

        cols = config.get('cols', 14)

        # Calculate current load per quadrant
        quadrants = {'Q1': 0, 'Q2': 0, 'Q3': 0, 'Q4': 0}

        for key, data in self.inventory_data.items():
            if data['room_no'] == room_no and data['floor'] == floor:
                # Handle gate_no that might have multiple values like "1, 2, 3"
                gate_no_str = str(data['gate_no']).split(',')[0].strip()
                try:
                    gate_no = int(gate_no_str)
                except ValueError:
                    continue
                col = self.get_column_from_gatar(gate_no, room_no, floor)
                row = (gate_no - config['start']) // cols

                # Determine quadrant
                if col <= cols // 2:
                    if row < 5:
                        quadrants['Q1'] += data['items']
                    else:
                        quadrants['Q3'] += data['items']
                else:
                    if row < 5:
                        quadrants['Q2'] += data['items']
                    else:
                        quadrants['Q4'] += data['items']

        # Calculate variance
        values = list(quadrants.values())
        if max(values) == 0:
            return 1.0  # Empty floor, perfect balance

        avg = sum(values) / 4
        variance = sum((v - avg) ** 2 for v in values) / 4
        std_dev = math.sqrt(variance)

        # Normalize score
        max_imbalance = avg * 2  # 200% imbalance threshold
        if std_dev < avg * 0.2:
            return 1.0  # Within 20%
        elif std_dev < avg * 0.3:
            return 0.8
        elif std_dev < avg * 0.5:
            return 0.5
        else:
            return 0.2

    def get_available_gatars(self, room_no: str, floor: str) -> List[int]:
        """Get list of available (empty or low occupancy) gatars"""
        config = GATAR_CONFIG.get(room_no, {}).get(floor, {})
        if not config:
            return []

        available = []
        for gatar in range(config['start'], config['end'] + 1):
            key = f"{room_no}-{floor}-{gatar}"
            data = self.inventory_data.get(key)

            if not data or data['items'] < 100:  # Empty or low occupancy
                available.append(gatar)

        return available

    def recommend(
        self,
        customer_id: int,
        quantity: int,
        category: str = 'seed',
        selected_room: str = None,
        selected_floor: str = None
    ) -> List[Recommendation]:
        """
        Generate storage location recommendations

        Args:
            customer_id: Customer ID
            quantity: Number of items to store
            category: 'seed' or 'sell'
            selected_room: Employee-selected room (optional - if provided, prioritize this room)
            selected_floor: Employee-selected floor (optional - if provided, prioritize this floor)

        Returns:
            List of recommendations sorted by score
        """
        # Load customer pattern (only if DB connected and not already loaded)
        if self.conn and customer_id not in self.customer_patterns:
            self.load_customer_patterns(customer_id)

        recommendations = []

        # If employee selected specific room/floor, adjust weights
        # Customer clustering becomes lower priority when employee chooses location
        employee_selected = selected_room is not None or selected_floor is not None

        # Determine rooms to consider based on category
        # STRICT: Only check rooms appropriate for the category
        if selected_room:
            # Employee chose specific room - only recommend gatars in that room
            rooms_to_check = [selected_room]
        elif category == 'seed':
            # Seed: Only Room 1, 2, and Gallery (G)
            rooms_to_check = ['1', '2', 'G']
        else:
            # Sell: Only Room 3, 4, and Gallery (G)
            rooms_to_check = ['3', '4', 'G']

        # Determine floors to consider
        if selected_floor:
            floors_to_check = [selected_floor]
        else:
            floors_to_check = ['0', '1', '2', '3', '4']

        # Adjust preferred rooms if not employee selected
        if not selected_room:
            if category == 'seed':
                preferred_rooms = ['1', '2']
            else:
                preferred_rooms = ['3', '4', 'G']
        else:
            preferred_rooms = [selected_room]

        # Score all available locations
        for room_no in rooms_to_check:
            for floor in floors_to_check:
                config = GATAR_CONFIG.get(room_no, {}).get(floor)
                if not config:
                    continue

                # Get available gatars
                available = self.get_available_gatars(room_no, floor)
                if not available:
                    continue

                # Calculate scores
                capacity_score = self.calculate_capacity_score(room_no, floor)
                if capacity_score == 0:
                    continue  # Skip full locations

                clustering_score = self.calculate_customer_clustering_score(room_no, floor, customer_id)
                floor_score = self.calculate_floor_score(floor)
                room_score = self.calculate_room_proximity_score(room_no)
                load_balance = self.calculate_load_balance_score(room_no, floor, quantity)

                # Find best gatar based on gallery proximity
                best_gatar = None
                best_gallery_score = 0
                best_distance = 999

                for gatar in available:
                    col = self.get_column_from_gatar(gatar, room_no, floor)
                    cols = config.get('cols', 14)
                    gallery_score = self.calculate_gallery_proximity_score(col, quantity, cols)
                    distance = self.get_gallery_distance(col, cols)

                    if gallery_score > best_gallery_score or (gallery_score == best_gallery_score and distance < best_distance):
                        best_gallery_score = gallery_score
                        best_gatar = gatar
                        best_distance = distance

                if not best_gatar:
                    continue

                # Adjust weights based on employee selection
                if employee_selected:
                    # When employee selects room/floor, reduce clustering weight
                    # Focus more on gallery proximity and load balance
                    adjusted_weights = {
                        'load_balance': 0.30,
                        'customer_clustering': 0.10,  # Lower - employee chose location
                        'gallery_proximity': 0.30,    # Higher - optimize within selected area
                        'floor_score': 0.15,
                        'capacity': 0.10,
                        'room_proximity': 0.05,
                    }
                else:
                    adjusted_weights = WEIGHTS

                # Calculate final score
                score_breakdown = {
                    'load_balance': load_balance * adjusted_weights['load_balance'],
                    'customer_clustering': clustering_score * adjusted_weights['customer_clustering'],
                    'gallery_proximity': best_gallery_score * adjusted_weights['gallery_proximity'],
                    'floor_score': floor_score * adjusted_weights['floor_score'],
                    'capacity': capacity_score * adjusted_weights['capacity'],
                    'room_proximity': room_score * adjusted_weights['room_proximity'],
                }

                # Bonus for preferred room based on category
                category_bonus = 0.05 if room_no in preferred_rooms else 0

                # Extra bonus if this is the employee-selected room/floor
                employee_bonus = 0
                if selected_room and room_no == selected_room:
                    employee_bonus += 0.05
                if selected_floor and floor == selected_floor:
                    employee_bonus += 0.05

                final_score = sum(score_breakdown.values()) + category_bonus + employee_bonus

                # Generate reasons
                reasons = []
                if clustering_score >= 0.7:
                    reasons.append("Customer has items here")
                if best_gallery_score >= 0.8:
                    reasons.append("Easy gallery access")
                if floor_score >= 0.8:
                    reasons.append("Accessible floor")
                if capacity_score >= 0.5:
                    reasons.append("Good space available")
                if room_no in preferred_rooms:
                    reasons.append(f"Preferred for {category}")

                # Get current items in this floor
                current_items = sum(
                    d['items'] for d in self.inventory_data.values()
                    if d['room_no'] == room_no and d['floor'] == floor
                )

                recommendations.append(Recommendation(
                    room_no=room_no,
                    floor=floor,
                    gatar_no=best_gatar,
                    score=round(final_score * 100, 1),
                    reasons=reasons,
                    score_breakdown={k: round(v * 100, 1) for k, v in score_breakdown.items()},
                    current_items=current_items,
                    distance_from_gallery=best_distance
                ))

        # Sort by score descending
        recommendations.sort(key=lambda x: x.score, reverse=True)

        return recommendations[:5]  # Return top 5


def test_model():
    """Test the recommendation model with sample data"""
    print("=" * 60)
    print("Storage Recommendation Model - Test")
    print("=" * 60)

    # Try to connect to database
    try:
        model = StorageRecommendationModel()
        model.connect_db()
        print("Connected to database")

        # Load inventory data
        model.load_inventory_data()

        # Get a sample customer for testing
        cursor = model.conn.cursor()
        cursor.execute("""
            SELECT DISTINCT e.customer_id, c.name, COUNT(DISTINCT re.id) as entries
            FROM room_entries re
            JOIN entries e ON re.thock_number = e.thock_number
            JOIN customers c ON e.customer_id = c.id
            GROUP BY e.customer_id, c.name
            ORDER BY entries DESC
            LIMIT 5
        """)

        print("\nTop 5 customers by entries:")
        customers = cursor.fetchall()
        for cid, name, entries in customers:
            print(f"  Customer {cid}: {name} ({entries} entries)")

        cursor.close()

        if customers:
            # Test with first customer
            test_customer_id = customers[0][0]
            test_customer_name = customers[0][1]

            print(f"\n{'=' * 60}")
            print(f"Testing recommendations for: {test_customer_name} (ID: {test_customer_id})")
            print(f"{'=' * 60}")

            # Test with different quantities
            for qty in [10, 30, 80]:
                print(f"\n--- Quantity: {qty} bags (category: seed) ---")
                recommendations = model.recommend(test_customer_id, qty, 'seed')

                for i, rec in enumerate(recommendations, 1):
                    print(f"\n  #{i} Room {rec.room_no}, Floor {rec.floor}, Gatar {rec.gatar_no}")
                    print(f"      Score: {rec.score}")
                    print(f"      Distance from gallery: {rec.distance_from_gallery} columns")
                    print(f"      Reasons: {', '.join(rec.reasons)}")
                    print(f"      Breakdown: {rec.score_breakdown}")

        model.conn.close()

    except Exception as e:
        print(f"Error: {e}")
        print("\nRunning with mock data instead...")
        test_with_mock_data()


def test_with_mock_data():
    """Test with mock data if database is not available"""
    print("\n--- Testing with mock data ---")

    model = StorageRecommendationModel()

    # Create mock inventory data
    model.inventory_data = {
        '1-0-5': {'room_no': '1', 'floor': '0', 'gate_no': '5', 'items': 50, 'customer_ids': [1], 'thock_numbers': ['T001']},
        '1-0-6': {'room_no': '1', 'floor': '0', 'gate_no': '6', 'items': 45, 'customer_ids': [1], 'thock_numbers': ['T002']},
        '1-0-7': {'room_no': '1', 'floor': '0', 'gate_no': '7', 'items': 30, 'customer_ids': [2], 'thock_numbers': ['T003']},
        '2-0-10': {'room_no': '2', 'floor': '0', 'gate_no': '10', 'items': 60, 'customer_ids': [1], 'thock_numbers': ['T004']},
    }

    # Create mock customer pattern
    model.customer_patterns[1] = {
        'total_items': 155,
        'total_thocks': 3,
        'distribution': [
            {'room_no': '1', 'floor': '0', 'items': 95, 'thocks': 2, 'gatars': ['5', '6']},
            {'room_no': '2', 'floor': '0', 'items': 60, 'thocks': 1, 'gatars': ['10']},
        ],
        'primary_room': '1',
        'primary_floor': '0',
    }

    print("\nMock inventory loaded")
    print(f"Customer 1 has items in Room 1 Floor 0 (gatars 5,6) and Room 2 Floor 0 (gatar 10)")

    # Test recommendations
    for qty in [10, 30, 80]:
        print(f"\n--- Quantity: {qty} bags ---")
        recommendations = model.recommend(1, qty, 'seed')

        for i, rec in enumerate(recommendations[:3], 1):
            print(f"  #{i} Room {rec.room_no}, Floor {rec.floor}, Gatar {rec.gatar_no}")
            print(f"      Score: {rec.score}, Gallery distance: {rec.distance_from_gallery}")
            print(f"      Reasons: {', '.join(rec.reasons)}")


if __name__ == "__main__":
    test_model()
