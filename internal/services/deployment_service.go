package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/infra"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// DeploymentService handles build and deployment operations
type DeploymentService struct {
	repo *repositories.InfrastructureRepository
	ssh  *infra.SSHService

	// Active deployment jobs
	jobs     map[int]*DeployJob
	jobMutex sync.RWMutex
}

// DeployJob represents an active deployment job
type DeployJob struct {
	HistoryID  int
	Status     chan DeployProgress
	Cancel     context.CancelFunc
	StartedAt  time.Time
	LastUpdate DeployProgress
}

// DeployProgress represents deployment progress
type DeployProgress struct {
	Step     string `json:"step"`     // building, distributing, updating, verifying, complete, failed
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
	Error    string `json:"error,omitempty"`
}

// DeployOptions holds deployment options
type DeployOptions struct {
	Version       string   `json:"version"`
	SkipBuild     bool     `json:"skip_build"`
	DeployTargets []string `json:"deploy_targets"` // ["employee", "customer"]
	UserID        int      `json:"user_id"`
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(repo *repositories.InfrastructureRepository) *DeploymentService {
	sshKeyPath := os.Getenv("SSH_KEY_PATH")
	if sshKeyPath == "" {
		sshKeyPath = "/etc/cold-backend/ssh/id_rsa"
	}

	sshService := infra.NewSSHService("root", sshKeyPath)

	return &DeploymentService{
		repo: repo,
		ssh:  sshService,
		jobs: make(map[int]*DeployJob),
	}
}

// ListDeploymentConfigs returns all deployment configurations
func (s *DeploymentService) ListDeploymentConfigs(ctx context.Context) ([]*models.DeploymentConfig, error) {
	return s.repo.ListDeploymentConfigs(ctx)
}

// GetDeploymentConfig returns a deployment configuration by ID
func (s *DeploymentService) GetDeploymentConfig(ctx context.Context, id int) (*models.DeploymentConfig, error) {
	return s.repo.GetDeploymentConfig(ctx, id)
}

// GetDeploymentHistory returns deployment history for a config
func (s *DeploymentService) GetDeploymentHistory(ctx context.Context, configID int, limit int) ([]*models.DeploymentHistory, error) {
	return s.repo.GetDeploymentHistory(ctx, configID, limit)
}

// Deploy starts a new deployment
func (s *DeploymentService) Deploy(ctx context.Context, configID int, opts DeployOptions) (int, chan DeployProgress, error) {
	// Get deployment config
	config, err := s.repo.GetDeploymentConfig(ctx, configID)
	if err != nil {
		return 0, nil, fmt.Errorf("deployment config not found: %w", err)
	}

	// Generate version if not provided
	if opts.Version == "" {
		opts.Version = fmt.Sprintf("v1.5.%d", time.Now().Unix())
	}

	// Default deploy targets
	if len(opts.DeployTargets) == 0 {
		opts.DeployTargets = []string{"employee", "customer"}
	}

	// Create history record
	history := &models.DeploymentHistory{
		DeploymentID:    configID,
		Version:         opts.Version,
		PreviousVersion: config.CurrentVersion,
		DeployedBy:      opts.UserID,
		Status:          models.DeploymentStatusPending,
	}

	if err := s.repo.CreateDeploymentHistory(ctx, history); err != nil {
		return 0, nil, fmt.Errorf("failed to create history: %w", err)
	}

	// Create progress channel
	progressChan := make(chan DeployProgress, 100)

	// Create cancellable context
	deployCtx, cancel := context.WithCancel(context.Background())

	// Create job
	job := &DeployJob{
		HistoryID: history.ID,
		Status:    progressChan,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}

	s.jobMutex.Lock()
	s.jobs[history.ID] = job
	s.jobMutex.Unlock()

	// Start deployment in background
	go s.runDeployment(deployCtx, config, history, opts, progressChan)

	return history.ID, progressChan, nil
}

// runDeployment executes the deployment process
func (s *DeploymentService) runDeployment(ctx context.Context, config *models.DeploymentConfig, history *models.DeploymentHistory, opts DeployOptions, progressChan chan<- DeployProgress) {
	defer close(progressChan)
	defer func() {
		s.jobMutex.Lock()
		delete(s.jobs, history.ID)
		s.jobMutex.Unlock()
	}()

	sendProgress := func(step string, progress int, message string) {
		progressChan <- DeployProgress{Step: step, Progress: progress, Message: message}
	}

	sendError := func(step string, err error) {
		progressChan <- DeployProgress{Step: "failed", Progress: 0, Message: step, Error: err.Error()}
		s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusFailed, "", "", err.Error())
	}

	// Update status to building
	s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusBuilding, "", "", "")

	// Step 1: Build binary
	if !opts.SkipBuild {
		sendProgress("building", 10, "Building Go binary...")
		buildOutput, err := s.buildBinary(config.BuildContext, config.BuildCommand)
		if err != nil {
			sendError("Build binary failed", err)
			return
		}
		s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusBuilding, buildOutput, "", "")

		// Step 2: Build Docker image
		sendProgress("building", 30, "Building Docker image...")
		dockerOutput, err := s.buildDockerImage(config.BuildContext, config.ImageRepo, opts.Version)
		if err != nil {
			sendError("Build Docker image failed", err)
			return
		}
		s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusBuilding, buildOutput+"\n"+dockerOutput, "", "")

		// Step 3: Save image
		sendProgress("building", 40, "Saving and compressing image...")
		if err := s.saveDockerImage(config.ImageRepo, opts.Version); err != nil {
			sendError("Save Docker image failed", err)
			return
		}
	} else {
		sendProgress("building", 40, "Skipping build (using existing image)")
	}

	// Step 4: Distribute to nodes
	s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusDeploying, "", "", "")
	sendProgress("distributing", 50, "Distributing to cluster nodes...")

	nodes, err := s.repo.ListNodes(ctx)
	if err != nil {
		sendError("Failed to get nodes", err)
		return
	}

	if len(nodes) == 0 {
		sendError("No nodes configured", fmt.Errorf("no nodes in database"))
		return
	}

	tarFile := fmt.Sprintf("/tmp/cold-backend-%s.tar.gz", opts.Version)
	failed := 0
	total := len(nodes)
	progressPerNode := 20 / total

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, node := range nodes {
		wg.Add(1)
		go func(n *models.ClusterNode, idx int) {
			defer wg.Done()

			if err := s.distributeToNode(n, tarFile, opts.Version); err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				sendProgress("distributing", 50+progressPerNode*(idx+1), fmt.Sprintf("Node %s: failed (%v)", n.IPAddress, err))
			} else {
				sendProgress("distributing", 50+progressPerNode*(idx+1), fmt.Sprintf("Node %s: success", n.IPAddress))
			}
		}(node, i)
	}

	wg.Wait()

	// Check failure threshold (40%)
	maxFailures := total * 40 / 100
	if maxFailures < 1 {
		maxFailures = 1
	}

	if failed > maxFailures {
		sendError("Too many nodes failed", fmt.Errorf("%d/%d nodes failed (max: %d)", failed, total, maxFailures))
		return
	}

	// Step 5: Update Kubernetes deployments
	sendProgress("updating", 80, "Updating Kubernetes deployments...")

	// Find control plane node
	var controlPlane *models.ClusterNode
	for _, node := range nodes {
		if node.Role == models.NodeRoleControlPlane {
			controlPlane = node
			break
		}
	}

	if controlPlane == nil {
		sendError("No control plane node", fmt.Errorf("no control-plane node found"))
		return
	}

	deployOutput := ""
	for _, target := range opts.DeployTargets {
		deployName := fmt.Sprintf("cold-backend-%s", target)
		cmd := fmt.Sprintf("k3s kubectl set image deployment/%s cold-backend=%s:%s -n default",
			deployName, config.ImageRepo, opts.Version)

		result, err := s.ssh.Execute(nil, controlPlane.IPAddress, cmd)
		if err != nil {
			sendError(fmt.Sprintf("Update %s deployment failed", target), err)
			return
		}
		deployOutput += result.Stdout + "\n"
		sendProgress("updating", 85, fmt.Sprintf("Updated %s deployment", target))
	}

	// Step 6: Wait for rollout
	sendProgress("verifying", 90, "Waiting for rollout...")

	for _, target := range opts.DeployTargets {
		deployName := fmt.Sprintf("cold-backend-%s", target)
		cmd := fmt.Sprintf("k3s kubectl rollout status deployment/%s -n default --timeout=120s", deployName)
		s.ssh.Execute(nil, controlPlane.IPAddress, cmd)
	}

	// Step 7: Health check
	sendProgress("verifying", 95, "Verifying pod health...")

	healthCmd := fmt.Sprintf(`k3s kubectl get pods -l app=cold-backend -n default -o jsonpath='{range .items[*]}{.spec.containers[0].image}{" "}{.status.containerStatuses[0].ready}{"\n"}{end}' | grep -c "%s.*true"`, opts.Version)
	result, _ := s.ssh.Execute(nil, controlPlane.IPAddress, healthCmd)
	readyPods := strings.TrimSpace(result.Stdout)

	if readyPods == "" || readyPods == "0" {
		// Not healthy, rollback
		sendProgress("verifying", 95, "Deployment unhealthy, initiating rollback...")
		s.rollback(controlPlane.IPAddress, opts.DeployTargets)
		sendError("Health check failed", fmt.Errorf("no healthy pods with version %s", opts.Version))
		s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusRolledback, "", deployOutput, "Health check failed")
		return
	}

	// Success!
	s.repo.UpdateDeploymentHistory(ctx, history.ID, models.DeploymentStatusSuccess, "", deployOutput, "")
	s.repo.UpdateDeploymentVersion(ctx, config.ID, opts.Version)
	sendProgress("complete", 100, fmt.Sprintf("Deployed %s successfully! (%s ready pods)", opts.Version, readyPods))

	// Cleanup
	os.Remove(tarFile)
}

// Rollback initiates a rollback
func (s *DeploymentService) Rollback(ctx context.Context, configID int, userID int) error {
	nodes, err := s.repo.ListNodes(ctx)
	if err != nil {
		return err
	}

	var controlPlane *models.ClusterNode
	for _, node := range nodes {
		if node.Role == models.NodeRoleControlPlane {
			controlPlane = node
			break
		}
	}

	if controlPlane == nil {
		return fmt.Errorf("no control-plane node found")
	}

	targets := []string{"employee", "customer"}
	return s.rollback(controlPlane.IPAddress, targets)
}

func (s *DeploymentService) rollback(controlPlaneIP string, targets []string) error {
	for _, target := range targets {
		cmd := fmt.Sprintf("k3s kubectl rollout undo deployment/cold-backend-%s -n default", target)
		s.ssh.Execute(nil, controlPlaneIP, cmd)
	}
	return nil
}

// GetJobStatus returns the status of an active deployment job
func (s *DeploymentService) GetJobStatus(historyID int) (*DeployJob, bool) {
	s.jobMutex.RLock()
	defer s.jobMutex.RUnlock()
	job, ok := s.jobs[historyID]
	return job, ok
}

// CancelDeployment cancels an active deployment
func (s *DeploymentService) CancelDeployment(historyID int) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	job, ok := s.jobs[historyID]
	if !ok {
		return fmt.Errorf("deployment job not found")
	}

	job.Cancel()
	delete(s.jobs, historyID)
	return nil
}

// Helper functions

func (s *DeploymentService) buildBinary(buildPath, buildCommand string) (string, error) {
	if buildCommand == "" {
		buildCommand = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-w -s' -o server ./cmd/server"
	}

	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s && %s", buildPath, buildCommand))
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (s *DeploymentService) buildDockerImage(buildPath, imageRepo, version string) (string, error) {
	dockerfile := fmt.Sprintf("%s/Dockerfile.ci", buildPath)

	// Check if Dockerfile.ci exists, fallback to Dockerfile
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		dockerfile = fmt.Sprintf("%s/Dockerfile", buildPath)
	}

	cmd := exec.Command("docker", "build",
		"-f", dockerfile,
		"-t", fmt.Sprintf("%s:%s", imageRepo, version),
		buildPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (s *DeploymentService) saveDockerImage(imageRepo, version string) error {
	tarFile := fmt.Sprintf("/tmp/cold-backend-%s.tar.gz", version)
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf("docker save %s:%s | gzip > %s", imageRepo, version, tarFile))
	return cmd.Run()
}

func (s *DeploymentService) distributeToNode(node *models.ClusterNode, tarFile, version string) error {
	// Get SSH key from default path or node config
	sshKeyPath := os.Getenv("SSH_KEY_PATH")
	if sshKeyPath == "" {
		sshKeyPath = "/etc/cold-backend/ssh/id_rsa"
	}

	var privateKey []byte
	if key, err := os.ReadFile(sshKeyPath); err == nil {
		privateKey = key
	}

	// Copy tarball
	if err := s.ssh.CopyFile(nil, node.IPAddress, node.SSHPort, node.SSHUser, privateKey, "", tarFile, tarFile); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	// Import image
	importCmd := fmt.Sprintf("gunzip -c %s | k3s ctr -n k8s.io images import - && rm -f %s", tarFile, tarFile)
	if _, err := s.ssh.Execute(nil, node.IPAddress, importCmd); err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	return nil
}
