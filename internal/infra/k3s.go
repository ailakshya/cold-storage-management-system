package infra

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// K3sService handles K3s installation and cluster management
type K3sService struct {
	ssh *SSHService
}

// NodeRole represents the role of a K3s node
type NodeRole string

const (
	RoleControlPlane NodeRole = "control-plane"
	RoleWorker       NodeRole = "worker"
)

// K3sInstallOptions holds options for K3s installation
type K3sInstallOptions struct {
	Host         string
	Port         int
	User         string
	PrivateKey   []byte
	Password     string
	Role         NodeRole
	ServerURL    string // For agent nodes: https://<server>:6443
	Token        string // K3s cluster token
	NodeName     string // Optional custom node name
	ExtraArgs    string // Extra args for k3s install
	InstallFlags string // Additional install script flags
}

// ProvisionStatus represents the current provisioning status
type ProvisionStatus struct {
	Step       string
	Progress   int // 0-100
	Message    string
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
}

// NewK3sService creates a new K3s service
func NewK3sService(ssh *SSHService) *K3sService {
	return &K3sService{ssh: ssh}
}

// CheckPrerequisites verifies the target node meets requirements
func (k *K3sService) CheckPrerequisites(ctx context.Context, opts *K3sInstallOptions) error {
	// Test SSH connectivity
	if err := k.ssh.TestConnection(ctx, opts.Host, opts.Port, opts.User, opts.PrivateKey, opts.Password); err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}

	// Check OS (Ubuntu/Debian preferred)
	osInfo, err := k.ssh.GetOSInfo(ctx, opts.Host, opts.Port, opts.User, opts.PrivateKey, opts.Password)
	if err != nil {
		return fmt.Errorf("failed to get OS info: %w", err)
	}

	// Verify supported OS
	if !strings.Contains(strings.ToLower(osInfo), "ubuntu") &&
		!strings.Contains(strings.ToLower(osInfo), "debian") &&
		!strings.Contains(strings.ToLower(osInfo), "rocky") &&
		!strings.Contains(strings.ToLower(osInfo), "centos") {
		return fmt.Errorf("unsupported OS: %s (supported: Ubuntu, Debian, Rocky, CentOS)", osInfo)
	}

	// Check if k3s is already installed
	result, _ := k.ssh.ExecuteWithConfig(ctx, opts.Host, opts.Port, opts.User, opts.PrivateKey, opts.Password, "which k3s")
	if strings.TrimSpace(result.Stdout) != "" {
		return fmt.Errorf("K3s is already installed on this node")
	}

	return nil
}

// InstallK3sAgent installs K3s as an agent node
func (k *K3sService) InstallK3sAgent(ctx context.Context, opts *K3sInstallOptions, statusChan chan<- ProvisionStatus) error {
	if opts.ServerURL == "" {
		return fmt.Errorf("server URL is required for agent installation")
	}
	if opts.Token == "" {
		return fmt.Errorf("cluster token is required for agent installation")
	}

	steps := []struct {
		name    string
		script  string
		timeout time.Duration
	}{
		{
			name: "Updating system packages",
			script: `
				export DEBIAN_FRONTEND=noninteractive
				apt-get update -qq || yum update -y -q
			`,
			timeout: 2 * time.Minute,
		},
		{
			name: "Installing dependencies",
			script: `
				apt-get install -y -qq curl wget net-tools || yum install -y -q curl wget net-tools
			`,
			timeout: 2 * time.Minute,
		},
		{
			name: "Setting hostname",
			script: fmt.Sprintf(`
				hostnamectl set-hostname %s || hostname %s
			`, k.generateHostname(opts), k.generateHostname(opts)),
			timeout: 30 * time.Second,
		},
		{
			name: "Installing K3s agent",
			script: fmt.Sprintf(`
				curl -sfL https://get.k3s.io | K3S_URL="%s" K3S_TOKEN="%s" sh -s - agent %s
			`, opts.ServerURL, opts.Token, opts.ExtraArgs),
			timeout: 5 * time.Minute,
		},
		{
			name: "Verifying K3s installation",
			script: `
				systemctl is-active k3s-agent || systemctl is-active k3s
			`,
			timeout: 30 * time.Second,
		},
		{
			name: "Installing node_exporter",
			script: k.nodeExporterInstallScript(),
			timeout: 2 * time.Minute,
		},
		{
			name: "Configuring firewall",
			script: k.firewallConfigScript(),
			timeout: 1 * time.Minute,
		},
	}

	return k.runProvisioningSteps(ctx, opts, steps, statusChan)
}

// InstallK3sServer installs K3s as a server (control plane) node
func (k *K3sService) InstallK3sServer(ctx context.Context, opts *K3sInstallOptions, statusChan chan<- ProvisionStatus) error {
	steps := []struct {
		name    string
		script  string
		timeout time.Duration
	}{
		{
			name: "Updating system packages",
			script: `
				export DEBIAN_FRONTEND=noninteractive
				apt-get update -qq || yum update -y -q
			`,
			timeout: 2 * time.Minute,
		},
		{
			name: "Installing dependencies",
			script: `
				apt-get install -y -qq curl wget net-tools || yum install -y -q curl wget net-tools
			`,
			timeout: 2 * time.Minute,
		},
		{
			name: "Setting hostname",
			script: fmt.Sprintf(`
				hostnamectl set-hostname %s || hostname %s
			`, k.generateHostname(opts), k.generateHostname(opts)),
			timeout: 30 * time.Second,
		},
		{
			name: "Installing K3s server",
			script: fmt.Sprintf(`
				curl -sfL https://get.k3s.io | sh -s - server %s
			`, opts.ExtraArgs),
			timeout: 5 * time.Minute,
		},
		{
			name: "Waiting for K3s to be ready",
			script: `
				for i in {1..60}; do
					k3s kubectl get nodes && break
					sleep 2
				done
			`,
			timeout: 2 * time.Minute,
		},
		{
			name: "Installing node_exporter",
			script: k.nodeExporterInstallScript(),
			timeout: 2 * time.Minute,
		},
		{
			name: "Configuring firewall",
			script: k.firewallConfigScript(),
			timeout: 1 * time.Minute,
		},
	}

	return k.runProvisioningSteps(ctx, opts, steps, statusChan)
}

// runProvisioningSteps executes provisioning steps sequentially
func (k *K3sService) runProvisioningSteps(ctx context.Context, opts *K3sInstallOptions, steps []struct {
	name    string
	script  string
	timeout time.Duration
}, statusChan chan<- ProvisionStatus) error {
	totalSteps := len(steps)
	startTime := time.Now()

	for i, step := range steps {
		progress := (i * 100) / totalSteps

		if statusChan != nil {
			statusChan <- ProvisionStatus{
				Step:      step.name,
				Progress:  progress,
				Message:   fmt.Sprintf("Step %d/%d: %s", i+1, totalSteps, step.name),
				StartedAt: startTime,
			}
		}

		stepCtx, cancel := context.WithTimeout(ctx, step.timeout)
		result, err := k.ssh.ExecuteScript(stepCtx, opts.Host, opts.Port, opts.User, opts.PrivateKey, opts.Password, step.script)
		cancel()

		if err != nil {
			errMsg := fmt.Sprintf("Step '%s' failed: %v", step.name, err)
			if statusChan != nil {
				statusChan <- ProvisionStatus{
					Step:       step.name,
					Progress:   progress,
					Error:      errMsg,
					StartedAt:  startTime,
					FinishedAt: time.Now(),
				}
			}
			return fmt.Errorf(errMsg)
		}

		if result.ExitCode != 0 {
			errMsg := fmt.Sprintf("Step '%s' failed with exit code %d: %s", step.name, result.ExitCode, result.Stderr)
			if statusChan != nil {
				statusChan <- ProvisionStatus{
					Step:       step.name,
					Progress:   progress,
					Error:      errMsg,
					StartedAt:  startTime,
					FinishedAt: time.Now(),
				}
			}
			return fmt.Errorf(errMsg)
		}
	}

	if statusChan != nil {
		statusChan <- ProvisionStatus{
			Step:       "Complete",
			Progress:   100,
			Message:    "Node provisioned successfully",
			StartedAt:  startTime,
			FinishedAt: time.Now(),
		}
	}

	return nil
}

// generateHostname creates a hostname from IP
func (k *K3sService) generateHostname(opts *K3sInstallOptions) string {
	if opts.NodeName != "" {
		return opts.NodeName
	}
	// Convert IP to hostname: 192.168.15.110 -> k3s-node-110
	parts := strings.Split(opts.Host, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("k3s-node-%s", parts[3])
	}
	return fmt.Sprintf("k3s-node-%s", strings.ReplaceAll(opts.Host, ".", "-"))
}

// nodeExporterInstallScript returns the script to install node_exporter
func (k *K3sService) nodeExporterInstallScript() string {
	return `
		# Check if node_exporter is already installed
		if systemctl is-active node_exporter >/dev/null 2>&1; then
			echo "node_exporter already running"
			exit 0
		fi

		# Download and install node_exporter
		cd /tmp
		wget -q https://github.com/prometheus/node_exporter/releases/download/v1.7.0/node_exporter-1.7.0.linux-amd64.tar.gz
		tar xzf node_exporter-1.7.0.linux-amd64.tar.gz
		mv node_exporter-1.7.0.linux-amd64/node_exporter /usr/local/bin/
		rm -rf node_exporter-1.7.0.linux-amd64*

		# Create systemd service
		cat > /etc/systemd/system/node_exporter.service << 'EOF'
[Unit]
Description=Node Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/node_exporter
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

		systemctl daemon-reload
		systemctl enable node_exporter
		systemctl start node_exporter
	`
}

// firewallConfigScript returns the script to configure firewall
func (k *K3sService) firewallConfigScript() string {
	return `
		# UFW (Ubuntu/Debian)
		if command -v ufw >/dev/null 2>&1; then
			ufw allow 22/tcp      # SSH
			ufw allow 6443/tcp    # K3s API
			ufw allow 10250/tcp   # Kubelet
			ufw allow 8472/udp    # Flannel VXLAN
			ufw allow 9100/tcp    # Node Exporter
			ufw --force enable || true
		fi

		# Firewalld (CentOS/Rocky)
		if command -v firewall-cmd >/dev/null 2>&1; then
			firewall-cmd --permanent --add-port=22/tcp
			firewall-cmd --permanent --add-port=6443/tcp
			firewall-cmd --permanent --add-port=10250/tcp
			firewall-cmd --permanent --add-port=8472/udp
			firewall-cmd --permanent --add-port=9100/tcp
			firewall-cmd --reload || true
		fi

		echo "Firewall configured"
	`
}

// GetClusterToken retrieves the K3s token from a server node
func (k *K3sService) GetClusterToken(ctx context.Context, host string, port int, user string, privateKey []byte, password string) (string, error) {
	result, err := k.ssh.ExecuteWithConfig(ctx, host, port, user, privateKey, password, "cat /var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return "", fmt.Errorf("failed to get cluster token: %w", err)
	}
	return strings.TrimSpace(result.Stdout), nil
}

// GetNodeStatus checks K3s node status
func (k *K3sService) GetNodeStatus(ctx context.Context, host string, port int, user string, privateKey []byte, password string) (string, error) {
	result, err := k.ssh.ExecuteWithConfig(ctx, host, port, user, privateKey, password, "systemctl is-active k3s-agent || systemctl is-active k3s")
	if err != nil {
		return "unknown", nil
	}
	return strings.TrimSpace(result.Stdout), nil
}

// RemoveNode removes K3s from a node
func (k *K3sService) RemoveNode(ctx context.Context, opts *K3sInstallOptions) error {
	script := `
		# Stop and uninstall k3s
		if [ -f /usr/local/bin/k3s-agent-uninstall.sh ]; then
			/usr/local/bin/k3s-agent-uninstall.sh
		elif [ -f /usr/local/bin/k3s-uninstall.sh ]; then
			/usr/local/bin/k3s-uninstall.sh
		fi

		# Clean up
		rm -rf /var/lib/rancher/k3s
		rm -rf /etc/rancher/k3s
	`

	result, err := k.ssh.ExecuteScript(ctx, opts.Host, opts.Port, opts.User, opts.PrivateKey, opts.Password, script)
	if err != nil {
		return fmt.Errorf("failed to remove K3s: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("K3s removal failed: %s", result.Stderr)
	}

	return nil
}

// RebootNode reboots the node
func (k *K3sService) RebootNode(ctx context.Context, host string, port int, user string, privateKey []byte, password string) error {
	// Use nohup to ensure reboot happens even after SSH disconnects
	_, _ = k.ssh.ExecuteWithConfig(ctx, host, port, user, privateKey, password, "nohup bash -c 'sleep 2 && reboot' &>/dev/null &")
	return nil
}
