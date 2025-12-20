package models

import (
	"time"
)

// NodeRole represents the role of a cluster node
type NodeRole string

const (
	NodeRoleControlPlane NodeRole = "control-plane"
	NodeRoleWorker       NodeRole = "worker"
	NodeRoleBackup       NodeRole = "backup"
)

// NodeStatus represents the provisioning status of a node
type NodeStatus string

const (
	NodeStatusPending    NodeStatus = "pending"
	NodeStatusConnecting NodeStatus = "connecting"
	NodeStatusInstalling NodeStatus = "installing"
	NodeStatusJoining    NodeStatus = "joining"
	NodeStatusReady      NodeStatus = "ready"
	NodeStatusFailed     NodeStatus = "failed"
	NodeStatusRemoved    NodeStatus = "removed"
)

// ClusterNode represents a node in the K3s cluster
type ClusterNode struct {
	ID            int        `json:"id"`
	IPAddress     string     `json:"ip_address"`
	Hostname      string     `json:"hostname"`
	Role          NodeRole   `json:"role"`
	Status        NodeStatus `json:"status"`
	SSHUser       string     `json:"ssh_user"`
	SSHPort       int        `json:"ssh_port"`
	SSHKeyID      *int       `json:"ssh_key_id,omitempty"`
	K3sVersion    string     `json:"k3s_version,omitempty"`
	OSInfo        string     `json:"os_info,omitempty"`
	CPUCores      int        `json:"cpu_cores,omitempty"`
	MemoryMB      int        `json:"memory_mb,omitempty"`
	DiskGB        int        `json:"disk_gb,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	ProvisionedAt *time.Time `json:"provisioned_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// InfraConfig represents a configuration key-value pair
type InfraConfig struct {
	ID          int       `json:"id"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
	IsSecret    bool      `json:"is_secret"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// SSHKey represents a stored SSH key
type SSHKey struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	PublicKey      string    `json:"public_key"`
	PrivateKeyPath string    `json:"private_key_path,omitempty"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
}

// NodeProvisionLog represents a provisioning step log entry
type NodeProvisionLog struct {
	ID         int        `json:"id"`
	NodeID     int        `json:"node_id"`
	Step       string     `json:"step"`
	Status     string     `json:"status"` // running, success, failed
	Message    string     `json:"message,omitempty"`
	Output     string     `json:"output,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// InfraActionLog represents an infrastructure action audit entry
type InfraActionLog struct {
	ID           int                    `json:"id"`
	UserID       int                    `json:"user_id"`
	Action       string                 `json:"action"`
	TargetType   string                 `json:"target_type,omitempty"`
	TargetID     string                 `json:"target_id,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Status       string                 `json:"status"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

// AddNodeRequest represents the request to add a new node
type AddNodeRequest struct {
	IPAddress  string   `json:"ip_address" validate:"required,ip"`
	SSHUser    string   `json:"ssh_user"`
	SSHPort    int      `json:"ssh_port"`
	SSHKey     string   `json:"ssh_key,omitempty"`     // Private key content
	SSHKeyID   int      `json:"ssh_key_id,omitempty"`  // Use stored key
	Password   string   `json:"password,omitempty"`    // For initial setup
	Role       NodeRole `json:"role"`
	Hostname   string   `json:"hostname,omitempty"`
	AutoSetup  bool     `json:"auto_setup"`            // If true, install K3s automatically
}

// NodeActionRequest represents a node management action request
type NodeActionRequest struct {
	Force bool `json:"force,omitempty"` // Force the action
}

// ConfigUpdateRequest represents a configuration update request
type ConfigUpdateRequest struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
}

// ProvisionProgress represents real-time provisioning progress
type ProvisionProgress struct {
	NodeID     int       `json:"node_id"`
	Step       string    `json:"step"`
	Progress   int       `json:"progress"` // 0-100
	Message    string    `json:"message"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// DatabaseRole represents the role of an external database
type DatabaseRole string

const (
	DatabaseRolePrimary  DatabaseRole = "primary"
	DatabaseRoleReplica  DatabaseRole = "replica"
	DatabaseRoleStandby  DatabaseRole = "standby"
	DatabaseRoleBackup   DatabaseRole = "backup"
)

// DatabaseStatus represents the status of an external database
type DatabaseStatus string

const (
	DatabaseStatusHealthy  DatabaseStatus = "healthy"
	DatabaseStatusDegraded DatabaseStatus = "degraded"
	DatabaseStatusFailed   DatabaseStatus = "failed"
	DatabaseStatusUnknown  DatabaseStatus = "unknown"
)

// ExternalDatabase represents an external PostgreSQL server
type ExternalDatabase struct {
	ID                    int             `json:"id"`
	Name                  string          `json:"name"`
	IPAddress             string          `json:"ip_address"`
	Port                  int             `json:"port"`
	DBName                string          `json:"db_name"`
	DBUser                string          `json:"db_user"`
	Role                  DatabaseRole    `json:"role"`
	Status                DatabaseStatus  `json:"status"`
	ReplicationSourceID   *int            `json:"replication_source_id,omitempty"`
	SSHUser               string          `json:"ssh_user"`
	SSHPort               int             `json:"ssh_port"`
	PGVersion             string          `json:"pg_version,omitempty"`
	ConnectionCount       int             `json:"connection_count,omitempty"`
	ReplicationLagSeconds float64         `json:"replication_lag_seconds,omitempty"`
	DiskUsagePercent      int             `json:"disk_usage_percent,omitempty"`
	LastBackupAt          *time.Time      `json:"last_backup_at,omitempty"`
	LastCheckedAt         *time.Time      `json:"last_checked_at,omitempty"`
	ErrorMessage          string          `json:"error_message,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// DeploymentConfig represents a deployment configuration
type DeploymentConfig struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	ImageRepo       string    `json:"image_repo"`
	CurrentVersion  string    `json:"current_version"`
	DeploymentName  string    `json:"deployment_name"`
	Namespace       string    `json:"namespace"`
	Replicas        int       `json:"replicas"`
	BuildCommand    string    `json:"build_command"`
	BuildContext    string    `json:"build_context"`
	DockerFile      string    `json:"docker_file"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusBuilding   DeploymentStatus = "building"
	DeploymentStatusDeploying  DeploymentStatus = "deploying"
	DeploymentStatusSuccess    DeploymentStatus = "success"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledback DeploymentStatus = "rolledback"
)

// DeploymentHistory represents a deployment history entry
type DeploymentHistory struct {
	ID              int              `json:"id"`
	DeploymentID    int              `json:"deployment_id"`
	Version         string           `json:"version"`
	PreviousVersion string           `json:"previous_version,omitempty"`
	DeployedBy      int              `json:"deployed_by"`
	Status          DeploymentStatus `json:"status"`
	BuildOutput     string           `json:"build_output,omitempty"`
	DeployOutput    string           `json:"deploy_output,omitempty"`
	ErrorMessage    string           `json:"error_message,omitempty"`
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
}

// AddDatabaseRequest represents a request to add an external database
type AddDatabaseRequest struct {
	Name      string       `json:"name" validate:"required"`
	IPAddress string       `json:"ip_address" validate:"required,ip"`
	Port      int          `json:"port"`
	DBName    string       `json:"db_name"`
	DBUser    string       `json:"db_user"`
	Role      DatabaseRole `json:"role"`
	SSHUser   string       `json:"ssh_user"`
	SSHPort   int          `json:"ssh_port"`
	Password  string       `json:"password,omitempty"` // For testing connection
}

// DeployRequest represents a request to deploy a new version
type DeployRequest struct {
	DeploymentID int    `json:"deployment_id" validate:"required"`
	Version      string `json:"version" validate:"required"`
	SkipBuild    bool   `json:"skip_build,omitempty"`  // Use existing image
	ForcePush    bool   `json:"force_push,omitempty"`  // Push to registry even if disabled
}
