package commands

import (
	"fmt"
	"os"
	"strings"

	"cold-backend/internal/infra"
)

func NodeCommand(args []string) {
	if len(args) == 0 {
		printNodeUsage()
		os.Exit(1)
	}

	cfg := LoadConfig()

	switch args[0] {
	case "add":
		nodeAdd(cfg, args[1:])
	case "remove":
		nodeRemove(cfg, args[1:])
	case "reboot":
		nodeReboot(cfg, args[1:])
	case "drain":
		nodeDrain(cfg, args[1:])
	case "cordon":
		nodeCordon(cfg, args[1:])
	case "uncordon":
		nodeUncordon(cfg, args[1:])
	case "logs":
		nodeLogs(cfg, args[1:])
	case "list":
		nodeList(cfg)
	case "help", "-h", "--help":
		printNodeUsage()
	default:
		fmt.Printf("Unknown node command: %s\n\n", args[0])
		printNodeUsage()
		os.Exit(1)
	}
}

func printNodeUsage() {
	fmt.Println(`cold-infra node - Manage cluster nodes

USAGE:
    cold-infra node <subcommand> [options]

SUBCOMMANDS:
    add         Add and provision a new node
    remove      Remove a node from the cluster
    reboot      Reboot a node via SSH
    drain       Drain pods from a node
    cordon      Mark node as unschedulable
    uncordon    Mark node as schedulable
    logs        View node system logs
    list        List all configured nodes

EXAMPLES:
    cold-infra node add 192.168.15.115 --role worker
    cold-infra node add 192.168.15.116 --role control-plane --password mypass
    cold-infra node remove 192.168.15.115
    cold-infra node reboot 192.168.15.115
    cold-infra node drain 192.168.15.115
    cold-infra node logs 192.168.15.115 --lines 50
    cold-infra node list
`)
}

func nodeAdd(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		fmt.Println("Usage: cold-infra node add <ip> [--role worker|control-plane] [--password <pass>]")
		os.Exit(1)
	}

	ip := args[0]
	role := "worker"
	password := ""

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--role", "-r":
			if i+1 < len(args) {
				role = args[i+1]
				i++
			}
		case "--password", "-p":
			if i+1 < len(args) {
				password = args[i+1]
				i++
			}
		}
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)
	k3s := infra.NewK3sService(ssh)

	fmt.Printf("Adding node %s as %s...\n", ip, role)

	// Test connection first
	fmt.Println("Testing SSH connection...")
	var privateKey []byte
	if cfg.SSHKeyPath != "" {
		key, err := os.ReadFile(cfg.SSHKeyPath)
		if err == nil {
			privateKey = key
		}
	}

	err := ssh.TestConnection(nil, ip, 22, cfg.SSHUser, privateKey, password)
	if err != nil {
		fmt.Printf("Error: Cannot connect to %s: %v\n", ip, err)
		os.Exit(1)
	}
	fmt.Println("Connection successful!")

	// Get cluster info
	var serverURL, token string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			serverURL = fmt.Sprintf("https://%s:6443", node.IP)
			result, err := ssh.Execute(nil, node.IP, "cat /var/lib/rancher/k3s/server/node-token 2>/dev/null")
			if err == nil {
				token = strings.TrimSpace(result.Stdout)
			}
			break
		}
	}

	if role == "worker" && (serverURL == "" || token == "") {
		fmt.Println("Error: No control plane node found. Add a control-plane node first or configure manually:")
		fmt.Println("  cold-infra config set k3s-server-url https://<control-plane-ip>:6443")
		fmt.Println("  cold-infra config set k3s-token <token>")
		os.Exit(1)
	}

	nodeRole := infra.RoleWorker
	if role == "control-plane" {
		nodeRole = infra.RoleControlPlane
	}

	opts := &infra.K3sInstallOptions{
		Host:       ip,
		Port:       22,
		User:       cfg.SSHUser,
		PrivateKey: privateKey,
		Password:   password,
		Role:       nodeRole,
		ServerURL:  serverURL,
		Token:      token,
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

	if nodeRole == infra.RoleControlPlane {
		err = k3s.InstallK3sServer(nil, opts, statusChan)
	} else {
		err = k3s.InstallK3sAgent(nil, opts, statusChan)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Add to config
	hostname := ip
	result, err := ssh.Execute(nil, ip, "hostname")
	if err == nil {
		hostname = strings.TrimSpace(result.Stdout)
	}

	cfg.Nodes = append(cfg.Nodes, NodeConfig{
		IP:       ip,
		Hostname: hostname,
		Role:     role,
	})
	SaveConfig(cfg)

	fmt.Printf("\nNode %s added successfully!\n", ip)
}

func nodeRemove(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	force := false

	for i := 1; i < len(args); i++ {
		if args[i] == "--force" || args[i] == "-f" {
			force = true
		}
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)
	k3s := infra.NewK3sService(ssh)

	fmt.Printf("Removing node %s from cluster...\n", ip)

	if !force {
		fmt.Print("Are you sure? This will remove the node from the cluster. [y/N]: ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	// Get node role
	nodeRole := infra.RoleWorker
	for _, node := range cfg.Nodes {
		if node.IP == ip {
			if node.Role == "control-plane" {
				nodeRole = infra.RoleControlPlane
			}
			break
		}
	}

	opts := &infra.K3sInstallOptions{
		Host: ip,
		Port: 22,
		User: cfg.SSHUser,
		Role: nodeRole,
	}

	err := k3s.RemoveNode(nil, opts)
	if err != nil {
		fmt.Printf("Warning: Error during removal: %v\n", err)
	}

	// Remove from config
	var newNodes []NodeConfig
	for _, node := range cfg.Nodes {
		if node.IP != ip {
			newNodes = append(newNodes, node)
		}
	}
	cfg.Nodes = newNodes
	SaveConfig(cfg)

	fmt.Printf("Node %s removed.\n", ip)
}

func nodeReboot(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	fmt.Printf("Rebooting node %s...\n", ip)
	_, err := ssh.Execute(nil, ip, "sudo reboot")
	if err != nil && !strings.Contains(err.Error(), "connection reset") {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Reboot initiated. Node will be unavailable for a few minutes.")
}

func nodeDrain(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	// Find control plane to run kubectl
	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("Error: No control plane node configured")
		os.Exit(1)
	}

	// Get node name
	hostname := ip
	for _, node := range cfg.Nodes {
		if node.IP == ip && node.Hostname != "" {
			hostname = node.Hostname
			break
		}
	}

	fmt.Printf("Draining node %s...\n", hostname)
	cmd := fmt.Sprintf("k3s kubectl drain %s --ignore-daemonsets --delete-emptydir-data --force", hostname)
	result, err := ssh.Execute(nil, controlPlane, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println(result.Stderr)
		os.Exit(1)
	}

	fmt.Println(result.Stdout)
	fmt.Printf("Node %s drained.\n", hostname)
}

func nodeCordon(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("Error: No control plane node configured")
		os.Exit(1)
	}

	hostname := ip
	for _, node := range cfg.Nodes {
		if node.IP == ip && node.Hostname != "" {
			hostname = node.Hostname
			break
		}
	}

	fmt.Printf("Cordoning node %s...\n", hostname)
	cmd := fmt.Sprintf("k3s kubectl cordon %s", hostname)
	result, err := ssh.Execute(nil, controlPlane, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.Stdout)
}

func nodeUncordon(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("Error: No control plane node configured")
		os.Exit(1)
	}

	hostname := ip
	for _, node := range cfg.Nodes {
		if node.IP == ip && node.Hostname != "" {
			hostname = node.Hostname
			break
		}
	}

	fmt.Printf("Uncordoning node %s...\n", hostname)
	cmd := fmt.Sprintf("k3s kubectl uncordon %s", hostname)
	result, err := ssh.Execute(nil, controlPlane, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.Stdout)
}

func nodeLogs(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: IP address required")
		os.Exit(1)
	}

	ip := args[0]
	lines := "50"

	for i := 1; i < len(args); i++ {
		if args[i] == "--lines" || args[i] == "-n" {
			if i+1 < len(args) {
				lines = args[i+1]
				i++
			}
		}
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	fmt.Printf("Fetching logs from %s (last %s lines)...\n\n", ip, lines)
	cmd := fmt.Sprintf("journalctl -n %s --no-pager", lines)
	result, err := ssh.Execute(nil, ip, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.Stdout)
}

func nodeList(cfg *Config) {
	if len(cfg.Nodes) == 0 {
		fmt.Println("No nodes configured.")
		fmt.Println("Use 'cold-infra node add <ip>' to add a node.")
		return
	}

	fmt.Println("Configured nodes:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-15s %-20s %-15s\n", "IP", "Hostname", "Role")
	fmt.Println(strings.Repeat("-", 60))

	for _, node := range cfg.Nodes {
		hostname := node.Hostname
		if hostname == "" {
			hostname = "-"
		}
		fmt.Printf("%-15s %-20s %-15s\n", node.IP, hostname, node.Role)
	}
}
