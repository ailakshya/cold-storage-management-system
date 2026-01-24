#!/bin/bash
set -e

# Manual Deployment Script for Cold Storage Management System
# This script deploys the application without GitHub Actions

# Configuration
VERSION="v1.5.manual-$(date +%s)"
IMAGE_NAME="lakshyajaat/cold-backend"
K3S_NODES="192.168.15.110 192.168.15.111 192.168.15.112"

echo "════════════════════════════════════════════════════════════════"
echo "  Cold Storage Management System - Manual Deployment"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Version: $VERSION"
echo "Image: $IMAGE_NAME:$VERSION"
echo "K3s Nodes: $K3S_NODES"
echo ""

# 1. Build Go binary
echo "[1/6] Building Go binary..."
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server
echo "✓ Go binary built ($(du -h server | cut -f1))"

# 2. Build Docker image
echo ""
echo "[2/6] Building Docker image..."
DOCKER_BUILDKIT=1 docker build \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  -f Dockerfile.ci \
  -t ${IMAGE_NAME}:${VERSION} .
echo "✓ Docker image built: ${IMAGE_NAME}:${VERSION}"

# 3. Save and compress image
echo ""
echo "[3/6] Saving and compressing Docker image..."
docker save ${IMAGE_NAME}:${VERSION} | gzip > /tmp/cold-backend-${VERSION}.tar.gz
SIZE=$(du -h /tmp/cold-backend-${VERSION}.tar.gz | cut -f1)
echo "✓ Image saved: $SIZE"

# 4. Deploy to K3s nodes
echo ""
echo "[4/6] Deploying to K3s nodes..."
FAILED=0
TOTAL=0

deploy_to_node() {
    local NODE=$1
    local MAX_RETRIES=3
    local RETRY=0
    
    while [ $RETRY -lt $MAX_RETRIES ]; do
        echo "  → Deploying to $NODE (attempt $((RETRY+1))/$MAX_RETRIES)..."
        
        if scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 /tmp/cold-backend-${VERSION}.tar.gz root@${NODE}:/tmp/ 2>/dev/null && \
           ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 root@${NODE} \
             "gunzip -c /tmp/cold-backend-${VERSION}.tar.gz | k3s ctr -n k8s.io images import - && rm -f /tmp/cold-backend-${VERSION}.tar.gz" 2>/dev/null; then
            echo "  ✓ $NODE: success"
            return 0
        fi
        
        RETRY=$((RETRY + 1))
        [ $RETRY -lt $MAX_RETRIES ] && sleep 3
    done
    
    echo "  ✗ $NODE: failed after $MAX_RETRIES attempts"
    return 1
}

# Deploy to all nodes in parallel
for NODE in ${K3S_NODES}; do
    TOTAL=$((TOTAL + 1))
    deploy_to_node ${NODE} &
done

# Wait for all deployments
for job in $(jobs -p); do
    wait $job || FAILED=$((FAILED + 1))
done

if [ $FAILED -gt 0 ]; then
    echo ""
    echo "⚠ Warning: $FAILED/$TOTAL nodes failed"
    if [ $FAILED -eq $TOTAL ]; then
        echo "✗ All nodes failed. Aborting deployment."
        exit 1
    fi
    echo "Continuing with successful nodes..."
else
    echo ""
    echo "✓ All nodes updated successfully"
fi

# 5. Update Kubernetes deployments
echo ""
echo "[5/6] Updating Kubernetes deployments..."

kubectl set image deployment/cold-backend-employee \
  cold-backend=${IMAGE_NAME}:${VERSION} -n default

kubectl set image deployment/cold-backend-customer \
  cold-backend=${IMAGE_NAME}:${VERSION} -n default

echo "✓ Deployments updated to ${VERSION}"

# 6. Wait for rollout
echo ""
echo "[6/6] Waiting for rollout (up to 3 minutes each)..."

if ! kubectl rollout status deployment/cold-backend-employee -n default --timeout=180s; then
    echo "⚠ Employee deployment timeout - checking status..."
fi

if ! kubectl rollout status deployment/cold-backend-customer -n default --timeout=180s; then
    echo "⚠ Customer deployment timeout - checking status..."
fi

# Verify deployment
echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  Deployment Status"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check ready pods
READY_PODS=$(kubectl get pods -l app=cold-backend -n default \
  -o jsonpath='{range .items[*]}{.spec.containers[0].image}{" "}{.status.containerStatuses[0].ready}{"\n"}{end}' | \
  grep "${VERSION}" | grep -c "true" || echo "0")

TOTAL_PODS=$(kubectl get pods -l app=cold-backend -n default \
  -o jsonpath='{.items[*].spec.containers[0].image}' | \
  grep -o "${VERSION}" | wc -w || echo "0")

echo "Ready Pods: ${READY_PODS}/${TOTAL_PODS} on ${VERSION}"
echo ""

# Show pod details
kubectl get pods -l app=cold-backend -n default \
  -o custom-columns="POD:.metadata.name,IMAGE:.spec.containers[0].image,READY:.status.containerStatuses[0].ready,STATUS:.status.phase" \
  --no-headers

echo ""

# Health check
if [ "$READY_PODS" -ge 2 ]; then
    echo "✅ Deployment successful!"
    echo ""
    echo "Deployed version: $VERSION"
    echo "Ready pods: $READY_PODS/$TOTAL_PODS"
else
    echo "❌ Deployment may have issues"
    echo ""
    echo "Less than 2 pods are ready. Please investigate:"
    echo ""
    echo "  kubectl describe pods -l app=cold-backend -n default"
    echo "  kubectl logs -l app=cold-backend -n default --tail=50"
    echo ""
    echo "To rollback:"
    echo "  kubectl rollout undo deployment/cold-backend-employee -n default"
    echo "  kubectl rollout undo deployment/cold-backend-customer -n default"
    
    # Cleanup
    rm -f /tmp/cold-backend-${VERSION}.tar.gz server
    
    exit 1
fi

# Cleanup
echo ""
echo "Cleaning up..."
rm -f /tmp/cold-backend-${VERSION}.tar.gz server
echo "✓ Cleanup complete"

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "  Deployment Complete"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "Version: $VERSION"
echo "Timestamp: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""
