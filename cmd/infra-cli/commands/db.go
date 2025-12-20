package commands

import (
	"fmt"
	"os"
	"strings"

	"cold-backend/internal/infra"
)

func DBCommand(args []string) {
	if len(args) == 0 {
		printDBUsage()
		os.Exit(1)
	}

	cfg := LoadConfig()

	switch args[0] {
	case "status":
		dbStatus(cfg)
	case "restore":
		dbRestore(cfg, args[1:])
	case "backup":
		dbBackup(cfg, args[1:])
	case "promote":
		dbPromote(cfg, args[1:])
	case "connections":
		dbConnections(cfg)
	case "help", "-h", "--help":
		printDBUsage()
	default:
		fmt.Printf("Unknown db command: %s\n\n", args[0])
		printDBUsage()
		os.Exit(1)
	}
}

func printDBUsage() {
	fmt.Println(`cold-infra db - Manage PostgreSQL database

USAGE:
    cold-infra db <subcommand> [options]

SUBCOMMANDS:
    status        Check PostgreSQL status (primary and replicas)
    backup        Create a backup
    restore       Restore from a backup file
    promote       Promote replica to primary (disaster recovery)
    connections   Show active database connections

EXAMPLES:
    cold-infra db status
    cold-infra db backup --output /path/to/backup.sql
    cold-infra db restore /path/to/backup.sql
    cold-infra db promote --host 192.168.15.195
    cold-infra db connections
`)
}

func dbStatus(cfg *Config) {
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	fmt.Println("PostgreSQL Database Status")
	fmt.Println(strings.Repeat("=", 60))

	// Check cluster PostgreSQL
	if cfg.ClusterVIP != "" {
		fmt.Printf("\nCluster (VIP: %s):\n", cfg.ClusterVIP)

		// Try to find a control plane node to check pods
		for _, node := range cfg.Nodes {
			if node.Role == "control-plane" {
				result, err := ssh.Execute(nil, node.IP, "k3s kubectl get pods -l app=postgresql -o wide 2>/dev/null || echo 'No pods found'")
				if err == nil {
					fmt.Println(result.Stdout)
				}

				// Check primary/replica status
				result, err = ssh.Execute(nil, node.IP, `k3s kubectl exec -it $(k3s kubectl get pods -l app=postgresql -o jsonpath='{.items[0].metadata.name}' 2>/dev/null) -- psql -U postgres -c "SELECT pg_is_in_recovery();" 2>/dev/null || echo 'Cannot check'`)
				if err == nil && !strings.Contains(result.Stdout, "Cannot check") {
					fmt.Printf("Recovery mode: %s\n", strings.TrimSpace(result.Stdout))
				}
				break
			}
		}
	}

	// Check offsite database
	if cfg.OffsiteDBHost != "" {
		fmt.Printf("\nOffsite Database (%s:%s):\n", cfg.OffsiteDBHost, cfg.OffsiteDBPort)

		// Check if PostgreSQL is running
		result, err := ssh.Execute(nil, cfg.OffsiteDBHost, "systemctl is-active postgresql 2>/dev/null || docker ps --filter name=postgres --format '{{.Status}}' 2>/dev/null")
		if err != nil {
			fmt.Printf("  Status: UNREACHABLE (%v)\n", err)
		} else {
			status := strings.TrimSpace(result.Stdout)
			if status == "active" || strings.Contains(status, "Up") {
				fmt.Printf("  Status: RUNNING\n")
			} else {
				fmt.Printf("  Status: %s\n", status)
			}
		}

		// Check replication lag
		result, err = ssh.Execute(nil, cfg.OffsiteDBHost, fmt.Sprintf(`docker exec cold-storage-postgres psql -U postgres -c "SELECT pg_is_in_recovery();" 2>/dev/null || PGPASSWORD=postgres psql -h localhost -p %s -U postgres -c "SELECT pg_is_in_recovery();" 2>/dev/null`, cfg.OffsiteDBPort))
		if err == nil {
			if strings.Contains(result.Stdout, "t") {
				fmt.Printf("  Mode: REPLICA (streaming)\n")

				// Get lag if it's a replica
				result, err = ssh.Execute(nil, cfg.OffsiteDBHost, `docker exec cold-storage-postgres psql -U postgres -c "SELECT CASE WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 0 ELSE EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp()) END AS lag_seconds;" 2>/dev/null`)
				if err == nil {
					fmt.Printf("  Replication lag: %s seconds\n", strings.TrimSpace(result.Stdout))
				}
			} else if strings.Contains(result.Stdout, "f") {
				fmt.Printf("  Mode: PRIMARY\n")
			}
		}

		// Check disk usage
		result, err = ssh.Execute(nil, cfg.OffsiteDBHost, "df -h /var/lib/postgresql 2>/dev/null || df -h / | tail -1")
		if err == nil {
			fmt.Printf("  Disk: %s\n", strings.TrimSpace(result.Stdout))
		}
	}
}

func dbBackup(cfg *Config, args []string) {
	output := "/tmp/cold-backup-" + strings.ReplaceAll(strings.Split(strings.ReplaceAll(fmt.Sprintf("%v", os.Getenv("USER")), " ", "_"), "@")[0], " ", "_") + ".sql"

	for i := 0; i < len(args); i++ {
		if args[i] == "--output" || args[i] == "-o" {
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		}
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	fmt.Println("Creating database backup...")

	// Find a working database
	var dbHost string
	if cfg.OffsiteDBHost != "" {
		dbHost = cfg.OffsiteDBHost
	} else if cfg.ClusterVIP != "" {
		dbHost = cfg.ClusterVIP
	}

	if dbHost == "" {
		fmt.Println("Error: No database host configured")
		os.Exit(1)
	}

	fmt.Printf("Connecting to %s...\n", dbHost)

	// Run pg_dump
	cmd := fmt.Sprintf(`docker exec cold-storage-postgres pg_dump -U postgres cold_db 2>/dev/null || PGPASSWORD=postgres pg_dump -h localhost -U postgres cold_db`)
	result, err := ssh.Execute(nil, dbHost, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Save to file
	err = os.WriteFile(output, []byte(result.Stdout), 0644)
	if err != nil {
		fmt.Printf("Error writing backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup saved to: %s\n", output)
}

func dbRestore(cfg *Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Error: Backup file path required")
		fmt.Println("Usage: cold-infra db restore <backup.sql> [--host <ip>]")
		os.Exit(1)
	}

	backupFile := args[0]
	targetHost := cfg.OffsiteDBHost

	for i := 1; i < len(args); i++ {
		if args[i] == "--host" || args[i] == "-h" {
			if i+1 < len(args) {
				targetHost = args[i+1]
				i++
			}
		}
	}

	if targetHost == "" {
		fmt.Println("Error: No target host specified. Use --host or configure offsite-db-host")
		os.Exit(1)
	}

	// Check backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		fmt.Printf("Error: Backup file not found: %s\n", backupFile)
		os.Exit(1)
	}

	fmt.Printf("Restoring backup to %s...\n", targetHost)
	fmt.Println("WARNING: This will overwrite the existing database!")
	fmt.Print("Are you sure? [y/N]: ")

	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("Aborted.")
		return
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	// Get private key if available
	var privateKey []byte
	if cfg.SSHKeyPath != "" {
		key, err := os.ReadFile(cfg.SSHKeyPath)
		if err == nil {
			privateKey = key
		}
	}

	// Copy backup file to target
	remotePath := "/tmp/restore-backup.sql"
	err := ssh.CopyFile(nil, targetHost, 22, cfg.SSHUser, privateKey, "", backupFile, remotePath)
	if err != nil {
		fmt.Printf("Error copying backup: %v\n", err)
		os.Exit(1)
	}

	// Restore
	cmd := fmt.Sprintf(`docker exec -i cold-storage-postgres psql -U postgres cold_db < %s 2>&1 || PGPASSWORD=postgres psql -h localhost -U postgres cold_db < %s 2>&1`, remotePath, remotePath)
	result, err := ssh.Execute(nil, targetHost, cmd)
	if err != nil {
		fmt.Printf("Error restoring: %v\n", err)
		fmt.Println(result.Stderr)
		os.Exit(1)
	}

	fmt.Println("Restore completed!")
	fmt.Println(result.Stdout)
}

func dbPromote(cfg *Config, args []string) {
	targetHost := cfg.OffsiteDBHost

	for i := 0; i < len(args); i++ {
		if args[i] == "--host" || args[i] == "-h" {
			if i+1 < len(args) {
				targetHost = args[i+1]
				i++
			}
		}
	}

	if targetHost == "" {
		fmt.Println("Error: No target host specified")
		os.Exit(1)
	}

	fmt.Printf("Promoting replica at %s to PRIMARY...\n", targetHost)
	fmt.Println("WARNING: This is a disaster recovery operation!")
	fmt.Println("Only proceed if the original primary is permanently unavailable.")
	fmt.Print("Are you sure? [y/N]: ")

	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
		fmt.Println("Aborted.")
		return
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	// Promote replica
	cmd := `docker exec cold-storage-postgres pg_ctl promote -D /var/lib/postgresql/data 2>&1 || pg_ctl promote -D /var/lib/postgresql/data 2>&1`
	result, err := ssh.Execute(nil, targetHost, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println(result.Stderr)
		os.Exit(1)
	}

	fmt.Println("Replica promoted to primary!")
	fmt.Println(result.Stdout)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Update application connection strings to point to the new primary")
	fmt.Println("  2. Configure old primary as replica (when recovered)")
}

func dbConnections(cfg *Config) {
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	var dbHost string
	if cfg.OffsiteDBHost != "" {
		dbHost = cfg.OffsiteDBHost
	} else if cfg.ClusterVIP != "" {
		dbHost = cfg.ClusterVIP
	}

	if dbHost == "" {
		fmt.Println("Error: No database host configured")
		os.Exit(1)
	}

	fmt.Printf("Active connections on %s:\n", dbHost)
	fmt.Println(strings.Repeat("-", 80))

	cmd := `docker exec cold-storage-postgres psql -U postgres -c "SELECT pid, usename, application_name, client_addr, state, query_start, left(query, 50) as query FROM pg_stat_activity WHERE datname = 'cold_db';" 2>/dev/null`
	result, err := ssh.Execute(nil, dbHost, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.Stdout)
}
