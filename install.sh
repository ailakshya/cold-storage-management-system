#!/bin/bash
# ============================================
# COLD STORAGE - ONE-CLICK DISASTER RECOVERY
# ============================================
# Usage: ./install.sh
# This script sets up everything automatically
#
# Supported distros:
#   - Ubuntu/Debian (apt)
#   - Arch Linux (pacman)
#   - Fedora/RHEL/CentOS (dnf/yum)
#   - openSUSE (zypper)

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

# ============================================
# DETECT PACKAGE MANAGER
# ============================================
detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        DISTRO=$ID
    elif [ -f /etc/arch-release ]; then
        DISTRO="arch"
    elif [ -f /etc/debian_version ]; then
        DISTRO="debian"
    elif [ -f /etc/redhat-release ]; then
        DISTRO="rhel"
    else
        DISTRO="unknown"
    fi
    echo $DISTRO
}

DISTRO=$(detect_distro)
echo -e "${GREEN}Detected distro:${NC} $DISTRO"

# ============================================
# PACKAGE MANAGER FUNCTIONS
# ============================================
install_postgresql() {
    case $DISTRO in
        ubuntu|debian|linuxmint|pop)
            echo -e "${GREEN}[1/5]${NC} Updating system (apt)..."
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
            ;;
        arch|manjaro|endeavouros|garuda)
            echo -e "${GREEN}[1/5]${NC} Updating system (pacman)..."
            pacman -Sy --noconfirm
            echo -e "${GREEN}[2/5]${NC} Installing PostgreSQL..."
            if command -v psql &> /dev/null; then
                echo "  PostgreSQL already installed"
            else
                pacman -S postgresql --noconfirm
                # Initialize PostgreSQL data directory on Arch
                if [ ! -d /var/lib/postgres/data ] || [ -z "$(ls -A /var/lib/postgres/data 2>/dev/null)" ]; then
                    echo "  Initializing PostgreSQL database..."
                    mkdir -p /var/lib/postgres/data
                    chown -R postgres:postgres /var/lib/postgres
                    su - postgres -c "initdb -D /var/lib/postgres/data"
                fi
                systemctl enable postgresql
                systemctl start postgresql
                echo "  PostgreSQL installed and started"
            fi
            ;;
        fedora|rhel|centos|rocky|almalinux)
            echo -e "${GREEN}[1/5]${NC} Updating system (dnf)..."
            dnf check-update -q || true
            echo -e "${GREEN}[2/5]${NC} Installing PostgreSQL..."
            if command -v psql &> /dev/null; then
                echo "  PostgreSQL already installed"
            else
                dnf install -y postgresql-server postgresql-contrib -q
                # Initialize PostgreSQL on RHEL-based
                if [ ! -f /var/lib/pgsql/data/PG_VERSION ]; then
                    postgresql-setup --initdb
                fi
                systemctl enable postgresql
                systemctl start postgresql
                echo "  PostgreSQL installed and started"
            fi
            ;;
        opensuse*|sles)
            echo -e "${GREEN}[1/5]${NC} Updating system (zypper)..."
            zypper refresh -q
            echo -e "${GREEN}[2/5]${NC} Installing PostgreSQL..."
            if command -v psql &> /dev/null; then
                echo "  PostgreSQL already installed"
            else
                zypper install -y postgresql-server postgresql -q
                systemctl enable postgresql
                systemctl start postgresql
                echo "  PostgreSQL installed and started"
            fi
            ;;
        *)
            echo -e "${RED}Unsupported distribution: $DISTRO${NC}"
            echo "Supported: Ubuntu, Debian, Arch, Manjaro, Fedora, RHEL, CentOS, openSUSE"
            echo ""
            echo "Manual installation required:"
            echo "  1. Install PostgreSQL"
            echo "  2. Create database: createdb cold_db"
            echo "  3. Run: ./server"
            exit 1
            ;;
    esac
}

# ============================================
# INSTALL POSTGRESQL
# ============================================
install_postgresql

# Wait for PostgreSQL to be ready
echo "  Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if sudo -u postgres psql -c "SELECT 1" &>/dev/null; then
        break
    fi
    sleep 1
done

# ============================================
# CREATE DATABASE
# ============================================
echo -e "${GREEN}[3/5]${NC} Creating database..."
if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw cold_db; then
    echo "  Database 'cold_db' already exists"
else
    sudo -u postgres psql -c "CREATE DATABASE cold_db;" > /dev/null
    echo "  Database 'cold_db' created"
fi

# ============================================
# SETUP SYSTEMD SERVICE
# ============================================
echo -e "${GREEN}[4/5]${NC} Setting up service..."
# Copy binary to /opt
mkdir -p /opt/cold-backend
cp "$BINARY" /opt/cold-backend/server
chmod +x /opt/cold-backend/server

# Copy templates if they exist
if [ -d "$SCRIPT_DIR/templates" ]; then
    cp -r "$SCRIPT_DIR/templates" /opt/cold-backend/
    echo "  Templates copied"
fi

# Copy static files if they exist
if [ -d "$SCRIPT_DIR/static" ]; then
    cp -r "$SCRIPT_DIR/static" /opt/cold-backend/
    echo "  Static files copied"
fi

# Copy migrations if they exist
if [ -d "$SCRIPT_DIR/migrations" ]; then
    cp -r "$SCRIPT_DIR/migrations" /opt/cold-backend/
    echo "  Migrations copied"
fi

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

# Environment
Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=postgres"
Environment="DB_NAME=cold_db"

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable cold-backend

# ============================================
# START SERVER
# ============================================
echo -e "${GREEN}[5/5]${NC} Starting server..."
systemctl start cold-backend

# Wait for server to start
sleep 10

# Check if running
if systemctl is-active --quiet cold-backend; then
    IP_ADDR=$(hostname -I | awk '{print $1}')
    echo ""
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                    INSTALLATION COMPLETE                    ║"
    echo "╠════════════════════════════════════════════════════════════╣"
    echo "║                                                            ║"
    echo "║  Server running on: http://$IP_ADDR:8080"
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
