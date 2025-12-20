package main

import (
	"fmt"
	"os"

	"cold-backend/cmd/infra-cli/commands"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "cluster":
		commands.ClusterCommand(os.Args[2:])
	case "node":
		commands.NodeCommand(os.Args[2:])
	case "db":
		commands.DBCommand(os.Args[2:])
	case "deploy":
		commands.DeployCommand(os.Args[2:])
	case "config":
		commands.ConfigCommand(os.Args[2:])
	case "version":
		fmt.Printf("cold-infra version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`cold-infra - Cold Storage Infrastructure Management CLI

USAGE:
    cold-infra <command> [options]

COMMANDS:
    cluster     Manage K3s cluster (status, bootstrap, restore)
    node        Manage cluster nodes (add, remove, reboot, logs)
    db          Manage PostgreSQL database (status, restore, promote)
    deploy      Build and deploy to K3s cluster (works without GitHub runner)
    config      Manage CLI configuration (set, list)
    version     Print version information
    help        Show this help message

EXAMPLES:
    # Check cluster status via SSH (works without K8s API)
    cold-infra cluster status

    # Add a new node to the cluster
    cold-infra node add 192.168.15.115 --role worker

    # Remove a node from the cluster
    cold-infra node remove 192.168.15.115

    # Check database status
    cold-infra db status

    # Restore from backup
    cold-infra db restore /path/to/backup.sql

    # Configure settings
    cold-infra config set ssh-key ~/.ssh/id_rsa
    cold-infra config set cluster-vip 192.168.15.200
    cold-infra config list

    # Deploy new version (works without GitHub runner)
    cold-infra deploy new --version v1.5.50
    cold-infra deploy status
    cold-infra deploy rollback

For more information on a specific command, run:
    cold-infra <command> --help
`)
}
