#!/bin/bash
# ============================================
# COLD STORAGE - ONE-CLICK DISASTER RECOVERY
# ============================================
# Usage: ./install.sh
# This script sets up everything automatically

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║     COLD STORAGE - DISASTER RECOVERY INSTALLER             ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory (where binary should be)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$SCRIPT_DIR/server"

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run as root: sudo ./install.sh${NC}"
    exit 1
fi

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo -e "${RED}Error: server binary not found in $SCRIPT_DIR${NC}"
    echo "Make sure 'server' binary is in the same folder as this script"
    exit 1
fi

echo -e "${GREEN}[1/5]${NC} Updating system..."
apt update -qq

echo -e "${GREEN}[2/5]${NC} Installing PostgreSQL..."
if command -v psql &> /dev/null; then
    echo "  PostgreSQL already installed"
else
    apt install -y postgresql postgresql-contrib -qq
    systemctl enable postgresql
    systemctl start postgresql
    echo "  PostgreSQL installed and started"
fi

echo -e "${GREEN}[3/5]${NC} Creating database..."
# Check if database exists
if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw cold_db; then
    echo "  Database 'cold_db' already exists"
else
    sudo -u postgres psql -c "CREATE DATABASE cold_db;" > /dev/null
    echo "  Database 'cold_db' created"
fi

echo -e "${GREEN}[4/5]${NC} Setting up service..."
# Copy binary to /opt
mkdir -p /opt/cold-backend
cp "$BINARY" /opt/cold-backend/server
chmod +x /opt/cold-backend/server

# Create systemd service
cat > /etc/systemd/system/cold-backend.service << 'EOF'
[Unit]
Description=Cold Storage Backend
After=postgresql.service network.target
Wants=postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/cold-backend
ExecStart=/opt/cold-backend/server
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable cold-backend

echo -e "${GREEN}[5/5]${NC} Starting server..."
systemctl start cold-backend

# Wait for server to start
sleep 10

# Check if running
if systemctl is-active --quiet cold-backend; then
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                    INSTALLATION COMPLETE                    ║"
    echo "╠════════════════════════════════════════════════════════════╣"
    echo "║                                                            ║"
    echo "║  Server running on: http://$(hostname -I | awk '{print $1}'):8080            ║"
    echo "║                                                            ║"
    echo "║  Commands:                                                 ║"
    echo "║    View logs:    journalctl -u cold-backend -f            ║"
    echo "║    Stop:         systemctl stop cold-backend              ║"
    echo "║    Start:        systemctl start cold-backend             ║"
    echo "║    Status:       systemctl status cold-backend            ║"
    echo "║                                                            ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""

    # Show recent logs
    echo "Recent logs:"
    journalctl -u cold-backend -n 20 --no-pager
else
    echo -e "${RED}Server failed to start. Check logs:${NC}"
    journalctl -u cold-backend -n 50 --no-pager
    exit 1
fi
