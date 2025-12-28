#!/bin/bash
# Deploy script with automatic image cleanup
set -e

VERSION="$1"
if [ -z "$VERSION" ]; then
    echo "Usage: ./deploy.sh v1.5.xx"
    exit 1
fi

KEEP_VERSIONS=5
IMAGE_NAME="lakshyajaat/cold-backend"

echo "=== Building $VERSION ==="
cd /home/lakshya/jupyter-/cold/cold-backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server
docker build -t $IMAGE_NAME:$VERSION .

echo "=== Saving image ==="
TAR_FILE="/tmp/cold-backend-$VERSION.tar"
docker save $IMAGE_NAME:$VERSION -o $TAR_FILE

echo "=== Deploying to all K3s nodes ==="
for NODE in 192.168.15.110 192.168.15.111 192.168.15.112; do
    echo "Node $NODE..."
    scp $TAR_FILE root@$NODE:/tmp/
    ssh root@$NODE "k3s ctr -n k8s.io images import /tmp/cold-backend-$VERSION.tar && rm /tmp/cold-backend-$VERSION.tar"
done

rm -f $TAR_FILE

echo "=== Updating Kubernetes deployments ==="
kubectl set image deployment/cold-backend-employee cold-backend=$IMAGE_NAME:$VERSION -n default
kubectl set image deployment/cold-backend-customer cold-backend=$IMAGE_NAME:$VERSION -n default

echo "=== Waiting for rollout ==="
kubectl rollout status deployment/cold-backend-employee -n default --timeout=120s
kubectl rollout status deployment/cold-backend-customer -n default --timeout=120s

echo "=== Cleaning old images (keeping last $KEEP_VERSIONS) ==="
for NODE in 192.168.15.110 192.168.15.111 192.168.15.112; do
    echo "Cleaning $NODE..."
    ssh root@$NODE "/usr/local/bin/k3s-image-gc.sh" 2>/dev/null || true
done

echo "=== Deployed $VERSION successfully! ==="
