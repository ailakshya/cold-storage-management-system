#!/bin/bash
set -e
export DEBIAN_FRONTEND=noninteractive

# Configuration
APP_DIR="/opt/cold-backend"
ENV_FILE="$APP_DIR/.env"
DB_PASS="Lak992723/" 

echo "========================================================"
echo " COLD STORAGE - FULL STACK DEPLOYER"
echo "========================================================"
echo "Components: PostgreSQL, Redis, K3s, Go App"
echo "Target OS: Ubuntu 22.04 LTS"
echo "========================================================"

if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

# 1. Update System
echo "[1/7] Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

# 2. Install Dependencies (Go, Redis, Postgres)
echo "[2/7] Installing Dependencies..."
apt-get install -y postgresql postgresql-contrib redis-server curl git make gcc openssl

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "Installing Golang..."
    curl -OL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
    export PATH=$PATH:/usr/local/go/bin
    rm go1.22.4.linux-amd64.tar.gz
fi

# 3. Configure PostgreSQL
echo "[3/7] Configuring PostgreSQL..."
systemctl start postgresql
systemctl enable postgresql

# Allow remote connections
for f in /etc/postgresql/*/main/postgresql.conf; do sed -i "s/#listen_addresses = 'localhost'/listen_addresses = '*'/" "$f"; done
for f in /etc/postgresql/*/main/pg_hba.conf; do echo "host all all 0.0.0.0/0 md5" >> "$f"; done
systemctl restart postgresql
sleep 5

# Set password for 'postgres' user
sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD '$DB_PASS';"

# Create DB if not exists
if ! sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw cold_db; then
    sudo -u postgres psql -c "CREATE DATABASE cold_db;"
    echo "Database 'cold_db' created."
fi

# 4. Install K3s (Single Node)
echo "[4/7] Installing K3s..."
if ! command -v k3s &> /dev/null; then
    curl -sfL https://get.k3s.io | sh -
    sleep 20 # Wait for startup
else
    echo "K3s already installed."
fi
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# 5. Build Container Image
echo "[5/7] Building Container Image..."
if [ ! -f "Dockerfile" ]; then
    echo "Error: Dockerfile not found."
    exit 1
fi

# Install Docker to build locally
if ! command -v docker &> /dev/null; then
    echo "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
fi

docker build -t cold-backend:latest .
docker save cold-backend:latest -o cold-backend.tar

# 6. Import to K3s
echo "[6/7] Importing to K3s..."
k3s ctr images import cold-backend.tar
rm cold-backend.tar

# 7. Deploy to K3s
echo "[7/7] Deploying Manifests..."
# Apply Apps
k3s kubectl apply -f k8s/production-apps.yaml

# Apply Cloudflare Tunnel (if present)
if [ -d "k8s/cloudflare" ]; then
    echo "Deploying Cloudflare Tunnel..."
    k3s kubectl apply -f k8s/cloudflare/
fi

echo "========================================================"
echo "          K3S DEPLOYMENT COMPLETE                       "
echo "========================================================"
echo "App accessible at: http://$(hostname -I | awk '{print $1}'):30080"
echo "Check pods: kubectl get pods"
