package commands

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"cold-backend/internal/infra"
)

func ClusterCommand(args []string) {
	if len(args) == 0 {
		printClusterUsage()
		os.Exit(1)
	}

	cfg := LoadConfig()

	switch args[0] {
	case "status":
		clusterStatus(cfg)
	case "bootstrap":
		clusterBootstrap(cfg, args[1:])
	case "token":
		clusterToken(cfg)
	case "help", "-h", "--help":
		printClusterUsage()
	default:
		fmt.Printf("Unknown cluster command: %s\n\n", args[0])
		printClusterUsage()
		os.Exit(1)
	}
}

func printClusterUsage() {
	fmt.Println(`cold-infra cluster - Manage K3s cluster

USAGE:
    cold-infra cluster <subcommand> [options]

SUBCOMMANDS:
    status      Check status of all cluster nodes via SSH
    bootstrap   Bootstrap a new K3s cluster
    token       Get cluster join token from control plane

EXAMPLES:
    cold-infra cluster status
    cold-infra cluster bootstrap --control-plane 192.168.15.110
    cold-infra cluster token
`)
}

func clusterStatus(cfg *Config) {
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	if len(cfg.Nodes) == 0 {
		fmt.Println("No nodes configured. Use 'cold-infra config add node <ip>' to add nodes.")
		return
	}

	fmt.Println("Checking cluster status via SSH...")
	fmt.Println(strings.Repeat("=", 80))

	var wg sync.WaitGroup
	results := make(chan string, len(cfg.Nodes))

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(n NodeConfig) {
			defer wg.Done()
			result := checkNodeStatus(ssh, n)
			results <- result
		}(node)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		fmt.Println(result)
	}

	fmt.Println(strings.Repeat("=", 80))

	// Try to get k3s status from control plane
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			fmt.Printf("\nK3s Cluster Status (from %s):\n", node.IP)
			result, err := ssh.Execute(nil, node.IP, "k3s kubectl get nodes -o wide 2>/dev/null || echo 'K3s not running'")
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
			} else {
				fmt.Println(result.Stdout)
			}
			break
		}
	}
}

func checkNodeStatus(ssh *infra.SSHService, node NodeConfig) string {
	var status strings.Builder
	status.WriteString(fmt.Sprintf("\n[%s] %s (%s)\n", node.Role, node.IP, node.Hostname))

	// Check if node is reachable
	err := ssh.TestConnection(nil, node.IP, 22, "", nil, "")
	if err != nil {
		status.WriteString(fmt.Sprintf("  Status: UNREACHABLE (%v)\n", err))
		return status.String()
	}

	// Check K3s service
	result, err := ssh.Execute(nil, node.IP, "systemctl is-active k3s k3s-agent 2>/dev/null | head -1")
	if err != nil {
		status.WriteString(fmt.Sprintf("  K3s: ERROR (%v)\n", err))
	} else {
		k3sStatus := strings.TrimSpace(result.Stdout)
		if k3sStatus == "active" {
			status.WriteString("  K3s: RUNNING\n")
		} else {
			status.WriteString(fmt.Sprintf("  K3s: %s\n", k3sStatus))
		}
	}

	// Check system load
	result, err = ssh.Execute(nil, node.IP, "uptime | awk -F'load average:' '{print $2}'")
	if err == nil {
		status.WriteString(fmt.Sprintf("  Load: %s\n", strings.TrimSpace(result.Stdout)))
	}

	// Check memory
	result, err = ssh.Execute(nil, node.IP, "free -h | awk '/^Mem:/ {print $3\"/\"$2}'")
	if err == nil {
		status.WriteString(fmt.Sprintf("  Memory: %s\n", strings.TrimSpace(result.Stdout)))
	}

	// Check disk
	result, err = ssh.Execute(nil, node.IP, "df -h / | awk 'NR==2 {print $3\"/\"$2\" (\"$5\" used)\"}'")
	if err == nil {
		status.WriteString(fmt.Sprintf("  Disk: %s\n", strings.TrimSpace(result.Stdout)))
	}

	return status.String()
}

func clusterBootstrap(cfg *Config, args []string) {
	var controlPlaneIP string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--control-plane", "-c":
			if i+1 < len(args) {
				controlPlaneIP = args[i+1]
				i++
			}
		}
	}

	if controlPlaneIP == "" {
		// Try to find control plane from config
		for _, node := range cfg.Nodes {
			if node.Role == "control-plane" {
				controlPlaneIP = node.IP
				break
			}
		}
	}

	if controlPlaneIP == "" {
		fmt.Println("Error: No control plane IP specified.")
		fmt.Println("Use: cold-infra cluster bootstrap --control-plane <ip>")
		os.Exit(1)
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)
	k3s := infra.NewK3sService(ssh)

	fmt.Printf("Bootstrapping K3s cluster on %s...\n", controlPlaneIP)

	opts := &infra.K3sInstallOptions{
		Host:       controlPlaneIP,
		Port:       22,
		User:       cfg.SSHUser,
		PrivateKey: nil, // Will use default
		Role:       infra.RoleControlPlane,
	}

	statusChan := make(chan infra.ProvisionStatus, 100)
	go func() {
		for status := range statusChan {
			if status.Error != "" {
				fmt.Printf("[%s] ERROR - %s\n", status.Step, status.Error)
			} else {
				fmt.Printf("[%s] %d%% - %s\n", status.Step, status.Progress, status.Message)
			}
		}
	}()

	err := k3s.InstallK3sServer(nil, opts, statusChan)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nCluster bootstrapped successfully!")
	fmt.Println("Next steps:")
	fmt.Println("  1. Get the join token: cold-infra cluster token")
	fmt.Println("  2. Add worker nodes: cold-infra node add <ip>")
}

func clusterToken(cfg *Config) {
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	var controlPlaneIP string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlaneIP = node.IP
			break
		}
	}

	if controlPlaneIP == "" {
		fmt.Println("Error: No control plane node configured.")
		os.Exit(1)
	}

	result, err := ssh.Execute(nil, controlPlaneIP, "cat /var/lib/rancher/k3s/server/node-token 2>/dev/null")
	if err != nil {
		fmt.Printf("Error getting token: %v\n", err)
		os.Exit(1)
	}

	token := strings.TrimSpace(result.Stdout)
	if token == "" {
		fmt.Println("Error: Token not found. Is K3s server running?")
		os.Exit(1)
	}

	fmt.Println("K3s Join Token:")
	fmt.Println(token)
	fmt.Printf("\nServer URL: https://%s:6443\n", controlPlaneIP)
}
