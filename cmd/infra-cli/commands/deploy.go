package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/infra"
)

func DeployCommand(args []string) {
	if len(args) == 0 {
		printDeployUsage()
		os.Exit(1)
	}

	cfg := LoadConfig()

	switch args[0] {
	case "new", "version":
		deployNew(cfg, args[1:])
	case "status":
		deployStatus(cfg)
	case "rollback":
		deployRollback(cfg, args[1:])
	case "help", "-h", "--help":
		printDeployUsage()
	default:
		// If first arg looks like a version, treat as deploy new
		if strings.HasPrefix(args[0], "v") || strings.HasPrefix(args[0], "--version") {
			deployNew(cfg, args)
		} else {
			fmt.Printf("Unknown deploy command: %s\n\n", args[0])
			printDeployUsage()
			os.Exit(1)
		}
	}
}

func printDeployUsage() {
	fmt.Println(`cold-infra deploy - Deploy application to K3s cluster

USAGE:
    cold-infra deploy <subcommand> [options]

SUBCOMMANDS:
    new           Build and deploy a new version
    status        Show current deployment status
    rollback      Rollback to previous version

OPTIONS:
    --version <tag>    Version tag (e.g., v1.5.50)
    --skip-build       Skip build, use existing image
    --skip-unhealthy   Continue even if some nodes fail

EXAMPLES:
    cold-infra deploy new --version v1.5.50
    cold-infra deploy new                          # Auto-generate version
    cold-infra deploy status
    cold-infra deploy rollback

CONFIGURATION REQUIRED:
    cold-infra config set build-path /path/to/cold-backend
    cold-infra config set image-repo lakshyajaat/cold-backend
    cold-infra config add node 192.168.15.110 control-plane
    cold-infra config add node 192.168.15.111 worker
`)
}

func deployNew(cfg *Config, args []string) {
	version := ""
	skipBuild := false
	skipUnhealthy := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			if i+1 < len(args) {
				version = args[i+1]
				i++
			}
		case "--skip-build":
			skipBuild = true
		case "--skip-unhealthy":
			skipUnhealthy = true
		default:
			// Treat as version if starts with v
			if strings.HasPrefix(args[i], "v") {
				version = args[i]
			}
		}
	}

	// Validate configuration
	if cfg.BuildPath == "" && !skipBuild {
		fmt.Println("Error: Build path not configured")
		fmt.Println("Run: cold-infra config set build-path /path/to/cold-backend")
		os.Exit(1)
	}

	if cfg.ImageRepo == "" {
		cfg.ImageRepo = "lakshyajaat/cold-backend"
	}

	if len(cfg.Nodes) == 0 {
		fmt.Println("Error: No nodes configured")
		fmt.Println("Run: cold-infra config add node <ip> <role>")
		os.Exit(1)
	}

	// Generate version if not provided
	if version == "" {
		version = fmt.Sprintf("v1.5.%d", time.Now().Unix())
	}

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  Deploying: %-44s  ║\n", version)
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	startTime := time.Now()

	// Step 1: Build
	if !skipBuild {
		fmt.Println("[1/5] Building Go binary...")
		if err := buildBinary(cfg.BuildPath); err != nil {
			fmt.Printf("Error building binary: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("      ✓ Binary built")

		fmt.Println("[2/5] Building Docker image...")
		if err := buildDockerImage(cfg.BuildPath, cfg.ImageRepo, version); err != nil {
			fmt.Printf("Error building Docker image: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("      ✓ Docker image built")

		fmt.Println("[3/5] Saving and compressing image...")
		if err := saveDockerImage(cfg.ImageRepo, version); err != nil {
			fmt.Printf("Error saving Docker image: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("      ✓ Image saved")
	} else {
		fmt.Println("[1-3/5] Skipping build (--skip-build)")
	}

	// Step 4: Distribute to nodes
	fmt.Println("[4/5] Distributing to nodes...")
	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)
	tarFile := fmt.Sprintf("/tmp/cold-backend-%s.tar.gz", version)

	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0
	successful := 0

	for _, node := range cfg.Nodes {
		wg.Add(1)
		go func(nodeIP string) {
			defer wg.Done()

			// Get private key if available
			var privateKey []byte
			if cfg.SSHKeyPath != "" {
				key, err := os.ReadFile(cfg.SSHKeyPath)
				if err == nil {
					privateKey = key
				}
			}

			// Copy tarball to node
			err := ssh.CopyFile(nil, nodeIP, 22, cfg.SSHUser, privateKey, "", tarFile, tarFile)
			if err != nil {
				fmt.Printf("      ✗ %s: copy failed (%v)\n", nodeIP, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			// Import image
			importCmd := fmt.Sprintf("gunzip -c %s | k3s ctr -n k8s.io images import - && rm -f %s", tarFile, tarFile)
			_, err = ssh.Execute(nil, nodeIP, importCmd)
			if err != nil {
				fmt.Printf("      ✗ %s: import failed (%v)\n", nodeIP, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			fmt.Printf("      ✓ %s: success\n", nodeIP)
			mu.Lock()
			successful++
			mu.Unlock()
		}(node.IP)
	}

	wg.Wait()

	// Check failure threshold
	totalNodes := len(cfg.Nodes)
	maxFailures := totalNodes * 40 / 100
	if maxFailures < 1 {
		maxFailures = 1
	}

	if failed > maxFailures && !skipUnhealthy {
		fmt.Printf("\n✗ Too many nodes failed (%d/%d). Max allowed: %d\n", failed, totalNodes, maxFailures)
		fmt.Println("Use --skip-unhealthy to continue anyway")
		os.Exit(1)
	}

	fmt.Printf("      Nodes updated: %d successful, %d failed\n", successful, failed)

	// Step 5: Update Kubernetes deployments
	fmt.Println("[5/5] Updating Kubernetes deployments...")

	// Find a control plane node to run kubectl
	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("Error: No control-plane node found in configuration")
		os.Exit(1)
	}

	// Get private key if available
	var privateKey []byte
	if cfg.SSHKeyPath != "" {
		key, err := os.ReadFile(cfg.SSHKeyPath)
		if err == nil {
			privateKey = key
		}
	}

	// Update both deployments
	updateCmd := fmt.Sprintf(
		"k3s kubectl set image deployment/cold-backend-employee cold-backend=%s:%s -n default && "+
			"k3s kubectl set image deployment/cold-backend-customer cold-backend=%s:%s -n default",
		cfg.ImageRepo, version, cfg.ImageRepo, version,
	)

	result, err := ssh.Execute(nil, controlPlane, updateCmd)
	if err != nil {
		fmt.Printf("Error updating deployments: %v\n", err)
		fmt.Println(result.Stderr)
		os.Exit(1)
	}
	fmt.Println("      ✓ Deployments updated")

	// Wait for rollout
	fmt.Println("      Waiting for rollout...")
	waitCmd := "k3s kubectl rollout status deployment/cold-backend-employee -n default --timeout=120s && " +
		"k3s kubectl rollout status deployment/cold-backend-customer -n default --timeout=120s"
	result, _ = ssh.Execute(nil, controlPlane, waitCmd)
	_ = privateKey // Mark as used

	elapsed := time.Since(startTime)
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Printf("║  ✓ Deployed %s successfully!                      ║\n", version)
	fmt.Printf("║  Time: %-49s  ║\n", elapsed.Round(time.Second))
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	// Cleanup local tar file
	os.Remove(tarFile)
}

func deployStatus(cfg *Config) {
	fmt.Println("Deployment Status")
	fmt.Println(strings.Repeat("=", 60))

	if len(cfg.Nodes) == 0 {
		fmt.Println("No nodes configured.")
		return
	}

	// Find a control plane node
	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("No control-plane node found.")
		return
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	// Get deployment status
	cmd := `k3s kubectl get pods -l app=cold-backend -n default -o custom-columns="NAME:.metadata.name,IMAGE:.spec.containers[0].image,STATUS:.status.phase,READY:.status.containerStatuses[0].ready" --no-headers`
	result, err := ssh.Execute(nil, controlPlane, cmd)
	if err != nil {
		fmt.Printf("Error getting status: %v\n", err)
		return
	}

	fmt.Println("\nRunning Pods:")
	fmt.Println(result.Stdout)

	// Get deployment details
	cmd = `k3s kubectl get deployments -l app=cold-backend -n default -o custom-columns="DEPLOYMENT:.metadata.name,REPLICAS:.spec.replicas,READY:.status.readyReplicas,VERSION:.spec.template.spec.containers[0].image" --no-headers`
	result, err = ssh.Execute(nil, controlPlane, cmd)
	if err == nil {
		fmt.Println("\nDeployments:")
		fmt.Println(result.Stdout)
	}
}

func deployRollback(cfg *Config, args []string) {
	target := "both" // employee, customer, or both

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target", "-t":
			if i+1 < len(args) {
				target = args[i+1]
				i++
			}
		}
	}

	if len(cfg.Nodes) == 0 {
		fmt.Println("Error: No nodes configured")
		os.Exit(1)
	}

	// Find control plane
	var controlPlane string
	for _, node := range cfg.Nodes {
		if node.Role == "control-plane" {
			controlPlane = node.IP
			break
		}
	}

	if controlPlane == "" {
		fmt.Println("Error: No control-plane node found")
		os.Exit(1)
	}

	ssh := infra.NewSSHService(cfg.SSHUser, cfg.SSHKeyPath)

	fmt.Printf("Rolling back %s deployment(s)...\n", target)

	var cmd string
	switch target {
	case "employee":
		cmd = "k3s kubectl rollout undo deployment/cold-backend-employee -n default"
	case "customer":
		cmd = "k3s kubectl rollout undo deployment/cold-backend-customer -n default"
	default:
		cmd = "k3s kubectl rollout undo deployment/cold-backend-employee -n default && " +
			"k3s kubectl rollout undo deployment/cold-backend-customer -n default"
	}

	result, err := ssh.Execute(nil, controlPlane, cmd)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		fmt.Println(result.Stderr)
		os.Exit(1)
	}

	fmt.Println("✓ Rollback initiated")
	fmt.Println(result.Stdout)
}

func buildBinary(buildPath string) error {
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf("cd %s && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-w -s' -o server ./cmd/server", buildPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildDockerImage(buildPath, imageRepo, version string) error {
	cmd := exec.Command("docker", "build",
		"-f", fmt.Sprintf("%s/Dockerfile.ci", buildPath),
		"-t", fmt.Sprintf("%s:%s", imageRepo, version),
		buildPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func saveDockerImage(imageRepo, version string) error {
	tarFile := fmt.Sprintf("/tmp/cold-backend-%s.tar.gz", version)
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf("docker save %s:%s | gzip > %s", imageRepo, version, tarFile))
	return cmd.Run()
}
