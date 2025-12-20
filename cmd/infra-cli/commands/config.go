package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds CLI configuration
type Config struct {
	SSHUser       string       `json:"ssh_user"`
	SSHKeyPath    string       `json:"ssh_key_path"`
	ClusterVIP    string       `json:"cluster_vip"`
	K3sServerURL  string       `json:"k3s_server_url"`
	K3sToken      string       `json:"k3s_token"`
	OffsiteDBHost string       `json:"offsite_db_host"`
	OffsiteDBPort string       `json:"offsite_db_port"`
	NASMountPath  string       `json:"nas_mount_path"`
	BuildPath     string       `json:"build_path"`
	ImageRepo     string       `json:"image_repo"`
	Nodes         []NodeConfig `json:"nodes"`
}

// NodeConfig holds node configuration
type NodeConfig struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Role     string `json:"role"`
}

func ConfigCommand(args []string) {
	if len(args) == 0 {
		printConfigUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "set":
		configSet(args[1:])
	case "get":
		configGet(args[1:])
	case "list":
		configList()
	case "add":
		configAdd(args[1:])
	case "remove":
		configRemove(args[1:])
	case "help", "-h", "--help":
		printConfigUsage()
	default:
		fmt.Printf("Unknown config command: %s\n\n", args[0])
		printConfigUsage()
		os.Exit(1)
	}
}

func printConfigUsage() {
	fmt.Println(`cold-infra config - Manage CLI configuration

USAGE:
    cold-infra config <subcommand> [options]

SUBCOMMANDS:
    set <key> <value>     Set a configuration value
    get <key>             Get a configuration value
    list                  List all configuration
    add node <ip> [role]  Add a node to configuration
    remove node <ip>      Remove a node from configuration

CONFIGURATION KEYS:
    ssh-user          Default SSH user (default: root)
    ssh-key           Path to SSH private key
    cluster-vip       Cluster VIP address
    k3s-server-url    K3s server URL (https://ip:6443)
    k3s-token         K3s cluster join token
    offsite-db-host   Offsite database IP
    offsite-db-port   Offsite database port
    nas-mount-path    NAS backup mount path

EXAMPLES:
    cold-infra config set ssh-key ~/.ssh/id_rsa
    cold-infra config set cluster-vip 192.168.15.200
    cold-infra config set offsite-db-host 192.168.15.195
    cold-infra config add node 192.168.15.110 control-plane
    cold-infra config add node 192.168.15.111 worker
    cold-infra config list
`)
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cold-infra", "config.json")
}

// LoadConfig loads configuration from file
func LoadConfig() *Config {
	cfg := &Config{
		SSHUser:       "root",
		OffsiteDBPort: "5434",
		Nodes:         []NodeConfig{},
	}

	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return cfg
	}

	json.Unmarshal(data, cfg)
	return cfg
}

// SaveConfig saves configuration to file
func SaveConfig(cfg *Config) error {
	configPath := getConfigPath()

	// Create directory if not exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func configSet(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: Key and value required")
		fmt.Println("Usage: cold-infra config set <key> <value>")
		os.Exit(1)
	}

	key := args[0]
	value := strings.Join(args[1:], " ")

	cfg := LoadConfig()

	switch key {
	case "ssh-user":
		cfg.SSHUser = value
	case "ssh-key":
		// Expand home directory
		if strings.HasPrefix(value, "~") {
			home, _ := os.UserHomeDir()
			value = filepath.Join(home, value[1:])
		}
		cfg.SSHKeyPath = value
	case "cluster-vip":
		cfg.ClusterVIP = value
	case "k3s-server-url":
		cfg.K3sServerURL = value
	case "k3s-token":
		cfg.K3sToken = value
	case "offsite-db-host":
		cfg.OffsiteDBHost = value
	case "offsite-db-port":
		cfg.OffsiteDBPort = value
	case "nas-mount-path":
		cfg.NASMountPath = value
	case "build-path":
		// Expand home directory
		if strings.HasPrefix(value, "~") {
			home, _ := os.UserHomeDir()
			value = filepath.Join(home, value[1:])
		}
		cfg.BuildPath = value
	case "image-repo":
		cfg.ImageRepo = value
	default:
		fmt.Printf("Unknown configuration key: %s\n", key)
		fmt.Println("Valid keys: ssh-user, ssh-key, cluster-vip, k3s-server-url, k3s-token, offsite-db-host, offsite-db-port, nas-mount-path, build-path, image-repo")
		os.Exit(1)
	}

	if err := SaveConfig(cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Set %s = %s\n", key, value)
}

func configGet(args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Key required")
		os.Exit(1)
	}

	key := args[0]
	cfg := LoadConfig()

	var value string
	switch key {
	case "ssh-user":
		value = cfg.SSHUser
	case "ssh-key":
		value = cfg.SSHKeyPath
	case "cluster-vip":
		value = cfg.ClusterVIP
	case "k3s-server-url":
		value = cfg.K3sServerURL
	case "k3s-token":
		value = cfg.K3sToken
	case "offsite-db-host":
		value = cfg.OffsiteDBHost
	case "offsite-db-port":
		value = cfg.OffsiteDBPort
	case "nas-mount-path":
		value = cfg.NASMountPath
	case "build-path":
		value = cfg.BuildPath
	case "image-repo":
		value = cfg.ImageRepo
	default:
		fmt.Printf("Unknown configuration key: %s\n", key)
		os.Exit(1)
	}

	if value == "" {
		fmt.Println("(not set)")
	} else {
		fmt.Println(value)
	}
}

func configList() {
	cfg := LoadConfig()

	fmt.Println("Cold Infrastructure CLI Configuration")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Config file: %s\n\n", getConfigPath())

	fmt.Println("General Settings:")
	fmt.Printf("  ssh-user:        %s\n", valueOrDefault(cfg.SSHUser, "(not set)"))
	fmt.Printf("  ssh-key:         %s\n", valueOrDefault(cfg.SSHKeyPath, "(not set)"))

	fmt.Println("\nCluster Settings:")
	fmt.Printf("  cluster-vip:     %s\n", valueOrDefault(cfg.ClusterVIP, "(not set)"))
	fmt.Printf("  k3s-server-url:  %s\n", valueOrDefault(cfg.K3sServerURL, "(not set)"))
	fmt.Printf("  k3s-token:       %s\n", maskToken(cfg.K3sToken))

	fmt.Println("\nDatabase Settings:")
	fmt.Printf("  offsite-db-host: %s\n", valueOrDefault(cfg.OffsiteDBHost, "(not set)"))
	fmt.Printf("  offsite-db-port: %s\n", valueOrDefault(cfg.OffsiteDBPort, "(not set)"))
	fmt.Printf("  nas-mount-path:  %s\n", valueOrDefault(cfg.NASMountPath, "(not set)"))

	fmt.Println("\nDeploy Settings:")
	fmt.Printf("  build-path:      %s\n", valueOrDefault(cfg.BuildPath, "(not set)"))
	fmt.Printf("  image-repo:      %s\n", valueOrDefault(cfg.ImageRepo, "(not set)"))

	fmt.Println("\nConfigured Nodes:")
	if len(cfg.Nodes) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, node := range cfg.Nodes {
			fmt.Printf("  - %s (%s) [%s]\n", node.IP, node.Hostname, node.Role)
		}
	}
}

func configAdd(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: Specify what to add")
		fmt.Println("Usage: cold-infra config add node <ip> [role]")
		os.Exit(1)
	}

	switch args[0] {
	case "node":
		if len(args) < 2 {
			fmt.Println("Error: IP address required")
			os.Exit(1)
		}

		ip := args[1]
		role := "worker"
		if len(args) > 2 {
			role = args[2]
		}

		cfg := LoadConfig()

		// Check if already exists
		for _, node := range cfg.Nodes {
			if node.IP == ip {
				fmt.Printf("Node %s already exists\n", ip)
				return
			}
		}

		cfg.Nodes = append(cfg.Nodes, NodeConfig{
			IP:   ip,
			Role: role,
		})

		if err := SaveConfig(cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added node %s (%s)\n", ip, role)

	default:
		fmt.Printf("Unknown add target: %s\n", args[0])
		os.Exit(1)
	}
}

func configRemove(args []string) {
	if len(args) < 2 {
		fmt.Println("Error: Specify what to remove")
		fmt.Println("Usage: cold-infra config remove node <ip>")
		os.Exit(1)
	}

	switch args[0] {
	case "node":
		ip := args[1]
		cfg := LoadConfig()

		var newNodes []NodeConfig
		found := false
		for _, node := range cfg.Nodes {
			if node.IP == ip {
				found = true
			} else {
				newNodes = append(newNodes, node)
			}
		}

		if !found {
			fmt.Printf("Node %s not found\n", ip)
			return
		}

		cfg.Nodes = newNodes
		if err := SaveConfig(cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed node %s\n", ip)

	default:
		fmt.Printf("Unknown remove target: %s\n", args[0])
		os.Exit(1)
	}
}

func valueOrDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func maskToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	if len(token) > 10 {
		return token[:5] + "..." + token[len(token)-5:]
	}
	return "***"
}
