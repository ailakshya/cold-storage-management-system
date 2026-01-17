package config

import "fmt"

// R2 Cloudflare configuration for disaster recovery
// These credentials are hardcoded for offline recovery scenarios
const (
	R2Endpoint   = "https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com"
	R2AccessKey  = "290bc63d7d6900dd2ca59751b7456899"
	R2SecretKey  = "038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"
	R2BucketName = "cold-db-backups"
	R2Region     = "auto"
)

// Environment prefix for R2 backups
// Values: "mac-mini-ha" (Mac Mini + Linux HA), "production-beta" (K3s), "poc" (test VMs), "local" (dev)
var R2BackupPrefix = "mac-mini-ha"

// GetR2BackupPrefix returns the backup prefix based on the connected database
func GetR2BackupPrefix() string {
	return R2BackupPrefix
}

// IsPOCEnvironment returns true if running in POC/test environment
func IsPOCEnvironment() bool {
	return R2BackupPrefix == "poc"
}

// IsMacMiniHAEnvironment returns true if running in Mac Mini HA environment
func IsMacMiniHAEnvironment() bool {
	return R2BackupPrefix == "mac-mini-ha"
}

// IsProductionEnvironment returns true if running in any production environment
func IsProductionEnvironment() bool {
	return R2BackupPrefix == "mac-mini-ha" || R2BackupPrefix == "production-beta"
}

// GetEnvironmentName returns human-readable environment name
func GetEnvironmentName() string {
	switch R2BackupPrefix {
	case "mac-mini-ha":
		return "Production (Mac Mini HA)"
	case "poc":
		return "POC"
	case "production-beta":
		return "Production (K3s)"
	case "local":
		return "Local Development"
	default:
		return "Unknown"
	}
}

// SetR2BackupPrefixFromDB sets the backup prefix based on the database host
func SetR2BackupPrefixFromDB(host string) {
	switch host {
	// Mac Mini HA cluster (Production)
	case "192.168.15.240", "192.168.15.241":
		R2BackupPrefix = "mac-mini-ha"
	// POC test VMs
	case "192.168.15.230", "192.168.15.231":
		R2BackupPrefix = "poc"
	// Legacy K3s production
	case "192.168.15.210":
		R2BackupPrefix = "production-beta"
	// Local development
	case "localhost", "/var/run/postgresql":
		R2BackupPrefix = "local"
	default:
		R2BackupPrefix = "mac-mini-ha"
	}
}

// Common passwords to try (CNPG may reset password from secret)
var CommonPasswords = []string{
	"SecurePostgresPassword123",
	"MetricsDB2025!", // Streaming replica on 195:5434
	"postgres",
	"", // Empty password - CNPG sometimes has no password set
}

// Database fallback configuration - will try all passwords for each host
// Order: Mac Mini Primary -> Linux Secondary -> POC VMs -> VIP-DB -> Backup -> Localhost
var DatabaseFallbacks = []DatabaseConfig{
	// Mac Mini HA (Production)
	{
		Name:     "Mac Mini Primary (192.168.15.240)",
		Host:     "192.168.15.240",
		Port:     5432,
		User:     "cold_user",
		Database: "cold_db",
	},
	{
		Name:     "Linux Secondary (192.168.15.241)",
		Host:     "192.168.15.241",
		Port:     5432,
		User:     "cold_user",
		Database: "cold_db",
	},
	// POC test environment
	{
		Name:     "POC Primary (192.168.15.230)",
		Host:     "192.168.15.230",
		Port:     5432,
		User:     "cold_user",
		Database: "cold_db",
	},
	{
		Name:     "POC Standby (192.168.15.231)",
		Host:     "192.168.15.231",
		Port:     5432,
		User:     "cold_user",
		Database: "cold_db",
	},
	// Legacy K3s production
	{
		Name:     "VIP-DB (Primary)",
		Host:     "192.168.15.210",
		Port:     5432,
		User:     "cold_user",
		Database: "cold_db",
	},
	{
		Name:     "Backup Server (192.168.15.195)",
		Host:     "192.168.15.195",
		Port:     5432,
		User:     "postgres",
		Database: "cold_db",
	},
	// Local fallback
	{
		Name:     "Localhost (Disaster Recovery)",
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "cold_db",
	},
	{
		Name:     "Localhost Unix Socket (Peer Auth)",
		Host:     "/var/run/postgresql", // Unix socket path for peer auth
		Port:     5432,
		User:     "postgres",
		Database: "cold_db",
		UsePeer:  true,
	},
}

type DatabaseConfig struct {
	Name     string
	Host     string
	Port     int
	User     string
	Password string // Will be set dynamically
	Database string
	UsePeer  bool   // Use Unix socket with peer auth (no password)
}

func (d DatabaseConfig) ConnectionString() string {
	if d.UsePeer {
		// Unix socket connection with peer auth
		return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
			d.Host, d.User, d.Database)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		d.Host, d.Port, d.User, d.Password, d.Database)
}

// ConnectionStringWithPassword returns connection string with specific password
func (d DatabaseConfig) ConnectionStringWithPassword(password string) string {
	if d.UsePeer {
		// Unix socket connection with peer auth - no password needed
		return fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable",
			d.Host, d.User, d.Database)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		d.Host, d.Port, d.User, password, d.Database)
}

// ConnectionURI returns a postgres:// URI format for psql command
func (d DatabaseConfig) ConnectionURI(password string) string {
	if d.UsePeer {
		// For psql with peer auth, just use the database name
		return fmt.Sprintf("postgresql:///%s?host=%s&user=%s",
			d.Database, d.Host, d.User)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, password, d.Host, d.Port, d.Database)
}
