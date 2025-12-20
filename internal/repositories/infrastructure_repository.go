package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"cold-backend/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InfrastructureRepository handles database operations for infrastructure management
type InfrastructureRepository struct {
	DB *pgxpool.Pool
}

// NewInfrastructureRepository creates a new infrastructure repository
func NewInfrastructureRepository(db *pgxpool.Pool) *InfrastructureRepository {
	return &InfrastructureRepository{DB: db}
}

// ============ Cluster Nodes ============

// CreateNode creates a new cluster node record
func (r *InfrastructureRepository) CreateNode(ctx context.Context, node *models.ClusterNode) error {
	query := `
		INSERT INTO cluster_nodes (ip_address, hostname, role, status, ssh_user, ssh_port, ssh_key_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	sshPort := node.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}

	sshUser := node.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}

	return r.DB.QueryRow(ctx, query,
		node.IPAddress, node.Hostname, node.Role, node.Status,
		sshUser, sshPort, node.SSHKeyID,
	).Scan(&node.ID, &node.CreatedAt, &node.UpdatedAt)
}

// GetNode retrieves a node by ID
func (r *InfrastructureRepository) GetNode(ctx context.Context, id int) (*models.ClusterNode, error) {
	query := `
		SELECT id, ip_address, hostname, role, status, ssh_user, ssh_port, ssh_key_id,
		       k3s_version, os_info, cpu_cores, memory_mb, disk_gb,
		       last_seen_at, provisioned_at, error_message, created_at, updated_at
		FROM cluster_nodes
		WHERE id = $1
	`

	node := &models.ClusterNode{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&node.ID, &node.IPAddress, &node.Hostname, &node.Role, &node.Status,
		&node.SSHUser, &node.SSHPort, &node.SSHKeyID,
		&node.K3sVersion, &node.OSInfo, &node.CPUCores, &node.MemoryMB, &node.DiskGB,
		&node.LastSeenAt, &node.ProvisionedAt, &node.ErrorMessage, &node.CreatedAt, &node.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// GetNodeByIP retrieves a node by IP address
func (r *InfrastructureRepository) GetNodeByIP(ctx context.Context, ipAddress string) (*models.ClusterNode, error) {
	query := `
		SELECT id, ip_address, hostname, role, status, ssh_user, ssh_port, ssh_key_id,
		       k3s_version, os_info, cpu_cores, memory_mb, disk_gb,
		       last_seen_at, provisioned_at, error_message, created_at, updated_at
		FROM cluster_nodes
		WHERE ip_address = $1
	`

	node := &models.ClusterNode{}
	err := r.DB.QueryRow(ctx, query, ipAddress).Scan(
		&node.ID, &node.IPAddress, &node.Hostname, &node.Role, &node.Status,
		&node.SSHUser, &node.SSHPort, &node.SSHKeyID,
		&node.K3sVersion, &node.OSInfo, &node.CPUCores, &node.MemoryMB, &node.DiskGB,
		&node.LastSeenAt, &node.ProvisionedAt, &node.ErrorMessage, &node.CreatedAt, &node.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return node, nil
}

// ListNodes retrieves all cluster nodes
func (r *InfrastructureRepository) ListNodes(ctx context.Context) ([]*models.ClusterNode, error) {
	query := `
		SELECT id, ip_address, hostname, role, status, ssh_user, ssh_port, ssh_key_id,
		       k3s_version, os_info, cpu_cores, memory_mb, disk_gb,
		       last_seen_at, provisioned_at, error_message, created_at, updated_at
		FROM cluster_nodes
		WHERE status != 'removed'
		ORDER BY role, ip_address
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*models.ClusterNode
	for rows.Next() {
		node := &models.ClusterNode{}
		err := rows.Scan(
			&node.ID, &node.IPAddress, &node.Hostname, &node.Role, &node.Status,
			&node.SSHUser, &node.SSHPort, &node.SSHKeyID,
			&node.K3sVersion, &node.OSInfo, &node.CPUCores, &node.MemoryMB, &node.DiskGB,
			&node.LastSeenAt, &node.ProvisionedAt, &node.ErrorMessage, &node.CreatedAt, &node.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// UpdateNodeStatus updates a node's status
func (r *InfrastructureRepository) UpdateNodeStatus(ctx context.Context, id int, status models.NodeStatus, errorMsg string) error {
	query := `
		UPDATE cluster_nodes
		SET status = $2, error_message = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, status, errorMsg)
	return err
}

// UpdateNodeInfo updates node hardware/software info
func (r *InfrastructureRepository) UpdateNodeInfo(ctx context.Context, id int, osInfo, k3sVersion string, cpuCores, memoryMB, diskGB int) error {
	query := `
		UPDATE cluster_nodes
		SET os_info = $2, k3s_version = $3, cpu_cores = $4, memory_mb = $5, disk_gb = $6,
		    last_seen_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, osInfo, k3sVersion, cpuCores, memoryMB, diskGB)
	return err
}

// SetNodeProvisioned marks a node as provisioned
func (r *InfrastructureRepository) SetNodeProvisioned(ctx context.Context, id int) error {
	query := `
		UPDATE cluster_nodes
		SET status = 'ready', provisioned_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// UpdateNodeLastSeen updates the last_seen_at timestamp
func (r *InfrastructureRepository) UpdateNodeLastSeen(ctx context.Context, id int) error {
	query := `
		UPDATE cluster_nodes
		SET last_seen_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// DeleteNode soft-deletes a node by setting status to removed
func (r *InfrastructureRepository) DeleteNode(ctx context.Context, id int) error {
	query := `
		UPDATE cluster_nodes
		SET status = 'removed', updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id)
	return err
}

// ============ Infrastructure Config ============

// GetConfig retrieves a configuration value by key
func (r *InfrastructureRepository) GetConfig(ctx context.Context, key string) (*models.InfraConfig, error) {
	query := `
		SELECT id, key, value, description, is_secret, updated_at, created_at
		FROM infra_config
		WHERE key = $1
	`

	config := &models.InfraConfig{}
	err := r.DB.QueryRow(ctx, query, key).Scan(
		&config.ID, &config.Key, &config.Value, &config.Description,
		&config.IsSecret, &config.UpdatedAt, &config.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetConfigValue retrieves just the value for a key
func (r *InfrastructureRepository) GetConfigValue(ctx context.Context, key string) (string, error) {
	var value string
	err := r.DB.QueryRow(ctx, "SELECT value FROM infra_config WHERE key = $1", key).Scan(&value)
	return value, err
}

// SetConfig sets or updates a configuration value
func (r *InfrastructureRepository) SetConfig(ctx context.Context, key, value, description string) error {
	query := `
		INSERT INTO infra_config (key, value, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, description = COALESCE(NULLIF($3, ''), infra_config.description), updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.DB.Exec(ctx, query, key, value, description)
	return err
}

// ListConfigs retrieves all configuration values
func (r *InfrastructureRepository) ListConfigs(ctx context.Context) ([]*models.InfraConfig, error) {
	query := `
		SELECT id, key, value, description, is_secret, updated_at, created_at
		FROM infra_config
		ORDER BY key
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.InfraConfig
	for rows.Next() {
		config := &models.InfraConfig{}
		err := rows.Scan(
			&config.ID, &config.Key, &config.Value, &config.Description,
			&config.IsSecret, &config.UpdatedAt, &config.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Mask secret values
		if config.IsSecret && config.Value != "" {
			config.Value = "********"
		}

		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// DeleteConfig deletes a configuration key
func (r *InfrastructureRepository) DeleteConfig(ctx context.Context, key string) error {
	_, err := r.DB.Exec(ctx, "DELETE FROM infra_config WHERE key = $1", key)
	return err
}

// ============ SSH Keys ============

// CreateSSHKey stores a new SSH key
func (r *InfrastructureRepository) CreateSSHKey(ctx context.Context, key *models.SSHKey) error {
	query := `
		INSERT INTO ssh_keys (name, public_key, private_key_path, fingerprint, is_default)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		key.Name, key.PublicKey, key.PrivateKeyPath, key.Fingerprint, key.IsDefault,
	).Scan(&key.ID, &key.CreatedAt)
}

// GetSSHKey retrieves an SSH key by ID
func (r *InfrastructureRepository) GetSSHKey(ctx context.Context, id int) (*models.SSHKey, error) {
	query := `
		SELECT id, name, public_key, private_key_path, fingerprint, is_default, created_at
		FROM ssh_keys
		WHERE id = $1
	`

	key := &models.SSHKey{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&key.ID, &key.Name, &key.PublicKey, &key.PrivateKeyPath,
		&key.Fingerprint, &key.IsDefault, &key.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// GetDefaultSSHKey retrieves the default SSH key
func (r *InfrastructureRepository) GetDefaultSSHKey(ctx context.Context) (*models.SSHKey, error) {
	query := `
		SELECT id, name, public_key, private_key_path, fingerprint, is_default, created_at
		FROM ssh_keys
		WHERE is_default = true
		LIMIT 1
	`

	key := &models.SSHKey{}
	err := r.DB.QueryRow(ctx, query).Scan(
		&key.ID, &key.Name, &key.PublicKey, &key.PrivateKeyPath,
		&key.Fingerprint, &key.IsDefault, &key.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// ListSSHKeys retrieves all SSH keys
func (r *InfrastructureRepository) ListSSHKeys(ctx context.Context) ([]*models.SSHKey, error) {
	query := `
		SELECT id, name, public_key, private_key_path, fingerprint, is_default, created_at
		FROM ssh_keys
		ORDER BY is_default DESC, name
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*models.SSHKey
	for rows.Next() {
		key := &models.SSHKey{}
		err := rows.Scan(
			&key.ID, &key.Name, &key.PublicKey, &key.PrivateKeyPath,
			&key.Fingerprint, &key.IsDefault, &key.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// ============ Provision Logs ============

// CreateProvisionLog creates a new provisioning log entry
func (r *InfrastructureRepository) CreateProvisionLog(ctx context.Context, log *models.NodeProvisionLog) error {
	query := `
		INSERT INTO node_provision_logs (node_id, step, status, message, output)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, started_at
	`
	return r.DB.QueryRow(ctx, query,
		log.NodeID, log.Step, log.Status, log.Message, log.Output,
	).Scan(&log.ID, &log.StartedAt)
}

// UpdateProvisionLog updates a provisioning log entry
func (r *InfrastructureRepository) UpdateProvisionLog(ctx context.Context, id int, status, message, output string) error {
	query := `
		UPDATE node_provision_logs
		SET status = $2, message = $3, output = $4, finished_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, status, message, output)
	return err
}

// GetProvisionLogs retrieves provisioning logs for a node
func (r *InfrastructureRepository) GetProvisionLogs(ctx context.Context, nodeID int) ([]*models.NodeProvisionLog, error) {
	query := `
		SELECT id, node_id, step, status, message, output, started_at, finished_at
		FROM node_provision_logs
		WHERE node_id = $1
		ORDER BY started_at
	`

	rows, err := r.DB.Query(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.NodeProvisionLog
	for rows.Next() {
		log := &models.NodeProvisionLog{}
		err := rows.Scan(
			&log.ID, &log.NodeID, &log.Step, &log.Status,
			&log.Message, &log.Output, &log.StartedAt, &log.FinishedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// ============ Action Logs ============

// CreateActionLog creates an infrastructure action audit log
func (r *InfrastructureRepository) CreateActionLog(ctx context.Context, log *models.InfraActionLog) error {
	detailsJSON, _ := json.Marshal(log.Details)

	query := `
		INSERT INTO infra_action_logs (user_id, action, target_type, target_id, details, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return r.DB.QueryRow(ctx, query,
		log.UserID, log.Action, log.TargetType, log.TargetID, detailsJSON, log.Status, log.ErrorMessage,
	).Scan(&log.ID, &log.CreatedAt)
}

// ListActionLogs retrieves infrastructure action logs
func (r *InfrastructureRepository) ListActionLogs(ctx context.Context, limit int) ([]*models.InfraActionLog, error) {
	if limit <= 0 {
		limit = 100
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, action, target_type, target_id, details, status, error_message, created_at
		FROM infra_action_logs
		ORDER BY created_at DESC
		LIMIT %d
	`, limit)

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.InfraActionLog
	for rows.Next() {
		log := &models.InfraActionLog{}
		var detailsJSON []byte
		err := rows.Scan(
			&log.ID, &log.UserID, &log.Action, &log.TargetType,
			&log.TargetID, &detailsJSON, &log.Status, &log.ErrorMessage, &log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &log.Details)
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// ============ Helpers ============

// GetK3sServerURL returns the K3s server URL from config
func (r *InfrastructureRepository) GetK3sServerURL(ctx context.Context) (string, error) {
	return r.GetConfigValue(ctx, "k3s_server_url")
}

// GetK3sToken returns the K3s cluster token from config
func (r *InfrastructureRepository) GetK3sToken(ctx context.Context) (string, error) {
	return r.GetConfigValue(ctx, "k3s_token")
}

// GetOffsiteDBConfig returns the offsite database configuration
func (r *InfrastructureRepository) GetOffsiteDBConfig(ctx context.Context) (host string, port string, err error) {
	host, err = r.GetConfigValue(ctx, "offsite_db_host")
	if err != nil {
		return "", "", err
	}
	port, err = r.GetConfigValue(ctx, "offsite_db_port")
	if err != nil {
		port = "5434"
	}
	return host, port, nil
}

// GetNodeCount returns the count of active nodes by role
func (r *InfrastructureRepository) GetNodeCount(ctx context.Context) (total, controlPlane, worker int, err error) {
	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE role = 'control-plane') as control_plane,
			COUNT(*) FILTER (WHERE role = 'worker') as worker
		FROM cluster_nodes
		WHERE status NOT IN ('removed', 'failed')
	`
	err = r.DB.QueryRow(ctx, query).Scan(&total, &controlPlane, &worker)
	return
}

// NodeExists checks if a node with the given IP already exists
func (r *InfrastructureRepository) NodeExists(ctx context.Context, ipAddress string) (bool, error) {
	var exists bool
	err := r.DB.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM cluster_nodes WHERE ip_address = $1 AND status != 'removed')",
		ipAddress,
	).Scan(&exists)
	return exists, err
}

// ============ External Databases ============

// CreateExternalDatabase creates a new external database record
func (r *InfrastructureRepository) CreateExternalDatabase(ctx context.Context, db *models.ExternalDatabase) error {
	query := `
		INSERT INTO external_databases (name, ip_address, port, db_name, db_user, role, status, replication_source_id, ssh_user, ssh_port)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	port := db.Port
	if port == 0 {
		port = 5432
	}
	sshPort := db.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}

	return r.DB.QueryRow(ctx, query,
		db.Name, db.IPAddress, port, db.DBName, db.DBUser,
		db.Role, db.Status, db.ReplicationSourceID, db.SSHUser, sshPort,
	).Scan(&db.ID, &db.CreatedAt, &db.UpdatedAt)
}

// GetExternalDatabase retrieves an external database by ID
func (r *InfrastructureRepository) GetExternalDatabase(ctx context.Context, id int) (*models.ExternalDatabase, error) {
	query := `
		SELECT id, name, ip_address, port, db_name, db_user, role, status, replication_source_id,
		       ssh_user, ssh_port, pg_version, connection_count, replication_lag_seconds,
		       disk_usage_percent, last_backup_at, last_checked_at, error_message, created_at, updated_at
		FROM external_databases
		WHERE id = $1
	`

	db := &models.ExternalDatabase{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&db.ID, &db.Name, &db.IPAddress, &db.Port, &db.DBName, &db.DBUser,
		&db.Role, &db.Status, &db.ReplicationSourceID, &db.SSHUser, &db.SSHPort,
		&db.PGVersion, &db.ConnectionCount, &db.ReplicationLagSeconds, &db.DiskUsagePercent,
		&db.LastBackupAt, &db.LastCheckedAt, &db.ErrorMessage, &db.CreatedAt, &db.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// ListExternalDatabases retrieves all external databases
func (r *InfrastructureRepository) ListExternalDatabases(ctx context.Context) ([]*models.ExternalDatabase, error) {
	query := `
		SELECT id, name, ip_address, port, db_name, db_user, role, status, replication_source_id,
		       ssh_user, ssh_port, pg_version, connection_count, replication_lag_seconds,
		       disk_usage_percent, last_backup_at, last_checked_at, error_message, created_at, updated_at
		FROM external_databases
		ORDER BY role, name
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []*models.ExternalDatabase
	for rows.Next() {
		db := &models.ExternalDatabase{}
		err := rows.Scan(
			&db.ID, &db.Name, &db.IPAddress, &db.Port, &db.DBName, &db.DBUser,
			&db.Role, &db.Status, &db.ReplicationSourceID, &db.SSHUser, &db.SSHPort,
			&db.PGVersion, &db.ConnectionCount, &db.ReplicationLagSeconds, &db.DiskUsagePercent,
			&db.LastBackupAt, &db.LastCheckedAt, &db.ErrorMessage, &db.CreatedAt, &db.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		databases = append(databases, db)
	}

	return databases, rows.Err()
}

// UpdateExternalDatabaseStatus updates an external database's status and metrics
func (r *InfrastructureRepository) UpdateExternalDatabaseStatus(ctx context.Context, id int, status models.DatabaseStatus,
	connCount int, replLag float64, diskUsage int, pgVersion, errorMsg string) error {
	query := `
		UPDATE external_databases
		SET status = $2, connection_count = $3, replication_lag_seconds = $4, disk_usage_percent = $5,
		    pg_version = $6, error_message = $7, last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, status, connCount, replLag, diskUsage, pgVersion, errorMsg)
	return err
}

// DeleteExternalDatabase deletes an external database record
func (r *InfrastructureRepository) DeleteExternalDatabase(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, "DELETE FROM external_databases WHERE id = $1", id)
	return err
}

// ============ Deployment Config ============

// GetDeploymentConfig retrieves a deployment configuration by ID
func (r *InfrastructureRepository) GetDeploymentConfig(ctx context.Context, id int) (*models.DeploymentConfig, error) {
	query := `
		SELECT id, name, image_repo, current_version, deployment_name, namespace, replicas,
		       build_command, build_context, docker_file, created_at, updated_at
		FROM deployment_config
		WHERE id = $1
	`

	config := &models.DeploymentConfig{}
	err := r.DB.QueryRow(ctx, query, id).Scan(
		&config.ID, &config.Name, &config.ImageRepo, &config.CurrentVersion,
		&config.DeploymentName, &config.Namespace, &config.Replicas,
		&config.BuildCommand, &config.BuildContext, &config.DockerFile,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// ListDeploymentConfigs retrieves all deployment configurations
func (r *InfrastructureRepository) ListDeploymentConfigs(ctx context.Context) ([]*models.DeploymentConfig, error) {
	query := `
		SELECT id, name, image_repo, current_version, deployment_name, namespace, replicas,
		       build_command, build_context, docker_file, created_at, updated_at
		FROM deployment_config
		ORDER BY name
	`

	rows, err := r.DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.DeploymentConfig
	for rows.Next() {
		config := &models.DeploymentConfig{}
		err := rows.Scan(
			&config.ID, &config.Name, &config.ImageRepo, &config.CurrentVersion,
			&config.DeploymentName, &config.Namespace, &config.Replicas,
			&config.BuildCommand, &config.BuildContext, &config.DockerFile,
			&config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// UpdateDeploymentVersion updates the current version of a deployment
func (r *InfrastructureRepository) UpdateDeploymentVersion(ctx context.Context, id int, version string) error {
	query := `
		UPDATE deployment_config
		SET current_version = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, version)
	return err
}

// ============ Deployment History ============

// CreateDeploymentHistory creates a new deployment history entry
func (r *InfrastructureRepository) CreateDeploymentHistory(ctx context.Context, history *models.DeploymentHistory) error {
	query := `
		INSERT INTO deployment_history (deployment_id, version, previous_version, deployed_by, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, started_at
	`
	return r.DB.QueryRow(ctx, query,
		history.DeploymentID, history.Version, history.PreviousVersion,
		history.DeployedBy, history.Status,
	).Scan(&history.ID, &history.StartedAt)
}

// UpdateDeploymentHistory updates a deployment history entry
func (r *InfrastructureRepository) UpdateDeploymentHistory(ctx context.Context, id int, status models.DeploymentStatus,
	buildOutput, deployOutput, errorMsg string) error {
	query := `
		UPDATE deployment_history
		SET status = $2, build_output = $3, deploy_output = $4, error_message = $5,
		    completed_at = CASE WHEN $2 IN ('success', 'failed', 'rolledback') THEN CURRENT_TIMESTAMP ELSE NULL END
		WHERE id = $1
	`
	_, err := r.DB.Exec(ctx, query, id, status, buildOutput, deployOutput, errorMsg)
	return err
}

// GetDeploymentHistory retrieves deployment history for a deployment
func (r *InfrastructureRepository) GetDeploymentHistory(ctx context.Context, deploymentID, limit int) ([]*models.DeploymentHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	query := fmt.Sprintf(`
		SELECT id, deployment_id, version, previous_version, deployed_by, status,
		       build_output, deploy_output, error_message, started_at, completed_at
		FROM deployment_history
		WHERE deployment_id = $1
		ORDER BY started_at DESC
		LIMIT %d
	`, limit)

	rows, err := r.DB.Query(ctx, query, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*models.DeploymentHistory
	for rows.Next() {
		h := &models.DeploymentHistory{}
		err := rows.Scan(
			&h.ID, &h.DeploymentID, &h.Version, &h.PreviousVersion,
			&h.DeployedBy, &h.Status, &h.BuildOutput, &h.DeployOutput,
			&h.ErrorMessage, &h.StartedAt, &h.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

// GetLatestDeployment retrieves the latest successful deployment
func (r *InfrastructureRepository) GetLatestDeployment(ctx context.Context, deploymentID int) (*models.DeploymentHistory, error) {
	query := `
		SELECT id, deployment_id, version, previous_version, deployed_by, status,
		       build_output, deploy_output, error_message, started_at, completed_at
		FROM deployment_history
		WHERE deployment_id = $1 AND status = 'success'
		ORDER BY started_at DESC
		LIMIT 1
	`

	h := &models.DeploymentHistory{}
	err := r.DB.QueryRow(ctx, query, deploymentID).Scan(
		&h.ID, &h.DeploymentID, &h.Version, &h.PreviousVersion,
		&h.DeployedBy, &h.Status, &h.BuildOutput, &h.DeployOutput,
		&h.ErrorMessage, &h.StartedAt, &h.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return h, nil
}
