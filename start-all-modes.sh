#!/bin/sh

# Start all three server modes concurrently
# This script is used when MODE=all in docker-compose.yml

echo "Starting all server modes..."

# Start employee mode on port 8080
./cold-backend --mode employee --port 8080 &
EMPLOYEE_PID=$!
echo "Started employee mode (PID: $EMPLOYEE_PID) on port 8080"

# Start customer portal on port 8081
./cold-backend --mode customer --port 8081 &
CUSTOMER_PID=$!
echo "Started customer portal (PID: $CUSTOMER_PID) on port 8081"

# Start website on port 8082
./cold-backend --mode website --port 8082 &
WEBSITE_PID=$!
echo "Started website mode (PID: $WEBSITE_PID) on port 8082"

# Wait for any process to exit
wait -n

# If any process exits, kill all others
echo "One of the servers exited, stopping all..."
kill $EMPLOYEE_PID $CUSTOMER_PID $WEBSITE_PID 2>/dev/null
wait

echo "All servers stopped"
exit 1
