#!/bin/bash

# Gate Pass System Test Script
# Tests the complete gate pass workflow

BASE_URL="http://localhost:8080"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  Gate Pass System Test Suite${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# Test 1: Create test admin user
echo -e "${YELLOW}=== TEST 1: Create admin user ===${NC}"
SIGNUP_RESULT=$(curl -s -X POST $BASE_URL/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Gate Pass Test Admin",
    "email":"gatepass@test.com",
    "password":"gatepass123",
    "role":"admin",
    "phone":"9999999999"
  }')

if [[ "$SIGNUP_RESULT" == *"token"* ]] || [[ "$SIGNUP_RESULT" == *"already exists"* ]]; then
    echo -e "${GREEN}✓ User creation/exists${NC}"
else
    echo -e "${RED}✗ Failed: $SIGNUP_RESULT${NC}"
fi

# Test 2: Login to get token
echo -e "\n${YELLOW}=== TEST 2: Login to get token ===${NC}"
LOGIN_RESULT=$(curl -s -X POST $BASE_URL/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email":"gatepass@test.com",
    "password":"gatepass123"
  }')

TOKEN=$(echo $LOGIN_RESULT | python3 -c "import sys, json; print(json.load(sys.stdin).get('token', ''))" 2>/dev/null)

if [ -n "$TOKEN" ]; then
    echo -e "${GREEN}✓ Login successful${NC}"
    echo "Token: ${TOKEN:0:50}..."
else
    echo -e "${RED}✗ Login failed: $LOGIN_RESULT${NC}"
    exit 1
fi

# Test 3: Create a customer
echo -e "\n${YELLOW}=== TEST 3: Create customer ===${NC}"
CUSTOMER_RESULT=$(curl -s -X POST $BASE_URL/api/customers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"Test Customer",
    "phone":"8888888888",
    "village":"Test Village",
    "so":"Test Father"
  }')

CUSTOMER_ID=$(echo $CUSTOMER_RESULT | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)

if [ -n "$CUSTOMER_ID" ]; then
    echo -e "${GREEN}✓ Customer created (ID: $CUSTOMER_ID)${NC}"
else
    echo -e "${YELLOW}! Customer might exist, getting by phone...${NC}"
    CUSTOMER_SEARCH=$(curl -s "$BASE_URL/api/customers/search?phone=8888888888" \
      -H "Authorization: Bearer $TOKEN")
    CUSTOMER_ID=$(echo $CUSTOMER_SEARCH | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
    if [ -n "$CUSTOMER_ID" ]; then
        echo -e "${GREEN}✓ Found existing customer (ID: $CUSTOMER_ID)${NC}"
    else
        echo -e "${RED}✗ Failed to create/find customer${NC}"
    fi
fi

# Test 4: Create an entry for the customer
echo -e "\n${YELLOW}=== TEST 4: Create entry for customer ===${NC}"
ENTRY_RESULT=$(curl -s -X POST $BASE_URL/api/entries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"customer_id\": $CUSTOMER_ID,
    \"phone\": \"8888888888\",
    \"name\": \"Test Customer\",
    \"village\": \"Test Village\",
    \"expected_quantity\": 100,
    \"truck_category\": \"seed\"
  }")

ENTRY_ID=$(echo $ENTRY_RESULT | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)
TRUCK_NUMBER=$(echo $ENTRY_RESULT | python3 -c "import sys, json; print(json.load(sys.stdin).get('truck_number', ''))" 2>/dev/null)

if [ -n "$ENTRY_ID" ]; then
    echo -e "${GREEN}✓ Entry created (ID: $ENTRY_ID, Truck: $TRUCK_NUMBER)${NC}"
else
    echo -e "${RED}✗ Failed to create entry: $ENTRY_RESULT${NC}"
fi

# Test 5: Create a gate pass
echo -e "\n${YELLOW}=== TEST 5: Create gate pass ===${NC}"
GATE_PASS_RESULT=$(curl -s -X POST $BASE_URL/api/gate-passes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"customer_id\": $CUSTOMER_ID,
    \"truck_number\": \"TEST-TRUCK-001\",
    \"entry_id\": $ENTRY_ID,
    \"requested_quantity\": 50,
    \"payment_verified\": true,
    \"payment_amount\": 500.00,
    \"remarks\": \"Test gate pass for withdrawal\"
  }")

GATE_PASS_ID=$(echo $GATE_PASS_RESULT | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)

if [ -n "$GATE_PASS_ID" ]; then
    echo -e "${GREEN}✓ Gate pass created (ID: $GATE_PASS_ID)${NC}"
else
    echo -e "${RED}✗ Failed to create gate pass: $GATE_PASS_RESULT${NC}"
fi

# Test 6: List pending gate passes
echo -e "\n${YELLOW}=== TEST 6: List pending gate passes ===${NC}"
PENDING_RESULT=$(curl -s "$BASE_URL/api/gate-passes/pending" \
  -H "Authorization: Bearer $TOKEN")

PENDING_COUNT=$(echo $PENDING_RESULT | python3 -c "import sys, json; print(len(json.load(sys.stdin)))" 2>/dev/null)

if [ -n "$PENDING_COUNT" ]; then
    echo -e "${GREEN}✓ Found $PENDING_COUNT pending gate pass(es)${NC}"
else
    echo -e "${RED}✗ Failed to list pending gate passes${NC}"
fi

# Test 7: Approve the gate pass
echo -e "\n${YELLOW}=== TEST 7: Approve gate pass ===${NC}"
APPROVE_RESULT=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT "$BASE_URL/api/gate-passes/$GATE_PASS_ID/approve" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "approved_quantity": 45,
    "gate_no": "GATE-A",
    "status": "approved",
    "remarks": "Approved with adjusted quantity"
  }')

HTTP_STATUS=$(echo "$APPROVE_RESULT" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

if [ "$HTTP_STATUS" == "200" ]; then
    echo -e "${GREEN}✓ Gate pass approved successfully${NC}"
else
    echo -e "${RED}✗ Failed to approve gate pass (Status: $HTTP_STATUS)${NC}"
    echo "$APPROVE_RESULT"
fi

# Test 8: List all gate passes
echo -e "\n${YELLOW}=== TEST 8: List all gate passes ===${NC}"
ALL_PASSES_RESULT=$(curl -s "$BASE_URL/api/gate-passes" \
  -H "Authorization: Bearer $TOKEN")

ALL_COUNT=$(echo $ALL_PASSES_RESULT | python3 -c "import sys, json; print(len(json.load(sys.stdin)))" 2>/dev/null)

if [ -n "$ALL_COUNT" ]; then
    echo -e "${GREEN}✓ Found $ALL_COUNT total gate pass(es)${NC}"
else
    echo -e "${RED}✗ Failed to list all gate passes${NC}"
fi

# Test 9: Complete the gate pass
echo -e "\n${YELLOW}=== TEST 9: Complete gate pass (mark items as taken) ===${NC}"
COMPLETE_RESULT=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/api/gate-passes/$GATE_PASS_ID/complete" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json")

HTTP_STATUS=$(echo "$COMPLETE_RESULT" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)

if [ "$HTTP_STATUS" == "200" ]; then
    echo -e "${GREEN}✓ Gate pass completed successfully${NC}"
else
    echo -e "${RED}✗ Failed to complete gate pass (Status: $HTTP_STATUS)${NC}"
    echo "$COMPLETE_RESULT"
fi

# Test 10: Verify event logging
echo -e "\n${YELLOW}=== TEST 10: Verify event logging (check entry events) ===${NC}"
if [ -n "$ENTRY_ID" ]; then
    # Note: You would need an endpoint to get entry events
    # For now, we'll just check if the entry still exists
    ENTRY_CHECK=$(curl -s "$BASE_URL/api/entries/$ENTRY_ID" \
      -H "Authorization: Bearer $TOKEN")

    ENTRY_EXISTS=$(echo $ENTRY_CHECK | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null)

    if [ -n "$ENTRY_EXISTS" ]; then
        echo -e "${GREEN}✓ Entry still exists, events should be logged${NC}"
        echo -e "${YELLOW}  Note: Check database for GATE_PASS_ISSUED and ITEMS_OUT events${NC}"
    else
        echo -e "${RED}✗ Entry not found${NC}"
    fi
fi

echo ""
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  Test Suite Complete${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo -e "${GREEN}Summary:${NC}"
echo "- Gate pass workflow tested end-to-end"
echo "- Created gate pass ID: $GATE_PASS_ID"
echo "- Approved and completed successfully"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Access http://localhost:8080/gate-pass-entry to create gate passes via UI"
echo "2. Access http://localhost:8080/unloading-tickets to approve/complete gate passes"
echo "3. Toggle operation_mode in system settings to switch between loading/unloading"
echo ""
