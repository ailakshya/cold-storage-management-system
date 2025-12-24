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

// Common passwords to try (CNPG may reset password from secret)
var CommonPasswords = []string{
	"SecurePostgresPassword123",
	"MetricsDB2025!", // Streaming replica on 195:5434
	"postgres",
	"", // Empty password - CNPG sometimes has no password set
}

// Database fallback configuration - will try all passwords for each host
// Order: K8s VIP -> Backup server -> Localhost (for disaster recovery)
var DatabaseFallbacks = []DatabaseConfig{
	{
		Name:     "K8s Cluster (Primary)",
		Host:     "192.168.15.200",
		Port:     5432,
		User:     "postgres",
		Database: "cold_db",
	},
	{
		Name:     "Local Backup (192.168.15.195)",
		Host:     "192.168.15.195",
		Port:     5434,
		User:     "postgres",
		Database: "cold_db",
	},
	{
		Name:     "Localhost (Disaster Recovery)",
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Database: "cold_db",
	},
}

type DatabaseConfig struct {
	Name     string
	Host     string
	Port     int
	User     string
	Password string // Will be set dynamically
	Database string
}

func (d DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, d.Password, d.Host, d.Port, d.Database)
}

// ConnectionStringWithPassword returns connection string with specific password
func (d DatabaseConfig) ConnectionStringWithPassword(password string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		d.User, password, d.Host, d.Port, d.Database)
}
