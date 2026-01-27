package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"cold-backend/internal/config"
	"cold-backend/migrations"
	"cold-backend/templates"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5"
)

type SetupHandler struct {
	templates      *template.Template
	isRecoveryMode bool
	connStr        string
	backupDir      string
}

func NewSetupHandler(backupDir string) *SetupHandler {
	tmpl, err := template.ParseFS(templates.FS, "setup.html")
	if err != nil {
		// Template might not exist yet during development
		tmpl = template.New("setup")
	}
	// Ensure backup directory exists
	if backupDir != "" {
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			log.Printf("[SetupHandler] Warning: failed to create backup dir %s: %v", backupDir, err)
		}
	}
	return &SetupHandler{
		templates: tmpl,
		backupDir: backupDir,
	}
}

// SetupPage shows the initial setup page
func (h *SetupHandler) SetupPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates.FS, "setup.html")
	if err != nil {
		http.Error(w, "Setup template not found", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// TestConnection tests a database connection
func (h *SetupHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		req.User, req.Password, req.Host, req.Port, req.Database)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	defer conn.Close(ctx)

	// Test query
	var result int
	err = conn.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Connection successful!",
	})
}

// SaveConfig saves the database configuration
func (h *SetupHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create .env file
	envContent := fmt.Sprintf(`DB_HOST=%s
DB_PORT=%d
DB_USER=%s
DB_PASSWORD=%s
DB_NAME=%s
JWT_SECRET=cold-backend-jwt-secret-2025
`, req.Host, req.Port, req.User, req.Password, req.Database)

	err := os.WriteFile(".env", []byte(envContent), 0600)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save configuration: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration saved! Restarting server...",
	})

	// Trigger restart
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0) // Exit so systemd/docker can restart
	}()
}

// ListBackups lists available backups from R2 (returns latest 50 sorted by date)
func (h *SetupHandler) ListBackups(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Create S3 client for R2
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to configure S3 client: " + err.Error(),
		})
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Collect all objects using pagination
	var allObjects []types.Object
	var continuationToken *string

	for {
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(config.R2BucketName),
			Prefix:            aws.String("base/"),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to list backups: " + err.Error(),
			})
			return
		}

		allObjects = append(allObjects, result.Contents...)

		if !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Sort by LastModified descending (newest first)
	sort.Slice(allObjects, func(i, j int) bool {
		return allObjects[i].LastModified.After(*allObjects[j].LastModified)
	})

	// Take only the latest 50 backups
	limit := 50
	if len(allObjects) < limit {
		limit = len(allObjects)
	}

	var backups []map[string]interface{}
	for _, obj := range allObjects[:limit] {
		backups = append(backups, map[string]interface{}{
			"key":          *obj.Key,
			"size":         *obj.Size,
			"lastModified": obj.LastModified.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"backups":     backups,
		"total_count": len(allObjects),
		"showing":     limit,
	})
}

// RestoreFromR2 restores database from R2 backup
func (h *SetupHandler) RestoreFromR2(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		User      string `json:"user"`
		Password  string `json:"password"`
		Database  string `json:"database"`
		BackupKey string `json:"backup_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// Create S3 client for R2
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to configure S3 client: " + err.Error(),
		})
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Download backup file
	backupKey := req.BackupKey
	if backupKey == "" {
		// Get latest backup
		result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket: aws.String(config.R2BucketName),
			Prefix: aws.String("base/"),
		})
		if err != nil || len(result.Contents) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "No backups found in R2",
			})
			return
		}
		// Get most recent
		var latest *string
		var latestTime time.Time
		for _, obj := range result.Contents {
			if obj.LastModified.After(latestTime) {
				latestTime = *obj.LastModified
				latest = obj.Key
			}
		}
		backupKey = *latest
	}

	// Download the backup
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(backupKey),
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to download backup: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), "cold_backup.sql")
	f, err := os.Create(tmpFile)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to create temp file: " + err.Error(),
		})
		return
	}
	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save backup: " + err.Error(),
		})
		return
	}

	// Restore using psql
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		req.User, req.Password, req.Host, req.Port, req.Database)

	// First, drop all tables to avoid duplicate key errors
	log.Println("[Restore] Cleaning database before restore...")
	cleanupSQL := `
DO $$
DECLARE
    r RECORD;
BEGIN
    -- Disable triggers
    SET session_replication_role = 'replica';

    -- Drop all tables in public schema
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;

    -- Re-enable triggers
    SET session_replication_role = 'origin';
END $$;
`
	cleanCmd := exec.Command("psql", connStr, "-c", cleanupSQL)
	cleanOutput, cleanErr := cleanCmd.CombinedOutput()
	if cleanErr != nil {
		log.Printf("[Restore] Warning: cleanup failed: %v - %s", cleanErr, string(cleanOutput))
	} else {
		log.Println("[Restore] Database cleaned successfully")
	}

	// Run schema creation from embedded migration file
	log.Println("[Restore] Creating database schema...")
	schemaSQL, err := migrations.FS.ReadFile("001_complete_schema.sql")
	if err != nil {
		log.Printf("[Restore] Warning: could not read schema file: %v", err)
	} else {
		// Write schema to temp file and execute
		schemaTmpFile := "/tmp/cold_schema.sql"
		if err := os.WriteFile(schemaTmpFile, schemaSQL, 0644); err != nil {
			log.Printf("[Restore] Warning: could not write schema file: %v", err)
		} else {
			schemaCmd := exec.Command("psql", connStr, "-f", schemaTmpFile)
			schemaOutput, schemaErr := schemaCmd.CombinedOutput()
			os.Remove(schemaTmpFile)
			if schemaErr != nil {
				log.Printf("[Restore] Warning: schema creation had issues: %v - %s", schemaErr, string(schemaOutput))
			} else {
				log.Println("[Restore] Schema created successfully")
			}
		}
	}

	// Now restore data from backup
	cmd := exec.Command("psql", connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	// Log restore output for debugging
	log.Printf("[Restore] psql output: %s", string(output))

	os.Remove(tmpFile) // Cleanup

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to restore: " + err.Error() + "\nOutput: " + string(output),
		})
		return
	}

	// Check if output contains PostgreSQL errors (not just any "ERROR" substring)
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR:") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Restore completed with errors:\n" + outputStr,
		})
		return
	}

	// Save config
	envContent := fmt.Sprintf(`DB_HOST=%s
DB_PORT=%d
DB_USER=%s
DB_PASSWORD=%s
DB_NAME=%s
JWT_SECRET=cold-backend-jwt-secret-2025
`, req.Host, req.Port, req.User, req.Password, req.Database)

	os.WriteFile(".env", []byte(envContent), 0600)

	// Set skip recovery marker so server runs normally after restart
	os.WriteFile("/tmp/.cold_skip_recovery", []byte("1"), 0644)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Database restored successfully! Restarting server...",
	})

	// Trigger restart - use syscall.Exec to replace current process
	go restartServer()
}

// restartServer restarts the server by replacing the current process
func restartServer() {
	time.Sleep(2 * time.Second)

	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		log.Printf("[Restart] Failed to get executable path: %v, falling back to exit", err)
		os.Exit(0)
		return
	}

	log.Printf("[Restart] Restarting server: %s", executable)

	// Replace current process with new instance
	err = syscall.Exec(executable, os.Args, os.Environ())
	if err != nil {
		log.Printf("[Restart] Failed to exec: %v, falling back to exit", err)
		os.Exit(0)
	}
}

// AutoRestoreFromR2 automatically restores from R2 if database is empty after migrations.
// This is called from main.go after migrations run for disaster recovery automation.
// Returns true if restore was performed, false if DB already has data or restore failed.
func AutoRestoreFromR2(connStr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log.Println("[AutoRestore] Checking if database needs restoration from R2...")

	// Use pgx to check if users table is empty
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Printf("[AutoRestore] Failed to connect: %v", err)
		return false
	}
	defer conn.Close(ctx)

	// Check if users table has any rows
	var userCount int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		log.Printf("[AutoRestore] Failed to check users table: %v", err)
		return false
	}

	if userCount > 0 {
		log.Printf("[AutoRestore] Database has %d users, skipping restore", userCount)
		return false
	}

	log.Println("[AutoRestore] Database is empty, starting automatic restore from R2...")

	// Create S3 client for R2
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		log.Printf("[AutoRestore] Failed to configure S3 client: %v", err)
		return false
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Get latest backup using pagination (R2 may have 1000s of backups)
	var latestKey string
	var latestTime time.Time
	var continuationToken *string
	totalBackups := 0

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(config.R2BucketName),
			Prefix:            aws.String("base/"),
			ContinuationToken: continuationToken,
		}

		result, err := client.ListObjectsV2(ctx, input)
		if err != nil {
			log.Printf("[AutoRestore] Failed to list backups: %v", err)
			return false
		}

		for _, obj := range result.Contents {
			totalBackups++
			if obj.LastModified != nil && obj.LastModified.After(latestTime) {
				latestTime = *obj.LastModified
				latestKey = *obj.Key
			}
		}

		if result.IsTruncated != nil && *result.IsTruncated {
			continuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	if latestKey == "" {
		log.Println("[AutoRestore] No backups found in R2")
		return false
	}

	log.Printf("[AutoRestore] Scanned %d backups in R2", totalBackups)

	log.Printf("[AutoRestore] Found latest backup: %s (%s)", latestKey, latestTime.Format(time.RFC3339))

	// Download the backup
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.R2BucketName),
		Key:    aws.String(latestKey),
	})
	if err != nil {
		log.Printf("[AutoRestore] Failed to download backup: %v", err)
		return false
	}
	defer resp.Body.Close()

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), "cold_auto_restore.sql")
	f, err := os.Create(tmpFile)
	if err != nil {
		log.Printf("[AutoRestore] Failed to create temp file: %v", err)
		return false
	}

	bytesWritten, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		log.Printf("[AutoRestore] Failed to save backup: %v", err)
		os.Remove(tmpFile)
		return false
	}

	log.Printf("[AutoRestore] Downloaded backup: %.2f KB", float64(bytesWritten)/1024)

	// Restore using psql
	cmd := exec.Command("psql", connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	os.Remove(tmpFile) // Cleanup

	if err != nil {
		log.Printf("[AutoRestore] psql failed: %v\nOutput: %s", err, string(output))
		return false
	}

	// Check for PostgreSQL errors
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR:") {
		log.Printf("[AutoRestore] Restore completed with errors:\n%s", outputStr)
		return false
	}

	// Verify restore by counting users again
	conn2, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Printf("[AutoRestore] Failed to verify restore: %v", err)
		return true // Still return true, restore might have worked
	}
	defer conn2.Close(ctx)

	err = conn2.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		log.Printf("[AutoRestore] Failed to count users after restore: %v", err)
		return true
	}

	log.Printf("[AutoRestore] ✓ Restore complete! %d users restored from backup", userCount)
	return true
}

// CheckR2Connection tests R2 connectivity
func (h *SetupHandler) CheckR2Connection(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.R2AccessKey,
			config.R2SecretKey,
			"",
		)),
		awsconfig.WithRegion(config.R2Region),
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(config.R2Endpoint)
	})

	// Try to list bucket
	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(config.R2BucketName),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "R2 connection successful!",
	})
}

// UploadRestore handles file upload and database restore from .sql file
func (h *SetupHandler) UploadRestore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to parse form: " + err.Error(),
		})
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No file uploaded: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".sql") {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Only .sql files are allowed",
		})
		return
	}

	log.Printf("[UploadRestore] Received file: %s (%d bytes)", header.Filename, header.Size)

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), "cold_upload_"+time.Now().Format("20060102_150405")+".sql")
	f, err := os.Create(tmpFile)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to create temp file: " + err.Error(),
		})
		return
	}

	bytesWritten, err := io.Copy(f, file)
	f.Close()
	if err != nil {
		os.Remove(tmpFile)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save file: " + err.Error(),
		})
		return
	}

	log.Printf("[UploadRestore] Saved to temp file: %s (%.2f KB)", tmpFile, float64(bytesWritten)/1024)

	// Also save to local backup directory if configured
	if h.backupDir != "" {
		timestamp := time.Now().Format("20060102_150405")
		backupFilename := fmt.Sprintf("cold_manual_restore_%s.sql", timestamp)
		backupPath := filepath.Join(h.backupDir, backupFilename)

		// Read temp file again
		data, err := os.ReadFile(tmpFile)
		if err == nil {
			if err := os.WriteFile(backupPath, data, 0644); err == nil {
				log.Printf("[UploadRestore] Saved copy to local backup: %s", backupPath)
			} else {
				log.Printf("[UploadRestore] Warning: failed to save local backup copy: %v", err)
			}
		} else {
			log.Printf("[UploadRestore] Warning: failed to read temp file for local copy: %v", err)
		}
	}

	// Get DB connection params from form
	host := r.FormValue("host")
	port := r.FormValue("port")
	user := r.FormValue("user")
	password := r.FormValue("password")
	database := r.FormValue("database")

	if host == "" || port == "" || user == "" || database == "" {
		os.Remove(tmpFile)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Missing database connection parameters",
		})
		return
	}

	// Build connection string
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
		user, password, host, port, database)

	log.Printf("[UploadRestore] Restoring to %s:%s/%s", host, port, database)

	// First, drop all tables to avoid duplicate key errors
	log.Println("[UploadRestore] Cleaning database before restore...")
	cleanupSQL := `
DO $$
DECLARE
    r RECORD;
BEGIN
    -- Disable triggers
    SET session_replication_role = 'replica';

    -- Drop all tables in public schema
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;

    -- Re-enable triggers
    SET session_replication_role = 'origin';
END $$;
`
	cleanCmd := exec.Command("psql", connStr, "-c", cleanupSQL)
	cleanOutput, cleanErr := cleanCmd.CombinedOutput()
	if cleanErr != nil {
		log.Printf("[UploadRestore] Warning: cleanup failed: %v - %s", cleanErr, string(cleanOutput))
	} else {
		log.Println("[UploadRestore] Database cleaned successfully")
	}

	// Run schema creation from embedded migration file
	log.Println("[UploadRestore] Creating database schema...")
	schemaSQL, schemaReadErr := migrations.FS.ReadFile("001_complete_schema.sql")
	if schemaReadErr != nil {
		log.Printf("[UploadRestore] Warning: could not read schema file: %v", schemaReadErr)
	} else {
		schemaTmpFile := "/tmp/cold_schema.sql"
		if writeErr := os.WriteFile(schemaTmpFile, schemaSQL, 0644); writeErr != nil {
			log.Printf("[UploadRestore] Warning: could not write schema file: %v", writeErr)
		} else {
			schemaCmd := exec.Command("psql", connStr, "-f", schemaTmpFile)
			schemaOutput, schemaErr := schemaCmd.CombinedOutput()
			os.Remove(schemaTmpFile)
			if schemaErr != nil {
				log.Printf("[UploadRestore] Warning: schema creation had issues: %v - %s", schemaErr, string(schemaOutput))
			} else {
				log.Println("[UploadRestore] Schema created successfully")
			}
		}
	}

	// Execute psql restore
	cmd := exec.Command("psql", connStr, "-f", tmpFile)
	output, err := cmd.CombinedOutput()

	os.Remove(tmpFile) // Cleanup

	if err != nil {
		log.Printf("[UploadRestore] psql error: %v\nOutput: %s", err, string(output))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Restore failed: " + err.Error() + "\n" + string(output),
		})
		return
	}

	// Check for PostgreSQL errors in output
	outputStr := string(output)
	if strings.Contains(outputStr, "ERROR:") {
		log.Printf("[UploadRestore] Restore completed with errors:\n%s", outputStr)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Restore completed with errors:\n" + outputStr,
		})
		return
	}

	log.Printf("[UploadRestore] Restore successful!")

	// Save config
	envContent := fmt.Sprintf(`DB_HOST=%s
DB_PORT=%s
DB_USER=%s
DB_PASSWORD=%s
DB_NAME=%s
JWT_SECRET=cold-backend-jwt-secret-2025
`, host, port, user, password, database)

	os.WriteFile(".env", []byte(envContent), 0600)

	// Set skip recovery marker so server runs normally after restart
	os.WriteFile("/tmp/.cold_skip_recovery", []byte("1"), 0644)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Database restored successfully! Restarting server...",
	})

	// Trigger restart - use syscall.Exec to replace current process
	go restartServer()
}

// SetRecoveryMode configures the handler for disaster recovery mode
func (h *SetupHandler) SetRecoveryMode(isRecovery bool, connStr string) {
	h.isRecoveryMode = isRecovery
	h.connStr = connStr
}

// RecoveryPage shows the recovery page when VIP and 195 are down but localhost is available
func (h *SetupHandler) RecoveryPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templates.FS, "recovery.html")
	if err != nil {
		// Fallback to setup.html if recovery.html doesn't exist
		tmpl, err = template.ParseFS(templates.FS, "setup.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
	}
	tmpl.Execute(w, map[string]interface{}{
		"IsRecoveryMode": true,
		"ConnStr":        h.connStr,
	})
}

// ContinueWithCurrent continues with the current database (skip restore)
func (h *SetupHandler) ContinueWithCurrent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Just trigger a restart - the app will connect to localhost and run normally
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Continuing with current database. Restarting server...",
	})

	// Set environment variable to skip recovery mode on next startup
	// This is done by creating a marker file
	os.WriteFile("/tmp/.cold_skip_recovery", []byte("1"), 0644)

	// Trigger restart - use syscall.Exec to replace current process
	go restartServer()
}
