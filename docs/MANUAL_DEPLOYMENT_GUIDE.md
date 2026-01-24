# Manual Deployment Guide for Cold Storage Management System

## Current Situation
- GitHub Actions workflow cancelled (Run #597)
- Self-hosted runner needed at: root@192.168.15.96
- Network connectivity intermittent from current machine

## Runner Installation (On Proxmox Host 192.168.15.96)

### Quick Installation Script

**Fresh GitHub Token (expires ~1 hour):**
```
AQ3BUNPQEF4C4NAYS2W6LMLJN7RRK
```

**Option 1: One-Line Install**
SSH into the Proxmox host and run:

```bash
ssh root@192.168.15.96
```

Then execute:

```bash
apt-get install -y sudo curl git jq && \
useradd -m -s /bin/bash runner 2>/dev/null || true && \
echo "runner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/runner && \
cd /home/runner && mkdir -p actions-runner && cd actions-runner && \
[ ! -f "config.sh" ] && curl -sL https://github.com/actions/runner/releases/download/v2.331.0/actions-runner-linux-x64-2.331.0.tar.gz | tar xz && \
chown -R runner:runner /home/runner/actions-runner && \
sudo -u runner ./config.sh \
  --url https://github.com/lakshyajaat/cold-storage-management-system \
  --token AQ3BUNPQEF4C4NAYS2W6LMLJN7RRK \
  --name cold-storage-runner-proxmox \
  --labels self-hosted,linux,proxmox \
  --unattended \
  --replace && \
./svc.sh install runner && \
./svc.sh start && \
./svc.sh status
```

**Option 2: Step-by-Step Install**

```bash
# 1. Install dependencies
apt-get update
apt-get install -y sudo curl git jq

# 2. Create runner user
useradd -m -s /bin/bash runner
echo "runner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/runner

# 3. Download runner
cd /home/runner
mkdir -p actions-runner
cd actions-runner
curl -L https://github.com/actions/runner/releases/download/v2.331.0/actions-runner-linux-x64-2.331.0.tar.gz | tar xz
chown -R runner:runner /home/runner/actions-runner

# 4. Configure runner
sudo -u runner ./config.sh \
  --url https://github.com/lakshyajaat/cold-storage-management-system \
  --token AQ3BUNPQEF4C4NAYS2W6LMLJN7RRK \
  --name cold-storage-runner-proxmox \
  --labels self-hosted,linux,proxmox \
  --unattended \
  --replace

# 5. Install and start service
./svc.sh install runner
./svc.sh start

# 6. Verify
./svc.sh status
systemctl status actions.runner.lakshyajaat-cold-storage-management-system.cold-storage-runner-proxmox
```

### Verify Runner is Active

1. Check locally on the host:
```bash
journalctl -u actions.runner.* -f
```

2. Check on GitHub:
https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners

You should see "cold-storage-runner-proxmox" with status "Idle"

---

## Manual Deployment (Alternative if Runner Fails)

If you need to deploy immediately without waiting for the runner:

### Prerequisites
- SSH access to K3s nodes (192.168.15.110, 192.168.15.111, 192.168.15.112)
- kubectl configured to access your K3s cluster
- Docker installed locally or on a build machine

### Quick Deploy Script

```bash
#!/bin/bash
set -e

# Build configuration
VERSION="v1.5.$(date +%s)"
IMAGE_NAME="lakshyajaat/cold-backend"

echo "=== Building version: $VERSION ==="

# 1. Build Go binary
echo "[1/6] Building Go binary..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd/server

# 2. Build Docker image
echo "[2/6] Building Docker image..."
DOCKER_BUILDKIT=1 docker build \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  -f Dockerfile.ci \
  -t ${IMAGE_NAME}:${VERSION} .

# 3. Save and compress image
echo "[3/6] Saving Docker image..."
docker save ${IMAGE_NAME}:${VERSION} | gzip > /tmp/cold-backend-${VERSION}.tar.gz
SIZE=$(du -h /tmp/cold-backend-${VERSION}.tar.gz | cut -f1)
echo "Image saved: $SIZE"

# 4. Deploy to K3s nodes
echo "[4/6] Deploying to K3s nodes..."
K3S_NODES="192.168.15.110 192.168.15.111 192.168.15.112"

for NODE in ${K3S_NODES}; do
  echo "  Deploying to $NODE..."
  scp /tmp/cold-backend-${VERSION}.tar.gz root@${NODE}:/tmp/ && \
  ssh root@${NODE} "gunzip -c /tmp/cold-backend-${VERSION}.tar.gz | k3s ctr -n k8s.io images import - && rm -f /tmp/cold-backend-${VERSION}.tar.gz" &
done

# Wait for all deployments
wait
echo "✓ All nodes updated"

# 5. Update Kubernetes deployments
echo "[5/6] Updating Kubernetes deployments..."
kubectl set image deployment/cold-backend-employee \
  cold-backend=${IMAGE_NAME}:${VERSION} -n default

kubectl set image deployment/cold-backend-customer \
  cold-backend=${IMAGE_NAME}:${VERSION} -n default

echo "✓ Deployments updated"

# 6. Wait for rollout
echo "[6/6] Waiting for rollout..."
kubectl rollout status deployment/cold-backend-employee -n default --timeout=180s
kubectl rollout status deployment/cold-backend-customer -n default --timeout=180s

# Verify
echo ""
echo "=== Deployment Complete ==="
kubectl get pods -l app=cold-backend -n default

# Cleanup
rm -f /tmp/cold-backend-${VERSION}.tar.gz server

echo ""
echo "✅ Deployment successful: $VERSION"
```

Save this as `manual-deploy.sh` and run:

```bash
chmod +x manual-deploy.sh
./manual-deploy.sh
```

---

## Troubleshooting

### Get New GitHub Token
If the token expires, generate a new one:

**Via GitHub Web:**
1. Visit: https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners/new
2. Copy the token from the configuration command

**Via GitHub CLI:**
```bash
gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/lakshyajaat/cold-storage-management-system/actions/runners/registration-token \
  | jq -r '.token'
```

### Check Runner Status
```bash
# On Proxmox host
systemctl status actions.runner.lakshyajaat-cold-storage-management-system.cold-storage-runner-proxmox
journalctl -u actions.runner.* -f

# On GitHub
# Visit: https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners
```

### Restart Runner
```bash
cd /home/runner/actions-runner
./svc.sh restart
```

### Check Deployment Status
```bash
kubectl get pods -n default
kubectl describe pod <pod-name> -n default
kubectl logs <pod-name> -n default
```

---

## Next Steps After Runner Installation

1. **Verify runner is online** on GitHub
2. **Trigger a new deployment:**
   - Push to main branch, OR
   - Go to: https://github.com/lakshyajaat/cold-storage-management-system/actions/workflows/deploy.yml
   - Click "Run workflow"
   - Select branch "main"
   - Click "Run workflow" button

3. **Monitor the deployment:**
   - Watch the workflow run at: https://github.com/lakshyajaat/cold-storage-management-system/actions
   - Check pods: `kubectl get pods -n default -w`

---

## Files and Resources

- Full setup guide: `./docs/GITHUB_RUNNER_SETUP.md`
- Setup scripts: `/tmp/setup-github-runner.sh`, `/tmp/quick-runner-setup.sh`
- Token helper: `/tmp/get-github-token.sh`
- GitHub-hosted workflow (alternative): `.github/workflows/deploy-github-hosted.yml`

**GitHub Actions Runners Page:**
https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners

**Actions Workflow Runs:**
https://github.com/lakshyajaat/cold-storage-management-system/actions
