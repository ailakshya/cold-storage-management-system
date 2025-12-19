# CI/CD Pipeline Documentation

## Overview

This project uses GitHub Actions for CI/CD with manual deployment trigger. The pipeline builds a Docker image and deploys it to a 5-node K3s cluster.

## Architecture

```
GitHub Actions (self-hosted runner)
         |
         v
    Build Stage
    - Go binary compilation
    - Docker image build
         |
         v
    Deploy Stage
    - SCP image to all K3s nodes
    - Import to containerd
    - Update Kubernetes deployments
         |
         v
    K3s Cluster (5 nodes)
    - 192.168.15.110 (master)
    - 192.168.15.111-114 (workers)
```

## Prerequisites

### 1. Self-Hosted Runner Setup

Since the K3s cluster is on a private network (192.168.15.x), you need a self-hosted GitHub Actions runner on the same network.

#### Install Runner

On a machine with access to the K3s cluster:

```bash
# Create runner directory
mkdir actions-runner && cd actions-runner

# Download latest runner
curl -o actions-runner-linux-x64.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-x64-2.311.0.tar.gz

# Extract
tar xzf actions-runner-linux-x64.tar.gz

# Configure (get token from GitHub: Settings > Actions > Runners > New self-hosted runner)
./config.sh --url https://github.com/YOUR_ORG/cold-backend --token YOUR_TOKEN

# Install as service
sudo ./svc.sh install
sudo ./svc.sh start
```

### 2. SSH Key Setup

The runner needs passwordless SSH access to all K3s nodes:

```bash
# Generate SSH key (if not exists)
ssh-keygen -t ed25519 -C "github-actions"

# Copy to all K3s nodes
for NODE in 192.168.15.110 192.168.15.111 192.168.15.112 192.168.15.113 192.168.15.114; do
    ssh-copy-id root@$NODE
done
```

### 3. Kubectl Configuration

The runner needs kubectl configured:

```bash
# Copy kubeconfig from master
scp root@192.168.15.110:/etc/rancher/k3s/k3s.yaml ~/.kube/config

# Update server address
sed -i 's/127.0.0.1/192.168.15.110/g' ~/.kube/config
```

### 4. Docker Setup

Docker must be installed on the runner machine:

```bash
sudo apt-get update
sudo apt-get install -y docker.io
sudo usermod -aG docker $USER
```

## Usage

### Manual Deployment

1. Go to **Actions** tab in GitHub
2. Select **Deploy to K3s** workflow
3. Click **Run workflow**
4. Enter version tag (e.g., `v1.5.40`)
5. Click **Run workflow**

### Deployment Process

The workflow performs these steps:

1. **Checkout** - Clone repository
2. **Set up Go** - Install Go 1.23
3. **Build Go binary** - Compile server binary
4. **Build Docker image** - Create container image
5. **Save Docker image** - Export as tar file
6. **Deploy to K3s nodes** - SCP and import on all nodes
7. **Update Kubernetes deployments** - Rolling update
8. **Cleanup** - Remove temporary files
9. **Verify deployment** - Show pod status

### Version Naming Convention

Use semantic versioning with `v` prefix:
- `v1.5.40` - Feature version
- `v1.5.41` - Patch/fix version
- `v1.6.0` - Minor version bump

## Workflow File

Location: `.github/workflows/deploy.yml`

Key configuration:

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Version tag (e.g., v1.5.40)'
        required: true
        type: string

env:
  IMAGE_NAME: lakshyajaat/cold-backend
  K3S_NODES: "192.168.15.110 192.168.15.111 192.168.15.112 192.168.15.113 192.168.15.114"
```

## Troubleshooting

### Common Issues

#### 1. SSH Connection Failed

```
Permission denied (publickey)
```

**Fix:** Ensure SSH keys are properly set up:
```bash
ssh-copy-id root@192.168.15.110
```

#### 2. kubectl Not Found

```
kubectl: command not found
```

**Fix:** Install kubectl on runner:
```bash
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

#### 3. Docker Permission Denied

```
Got permission denied while trying to connect to the Docker daemon
```

**Fix:** Add user to docker group and restart:
```bash
sudo usermod -aG docker $USER
newgrp docker
```

#### 4. Deployment Timeout

```
error: timed out waiting for the condition
```

**Fix:** Check pod status and logs:
```bash
kubectl get pods -l app=cold-backend
kubectl logs -l app=cold-backend --tail=50
```

### Manual Rollback

If deployment fails, rollback to previous version:

```bash
# Rollback to previous revision
kubectl rollout undo deployment/cold-backend-employee
kubectl rollout undo deployment/cold-backend-customer

# Or deploy specific version
kubectl set image deployment/cold-backend-employee cold-backend=lakshyajaat/cold-backend:v1.5.39
kubectl set image deployment/cold-backend-customer cold-backend=lakshyajaat/cold-backend:v1.5.39
```

## Local Development Deployment

For quick deployments without GitHub Actions:

```bash
# Build and deploy script
VERSION="v1.5.40"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server
docker build -t lakshyajaat/cold-backend:$VERSION .
docker save lakshyajaat/cold-backend:$VERSION -o /tmp/cold-backend-$VERSION.tar

for NODE in 192.168.15.110 192.168.15.111 192.168.15.112 192.168.15.113 192.168.15.114; do
    scp /tmp/cold-backend-$VERSION.tar root@$NODE:/tmp/
    ssh root@$NODE "k3s ctr -n k8s.io images import /tmp/cold-backend-$VERSION.tar && rm /tmp/cold-backend-$VERSION.tar"
done

kubectl set image deployment/cold-backend-employee cold-backend=lakshyajaat/cold-backend:$VERSION -n default
kubectl set image deployment/cold-backend-customer cold-backend=lakshyajaat/cold-backend:$VERSION -n default
```

## Security Notes

- SSH keys should be kept secure on the runner machine
- Never commit SSH keys or kubeconfig to the repository
- Runner should be on a trusted network with firewall protection
- Consider using kubectl port-forward instead of exposing services
