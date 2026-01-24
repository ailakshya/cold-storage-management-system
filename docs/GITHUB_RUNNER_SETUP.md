# GitHub Actions Runner Setup Guide for Proxmox
# Manual Installation Steps

## Prerequisites
- Proxmox host:  192.168.15.96
- Repository: lakshyajaat/cold-storage-management-system
- Registration Token: AQ3BUNPLTWNLYKNOVM625UDJN7QLA (valid for 1 hour)

## Installation Steps

### 1. SSH into the Proxmox Host
```bash
ssh root@192.168.15.96
```

### 2. Fix APT Repositories (to avoid 401 errors)
```bash
# Backup enterprise repos
mv /etc/apt/sources.list.d/*.list /tmp/ 2>/dev/null || true

# Add open-source repo
echo 'deb http://download.proxmox.com/debian/pve trixie pve-no-subscription' > /etc/apt/sources.list.d/pve-no-subscription.list

# Update package list
apt-get update
```

### 3. Install Required Packages
```bash
apt-get install -y curl git jq libicu72
```

### 4. Create Runner User
```bash
useradd -m -s /bin/bash runner
echo "runner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/runner
```

### 5. Download and Install GitHub Actions Runner
```bash
cd /home/runner
sudo -u runner mkdir -p actions-runner
cd actions-runner

# Download runner (version 2.331.0)
sudo -u runner curl -o actions-runner-linux-x64-2.331.0.tar.gz \
  -L https://github.com/actions/runner/releases/download/v2.331.0/actions-runner-linux-x64-2.331.0.tar.gz

# Extract
sudo -u runner tar xzf actions-runner-linux-x64-2.331.0.tar.gz
rm actions-runner-linux-x64-2.331.0.tar.gz
```

### 6. Configure the Runner
```bash
cd /home/runner/actions-runner

# Configure with GitHub
sudo -u runner ./config.sh \
  --url https://github.com/lakshyajaat/cold-storage-management-system \
  --token AQ3BUNPLTWNLYKNOVM625UDJN7QLA \
  --name cold-storage-runner-proxmox \
  --labels self-hosted,linux,proxmox \
  --unattended \
  --replace
```

### 7. Install as System Service
```bash
cd /home/runner/actions-runner
./svc.sh install runner
./svc.sh start
```

### 8. Verify Installation
```bash
# Check service status
./svc.sh status

# Or use systemctl
systemctl status actions.runner.lakshyajaat-cold-storage-management-system.cold-storage-runner-proxmox

# View logs
journalctl -u actions.runner.lakshyajaat-cold-storage-management-system.cold-storage-runner-proxmox -f
```

## Troubleshooting

### If Token Expired
Get a new token from:
https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners/new

Or use GitHub CLI:
```bash
gh api --method POST \
  -H "Accept: application/vnd.github+json" \
  /repos/lakshyajaat/cold-storage-management-system/actions/runners/registration-token \
  | jq -r '.token'
```

### Check Runner Status on GitHub
Visit: https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners

### Restart Runner
```bash
cd /home/runner/actions-runner
./svc.sh stop
./svc.sh start
```

### Remove and Reconfigure Runner
```bash
cd /home/runner/actions-runner
./svc.sh stop
./svc.sh uninstall
sudo -u runner ./config.sh remove --token <NEW_TOKEN>
# Then repeat steps 6-7
```

## Quick One-Liner Install (if network is stable)

```bash
ssh root@192.168.15.96 'bash -s' << 'ENDSSH'
set -e
apt-get update 2>&1 | grep -v "enterprise\|401" || true
apt-get install -y curl git jq libicu72
useradd -m -s /bin/bash runner 2>/dev/null || true
echo "runner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/runner
cd /home/runner
sudo -u runner mkdir -p actions-runner
cd actions-runner
if [ ! -f "config.sh" ]; then
  sudo -u runner curl -sL https://github.com/actions/runner/releases/download/v2.331.0/actions-runner-linux-x64-2.331.0.tar.gz | sudo -u runner tar xz
fi
sudo -u runner ./config.sh --url https://github.com/lakshyajaat/cold-storage-management-system --token AQ3BUNPLTWNLYKNOVM625UDJN7QLA --name cold-storage-runner-proxmox --labels self-hosted,linux,proxmox --unattended --replace
./svc.sh install runner
./svc.sh start
./svc.sh status
ENDSSH
```

## Post-Installation

Once the runner is installed and running:
1. Check GitHub: https://github.com/lakshyajaat/cold-storage-management-system/settings/actions/runners
2. You should see "cold-storage-runner-proxmox" with status "Idle" or "Active"
3. Trigger a deployment by pushing to main branch or manually triggering the workflow

## Service Management Commands

```bash
# Status
sudo systemctl status actions.runner.*

# Logs
sudo journalctl -u actions.runner.* -f

# Restart
cd /home/runner/actions-runner && sudo ./svc.sh restart

# Stop
cd /home/runner/actions-runner && sudo ./svc.sh stop

# Start
cd /home/runner/actions-runner && sudo ./svc.sh start
```
