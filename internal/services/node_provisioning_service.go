package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cold-backend/internal/infra"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

// NodeProvisioningService handles node provisioning operations
type NodeProvisioningService struct {
	repo       *repositories.InfrastructureRepository
	ssh        *infra.SSHService
	k3s        *infra.K3sService

	// Active provisioning jobs
	jobs     map[int]*ProvisionJob
	jobMutex sync.RWMutex
}

// ProvisionJob represents an active provisioning job
type ProvisionJob struct {
	NodeID     int
	Status     chan models.ProvisionProgress
	Cancel     context.CancelFunc
	StartedAt  time.Time
	LastUpdate models.ProvisionProgress
}

// NewNodeProvisioningService creates a new node provisioning service
func NewNodeProvisioningService(repo *repositories.InfrastructureRepository) *NodeProvisioningService {
	// Get default SSH key path from environment or use default
	sshKeyPath := os.Getenv("SSH_KEY_PATH")
	if sshKeyPath == "" {
		sshKeyPath = "/etc/cold-backend/ssh/id_rsa"
	}

	sshService := infra.NewSSHService("root", sshKeyPath)
	k3sService := infra.NewK3sService(sshService)

	return &NodeProvisioningService{
		repo: repo,
		ssh:  sshService,
		k3s:  k3sService,
		jobs: make(map[int]*ProvisionJob),
	}
}

// AddNode adds a new node and optionally starts provisioning
func (s *NodeProvisioningService) AddNode(ctx context.Context, req *models.AddNodeRequest, userID int) (*models.ClusterNode, error) {
	// Check if node already exists
	exists, err := s.repo.NodeExists(ctx, req.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to check node existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("node with IP %s already exists", req.IPAddress)
	}

	// Set defaults
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.Role == "" {
		req.Role = models.NodeRoleWorker
	}

	// Create node record
	node := &models.ClusterNode{
		IPAddress: req.IPAddress,
		Hostname:  req.Hostname,
		Role:      req.Role,
		Status:    models.NodeStatusPending,
		SSHUser:   req.SSHUser,
		SSHPort:   req.SSHPort,
	}

	if err := s.repo.CreateNode(ctx, node); err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}

	// Log the action
	s.logAction(ctx, userID, "add_node", "node", req.IPAddress, map[string]interface{}{
		"role":     req.Role,
		"hostname": req.Hostname,
	})

	// Start provisioning if requested
	if req.AutoSetup {
		go s.ProvisionNode(context.Background(), node.ID, req.SSHKey, req.Password, userID)
	}

	return node, nil
}

// ProvisionNode provisions a node with K3s
func (s *NodeProvisioningService) ProvisionNode(ctx context.Context, nodeID int, sshKey, password string, userID int) error {
	// Get node
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Create provisioning job
	jobCtx, cancel := context.WithCancel(ctx)
	statusChan := make(chan models.ProvisionProgress, 10)

	job := &ProvisionJob{
		NodeID:    nodeID,
		Status:    statusChan,
		Cancel:    cancel,
		StartedAt: time.Now(),
	}

	s.jobMutex.Lock()
	s.jobs[nodeID] = job
	s.jobMutex.Unlock()

	defer func() {
		s.jobMutex.Lock()
		delete(s.jobs, nodeID)
		s.jobMutex.Unlock()
		close(statusChan)
	}()

	// Update status to connecting
	s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusConnecting, "")

	// Get SSH key
	var privateKey []byte
	if sshKey != "" {
		privateKey = []byte(sshKey)
	} else {
		// Try to get default SSH key
		key, err := s.repo.GetDefaultSSHKey(ctx)
		if err == nil && key.PrivateKeyPath != "" {
			privateKey, _ = os.ReadFile(key.PrivateKeyPath)
		}
	}

	// Build K3s install options
	opts := &infra.K3sInstallOptions{
		Host:       node.IPAddress,
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
		Password:   password,
		Role:       infra.NodeRole(node.Role),
		NodeName:   node.Hostname,
	}

	// Get K3s server URL and token
	serverURL, _ := s.repo.GetK3sServerURL(ctx)
	token, _ := s.repo.GetK3sToken(ctx)

	if serverURL != "" {
		opts.ServerURL = serverURL
	}
	if token != "" {
		opts.Token = token
	}

	// Check prerequisites
	s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusConnecting, "")
	s.logProvisionStep(ctx, nodeID, "Checking prerequisites", "running", "")

	if err := s.k3s.CheckPrerequisites(jobCtx, opts); err != nil {
		s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusFailed, err.Error())
		s.logProvisionStep(ctx, nodeID, "Checking prerequisites", "failed", err.Error())
		s.logAction(ctx, userID, "provision_node", "node", node.IPAddress, map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
		})
		return err
	}

	s.logProvisionStep(ctx, nodeID, "Checking prerequisites", "success", "Node is ready for provisioning")

	// Update status to installing
	s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusInstalling, "")

	// Start K3s installation based on role
	var installErr error
	infraStatusChan := make(chan infra.ProvisionStatus, 10)

	go func() {
		for status := range infraStatusChan {
			s.logProvisionStep(ctx, nodeID, status.Step, "running", status.Message)
			job.LastUpdate = models.ProvisionProgress{
				NodeID:   nodeID,
				Step:     status.Step,
				Progress: status.Progress,
				Message:  status.Message,
				Error:    status.Error,
			}
		}
	}()

	if node.Role == models.NodeRoleControlPlane {
		installErr = s.k3s.InstallK3sServer(jobCtx, opts, infraStatusChan)
	} else {
		installErr = s.k3s.InstallK3sAgent(jobCtx, opts, infraStatusChan)
	}

	close(infraStatusChan)

	if installErr != nil {
		s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusFailed, installErr.Error())
		s.logProvisionStep(ctx, nodeID, "Installation", "failed", installErr.Error())
		s.logAction(ctx, userID, "provision_node", "node", node.IPAddress, map[string]interface{}{
			"status": "failed",
			"error":  installErr.Error(),
		})
		return installErr
	}

	// Get node info
	s.repo.UpdateNodeStatus(ctx, nodeID, models.NodeStatusJoining, "")
	s.logProvisionStep(ctx, nodeID, "Joining cluster", "running", "Waiting for node to join cluster")

	// Give it a moment to join
	time.Sleep(10 * time.Second)

	// Collect node info
	osInfo, _ := s.ssh.GetOSInfo(ctx, node.IPAddress, node.SSHPort, node.SSHUser, privateKey, password)
	s.repo.UpdateNodeInfo(ctx, nodeID, osInfo, "", 0, 0, 0)

	// Mark as provisioned
	s.repo.SetNodeProvisioned(ctx, nodeID)
	s.logProvisionStep(ctx, nodeID, "Complete", "success", "Node provisioned successfully")

	s.logAction(ctx, userID, "provision_node", "node", node.IPAddress, map[string]interface{}{
		"status": "success",
		"role":   node.Role,
	})

	return nil
}

// GetProvisionStatus returns the current provisioning status for a node
func (s *NodeProvisioningService) GetProvisionStatus(nodeID int) (*models.ProvisionProgress, bool) {
	s.jobMutex.RLock()
	defer s.jobMutex.RUnlock()

	if job, ok := s.jobs[nodeID]; ok {
		return &job.LastUpdate, true
	}
	return nil, false
}

// CancelProvisioning cancels an active provisioning job
func (s *NodeProvisioningService) CancelProvisioning(nodeID int) error {
	s.jobMutex.Lock()
	defer s.jobMutex.Unlock()

	if job, ok := s.jobs[nodeID]; ok {
		job.Cancel()
		delete(s.jobs, nodeID)
		return nil
	}
	return fmt.Errorf("no active provisioning job for node %d", nodeID)
}

// RemoveNode removes a node from the cluster
func (s *NodeProvisioningService) RemoveNode(ctx context.Context, nodeID int, force bool, userID int) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Get SSH credentials
	var privateKey []byte
	key, err := s.repo.GetDefaultSSHKey(ctx)
	if err == nil && key.PrivateKeyPath != "" {
		privateKey, _ = os.ReadFile(key.PrivateKeyPath)
	}

	// Remove K3s from the node
	opts := &infra.K3sInstallOptions{
		Host:       node.IPAddress,
		Port:       node.SSHPort,
		User:       node.SSHUser,
		PrivateKey: privateKey,
	}

	if err := s.k3s.RemoveNode(ctx, opts); err != nil && !force {
		return fmt.Errorf("failed to remove K3s: %w", err)
	}

	// Mark node as removed
	s.repo.DeleteNode(ctx, nodeID)

	s.logAction(ctx, userID, "remove_node", "node", node.IPAddress, map[string]interface{}{
		"force": force,
	})

	return nil
}

// RebootNode reboots a node
func (s *NodeProvisioningService) RebootNode(ctx context.Context, nodeID int, userID int) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Get SSH credentials
	var privateKey []byte
	key, err := s.repo.GetDefaultSSHKey(ctx)
	if err == nil && key.PrivateKeyPath != "" {
		privateKey, _ = os.ReadFile(key.PrivateKeyPath)
	}

	if err := s.k3s.RebootNode(ctx, node.IPAddress, node.SSHPort, node.SSHUser, privateKey, ""); err != nil {
		return fmt.Errorf("failed to reboot node: %w", err)
	}

	s.logAction(ctx, userID, "reboot_node", "node", node.IPAddress, nil)

	return nil
}

// TestNodeConnection tests SSH connectivity to a node
func (s *NodeProvisioningService) TestNodeConnection(ctx context.Context, ipAddress string, sshUser string, sshPort int, sshKey, password string) error {
	if sshUser == "" {
		sshUser = "root"
	}
	if sshPort == 0 {
		sshPort = 22
	}

	var privateKey []byte
	if sshKey != "" {
		privateKey = []byte(sshKey)
	}

	return s.ssh.TestConnection(ctx, ipAddress, sshPort, sshUser, privateKey, password)
}

// DrainNode drains pods from a node
func (s *NodeProvisioningService) DrainNode(ctx context.Context, nodeID int, userID int) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	// Execute kubectl drain command
	result, err := s.ssh.Execute(ctx, node.IPAddress, fmt.Sprintf(
		"kubectl drain %s --ignore-daemonsets --delete-emptydir-data --force --timeout=300s",
		node.Hostname,
	))
	if err != nil {
		return fmt.Errorf("drain failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("drain failed: %s", result.Stderr)
	}

	s.logAction(ctx, userID, "drain_node", "node", node.IPAddress, nil)

	return nil
}

// CordonNode marks a node as unschedulable
func (s *NodeProvisioningService) CordonNode(ctx context.Context, nodeID int, userID int) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	result, err := s.ssh.Execute(ctx, node.IPAddress, fmt.Sprintf("kubectl cordon %s", node.Hostname))
	if err != nil {
		return fmt.Errorf("cordon failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("cordon failed: %s", result.Stderr)
	}

	s.logAction(ctx, userID, "cordon_node", "node", node.IPAddress, nil)

	return nil
}

// UncordonNode marks a node as schedulable
func (s *NodeProvisioningService) UncordonNode(ctx context.Context, nodeID int, userID int) error {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("node not found: %w", err)
	}

	result, err := s.ssh.Execute(ctx, node.IPAddress, fmt.Sprintf("kubectl uncordon %s", node.Hostname))
	if err != nil {
		return fmt.Errorf("uncordon failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("uncordon failed: %s", result.Stderr)
	}

	s.logAction(ctx, userID, "uncordon_node", "node", node.IPAddress, nil)

	return nil
}

// GetNodeLogs retrieves system logs from a node
func (s *NodeProvisioningService) GetNodeLogs(ctx context.Context, nodeID int, lines int) (string, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("node not found: %w", err)
	}

	if lines <= 0 {
		lines = 100
	}

	// Get SSH credentials
	var privateKey []byte
	key, err := s.repo.GetDefaultSSHKey(ctx)
	if err == nil && key.PrivateKeyPath != "" {
		privateKey, _ = os.ReadFile(key.PrivateKeyPath)
	}

	result, err := s.ssh.ExecuteWithConfig(ctx, node.IPAddress, node.SSHPort, node.SSHUser, privateKey, "",
		fmt.Sprintf("journalctl -n %d --no-pager", lines))
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}

	return result.Stdout, nil
}

// GetNodes returns all cluster nodes
func (s *NodeProvisioningService) GetNodes(ctx context.Context) ([]*models.ClusterNode, error) {
	return s.repo.ListNodes(ctx)
}

// GetNode returns a single cluster node
func (s *NodeProvisioningService) GetNode(ctx context.Context, id int) (*models.ClusterNode, error) {
	return s.repo.GetNode(ctx, id)
}

// GetNodeByIP returns a node by IP address
func (s *NodeProvisioningService) GetNodeByIP(ctx context.Context, ip string) (*models.ClusterNode, error) {
	return s.repo.GetNodeByIP(ctx, ip)
}

// GetProvisionLogs returns provisioning logs for a node
func (s *NodeProvisioningService) GetProvisionLogs(ctx context.Context, nodeID int) ([]*models.NodeProvisionLog, error) {
	return s.repo.GetProvisionLogs(ctx, nodeID)
}

// UpdateConfig updates an infrastructure configuration value
func (s *NodeProvisioningService) UpdateConfig(ctx context.Context, key, value string, userID int) error {
	if err := s.repo.SetConfig(ctx, key, value, ""); err != nil {
		return err
	}

	s.logAction(ctx, userID, "update_config", "config", key, map[string]interface{}{
		"value": maskSecretValue(key, value),
	})

	return nil
}

// GetConfigs returns all infrastructure configurations
func (s *NodeProvisioningService) GetConfigs(ctx context.Context) ([]*models.InfraConfig, error) {
	return s.repo.ListConfigs(ctx)
}

// GetConfig returns a single configuration value
func (s *NodeProvisioningService) GetConfig(ctx context.Context, key string) (string, error) {
	return s.repo.GetConfigValue(ctx, key)
}

// Helper functions

func (s *NodeProvisioningService) logProvisionStep(ctx context.Context, nodeID int, step, status, message string) {
	log := &models.NodeProvisionLog{
		NodeID:  nodeID,
		Step:    step,
		Status:  status,
		Message: message,
	}
	s.repo.CreateProvisionLog(ctx, log)
}

func (s *NodeProvisioningService) logAction(ctx context.Context, userID int, action, targetType, targetID string, details map[string]interface{}) {
	log := &models.InfraActionLog{
		UserID:     userID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		Status:     "success",
	}
	s.repo.CreateActionLog(ctx, log)
}

func maskSecretValue(key, value string) string {
	secretKeys := []string{"token", "password", "secret", "key"}
	for _, sk := range secretKeys {
		if strings.Contains(strings.ToLower(key), sk) {
			return "********"
		}
	}
	return value
}
