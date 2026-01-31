package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"cold-backend/installer"
	"cold-backend/internal/auth"
	"cold-backend/internal/cache"
	"cold-backend/internal/config"
	"cold-backend/internal/database"
	"cold-backend/internal/db"
	"cold-backend/internal/g"
	"cold-backend/internal/handlers"
	"cold-backend/internal/health"
	h "cold-backend/internal/http"
	"cold-backend/internal/middleware"
	"cold-backend/internal/monitoring"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"
	"cold-backend/internal/sms"
	"cold-backend/migrations"
	"cold-backend/static"
)

// startSetupMode starts the server in setup mode when no database is available
func startSetupMode(cfg *config.Config) {
	setupHandler := handlers.NewSetupHandler(cfg.BackupDir)

	mux := http.NewServeMux()

	// Setup routes
	mux.HandleFunc("/", setupHandler.SetupPage)
	mux.HandleFunc("/setup", setupHandler.SetupPage)
	mux.HandleFunc("/setup/test", setupHandler.TestConnection)
	mux.HandleFunc("/setup/save", setupHandler.SaveConfig)
	mux.HandleFunc("/setup/r2-check", setupHandler.CheckR2Connection)
	mux.HandleFunc("/setup/backups", setupHandler.ListBackups)
	mux.HandleFunc("/setup/restore", setupHandler.RestoreFromR2)
	mux.HandleFunc("/setup/upload-restore", setupHandler.UploadRestore)

	// Serve static files from embedded filesystem
	staticFS, _ := fs.Sub(static.FS, ".")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Setup mode running on %s", addr)
	log.Println("Open your browser to configure database connection")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Setup server failed: %v", err)
	}
}

// startRecoveryMode starts the server in recovery mode when VIP and backup servers are down
// but localhost PostgreSQL is available. Shows restore options instead of auto-restoring.
func startRecoveryMode(cfg *config.Config, connStr string) {
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║  RECOVERY MODE - Primary servers unreachable               ║")
	log.Println("║                                                            ║")
	log.Println("║  VIP-DB (192.168.15.210) - FAILED                          ║")
	log.Println("║  Backup Server (192.168.15.195) - FAILED                   ║")
	log.Println("║  Localhost PostgreSQL - CONNECTED                          ║")
	log.Println("║                                                            ║")
	log.Println("║  Open browser to restore from backup:                      ║")
	log.Println("║    - Restore from R2 cloud backup                          ║")
	log.Println("║    - Upload .sql backup file                               ║")
	log.Println("║    - Continue with current database                        ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	setupHandler := handlers.NewSetupHandler(cfg.BackupDir)
	setupHandler.SetRecoveryMode(true, connStr)

	mux := http.NewServeMux()

	// Recovery routes (same as setup, but in recovery mode)
	mux.HandleFunc("/", setupHandler.RecoveryPage)
	mux.HandleFunc("/recovery", setupHandler.RecoveryPage)
	mux.HandleFunc("/setup/r2-check", setupHandler.CheckR2Connection)
	mux.HandleFunc("/setup/backups", setupHandler.ListBackups)
	mux.HandleFunc("/setup/restore", setupHandler.RestoreFromR2)
	mux.HandleFunc("/setup/upload-restore", setupHandler.UploadRestore)
	mux.HandleFunc("/setup/continue", setupHandler.ContinueWithCurrent)

	// Serve static files from embedded filesystem
	staticFS, _ := fs.Sub(static.FS, ".")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Recovery mode running on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Recovery server failed: %v", err)
	}
}

// connectTimescaleDB connects to the TimescaleDB metrics database
func connectTimescaleDB() *pgxpool.Pool {
	// TimescaleDB connection string from environment or default
	// Using METRICS_DB_* prefix to avoid Kubernetes service env var collision
	host := os.Getenv("METRICS_DB_HOST")
	if host == "" {
		host = "timescaledb.default.svc.cluster.local" // K8s service DNS
	}
	port := os.Getenv("METRICS_DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("METRICS_DB_USER")
	if user == "" {
		user = "metrics"
	}
	password := os.Getenv("METRICS_DB_PASSWORD")
	if password == "" {
		log.Printf("[TimescaleDB] METRICS_DB_PASSWORD not set, skipping metrics DB connection")
		return nil
	}
	database := os.Getenv("METRICS_DB_NAME")
	if database == "" {
		database = "metrics_db"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.PathEscape(user), url.PathEscape(password), host, port, database)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[TimescaleDB] Failed to parse config: %v", err)
		return nil
	}

	// Connection pool settings
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Printf("[TimescaleDB] Failed to connect: %v", err)
		return nil
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		log.Printf("[TimescaleDB] Failed to ping: %v", err)
		pool.Close()
		return nil
	}

	log.Println("[TimescaleDB] Connected successfully")
	return pool
}

// autoInstallPostgreSQL installs PostgreSQL and creates database when running as root
// This is called automatically when all database connections fail
func autoInstallPostgreSQL() {
	log.Println("[AutoInstall] Installing PostgreSQL...")

	// Install PostgreSQL
	cmd := exec.Command("bash", "-c", `
		if command -v psql &> /dev/null; then
			echo "PostgreSQL already installed"
			# Make sure it's running
			systemctl start postgresql 2>/dev/null || true
		else
			apt update -qq && apt install -y postgresql postgresql-contrib -qq
			systemctl enable postgresql
			systemctl start postgresql
			echo "PostgreSQL installed"
		fi
	`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[AutoInstall] Warning: %v", err)
	}

	// Wait for PostgreSQL to be ready
	time.Sleep(3 * time.Second)

	// Set password for postgres user and create database
	log.Println("[AutoInstall] Configuring database...")
	cmd = exec.Command("bash", "-c", `
		# Set a known password for postgres user so we can connect via TCP
		sudo -u postgres psql -c "ALTER USER postgres PASSWORD 'SecurePostgresPassword123';"

		# Create cold_user (used in migrations for GRANT statements)
		sudo -u postgres psql -c "CREATE USER cold_user WITH PASSWORD 'SecurePostgresPassword123';" 2>/dev/null || true

		# Create database if not exists
		if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw cold_db; then
			echo "Database 'cold_db' already exists"
		else
			sudo -u postgres psql -c "CREATE DATABASE cold_db OWNER postgres;"
			echo "Database 'cold_db' created"
		fi

		# Grant cold_user access to cold_db
		sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE cold_db TO cold_user;"

		# Enable password auth for localhost (modify pg_hba.conf)
		PG_HBA=$(sudo -u postgres psql -t -c "SHOW hba_file;" | xargs)
		if [ -f "$PG_HBA" ]; then
			# Check if already configured
			if ! grep -q "host.*cold_db.*127.0.0.1" "$PG_HBA"; then
				# Add password auth for localhost before the first "host" line
				sudo sed -i '/^host/i host    cold_db         postgres        127.0.0.1/32            scram-sha-256' "$PG_HBA"
				sudo sed -i '/^host/i host    cold_db         postgres        ::1/128                 scram-sha-256' "$PG_HBA"
				sudo sed -i '/^host/i host    cold_db         cold_user       127.0.0.1/32            scram-sha-256' "$PG_HBA"
				# Reload PostgreSQL
				sudo systemctl reload postgresql
				echo "Configured password authentication"
			fi
		fi
	`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[AutoInstall] Warning: %v", err)
	}

	// Wait for reload
	time.Sleep(2 * time.Second)
	log.Println("[AutoInstall] PostgreSQL ready, reconnecting...")
}

// runInstall extracts and runs the embedded install.sh script
func runInstall() {
	// Check if running as root
	if os.Geteuid() != 0 {
		log.Fatal("Please run with sudo: sudo ./server --install")
	}

	// Extract embedded install.sh to temp file
	scriptData, err := installer.FS.ReadFile("install.sh")
	if err != nil {
		log.Fatalf("Failed to read embedded install script: %v", err)
	}

	// Get current binary path to pass to script
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	// Write script to temp location
	tmpScript := "/tmp/cold_install.sh"
	if err := os.WriteFile(tmpScript, scriptData, 0755); err != nil {
		log.Fatalf("Failed to write install script: %v", err)
	}
	defer os.Remove(tmpScript)

	// Create a temp directory with the binary for the script to find
	tmpDir := "/tmp/cold_install_tmp"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Copy binary to temp dir so install.sh can find it
	binData, err := os.ReadFile(execPath)
	if err != nil {
		log.Fatalf("Failed to read binary: %v", err)
	}
	if err := os.WriteFile(tmpDir+"/server", binData, 0755); err != nil {
		log.Fatalf("Failed to copy binary: %v", err)
	}

	// Also copy install.sh to temp dir
	if err := os.WriteFile(tmpDir+"/install.sh", scriptData, 0755); err != nil {
		log.Fatalf("Failed to copy install script: %v", err)
	}

	// Run the install script from temp dir
	cmd := exec.Command("bash", tmpDir+"/install.sh")
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		log.Fatalf("Install script failed: %v", err)
	}

	os.Exit(0)
}

func main() {
	// Parse command-line flags
	mode := flag.String("mode", "employee", "Server mode: employee or customer")
	port := flag.Int("port", 0, "Server port (overrides config)")
	install := flag.Bool("install", false, "Install PostgreSQL, create database, and setup systemd service")
	flag.Parse()

	// Run install if requested
	if *install {
		runInstall()
		return
	}

	// Load configuration
	cfg := config.Load()

	// Override port if specified
	if *port != 0 {
		cfg.Server.Port = *port
	} else {
		// Set default ports based on mode
		if *mode == "customer" {
			cfg.Server.Port = 8081
		}
		// Employee mode uses config.yaml port (8080)
	}

	// Try cascading database connection: VIP → 195 → localhost → Unix socket
	// If all fail and running as root, install PostgreSQL automatically
	pool, connectedTo, connStr, isDisasterRecovery := db.TryConnectWithFallback()
	if pool == nil {
		// Check if running as root - if so, auto-install PostgreSQL
		if os.Geteuid() == 0 {
			log.Println("╔════════════════════════════════════════════════════════════╗")
			log.Println("║  NO DATABASE - INSTALLING POSTGRESQL AUTOMATICALLY         ║")
			log.Println("╚════════════════════════════════════════════════════════════╝")
			autoInstallPostgreSQL()
			// Try connecting again after install
			pool, connectedTo, connStr, isDisasterRecovery = db.TryConnectWithFallback()
		}

		if pool == nil {
			log.Println("╔════════════════════════════════════════════════════════════╗")
			log.Println("║  NO DATABASE AVAILABLE - ENTERING SETUP MODE               ║")
			log.Println("║                                                            ║")
			log.Println("║  Run as root for auto-install: sudo ./server               ║")
			log.Println("║  Or open browser to configure manually                     ║")
			log.Println("╚════════════════════════════════════════════════════════════╝")
			startSetupMode(cfg)
			return // Will never reach here (startSetupMode blocks)
		}
	}
	log.Printf("Connected to database: %s", connectedTo)

	// If this is a disaster recovery scenario (VIP and 195 failed, localhost connected),
	// enter recovery mode to let user choose restore option instead of auto-restoring
	// Skip if user previously chose to continue with current database
	if isDisasterRecovery {
		// Check if user chose to skip recovery mode
		if _, err := os.Stat("/tmp/.cold_skip_recovery"); err == nil {
			log.Println("[Recovery] Skipping recovery mode (user chose to continue with current database)")
			os.Remove("/tmp/.cold_skip_recovery") // Clean up marker
		} else {
			pool.Close() // Close the pool before entering recovery mode
			startRecoveryMode(cfg, connStr)
			return // Will never reach here (startRecoveryMode blocks)
		}
	}

	defer pool.Close()

	// Initialize Redis cache (optional - graceful fallback if unavailable)
	if err := cache.Init(); err != nil {
		log.Printf("[Redis] Cache unavailable: %v (login will use bcrypt only)", err)
	} else {
		log.Println("[Redis] Cache connected successfully")
	}

	// Run database migrations
	// This automatically creates all required tables on startup
	// Uses embedded migrations for standalone binary operation
	log.Println("Running database migrations...")
	migrator := database.NewMigratorWithFS(pool, migrations.FS, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := migrator.RunMigrations(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize health checker
	healthChecker := health.NewHealthChecker(pool)

	// Start monitoring dashboard server in background
	go monitoring.NewMonitoringServer(pool, 9090).Start()

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(cfg)

	// Initialize repositories
	userRepo := repositories.NewUserRepository(pool)
	customerRepo := repositories.NewCustomerRepository(pool)
	entryRepo := repositories.NewEntryRepository(pool)
	entryEventRepo := repositories.NewEntryEventRepository(pool)
	roomEntryRepo := repositories.NewRoomEntryRepository(pool)
	roomEntryGatarRepo := repositories.NewRoomEntryGatarRepository(pool)
	systemSettingRepo := repositories.NewSystemSettingRepository(pool)
	rentPaymentRepo := repositories.NewRentPaymentRepository(pool)
	gatePassRepo := repositories.NewGatePassRepository(pool)
	invoiceRepo := repositories.NewInvoiceRepository(pool)
	loginLogRepo := repositories.NewLoginLogRepository(pool)
	roomEntryEditLogRepo := repositories.NewRoomEntryEditLogRepository(pool)
	entryEditLogRepo := repositories.NewEntryEditLogRepository(pool)
	entryManagementLogRepo := repositories.NewEntryManagementLogRepository(pool)
	adminActionLogRepo := repositories.NewAdminActionLogRepository(pool)
	gatePassPickupRepo := repositories.NewGatePassPickupRepository(pool)
	gatePassMediaRepo := repositories.NewGatePassMediaRepository(pool)
	roomEntryMediaRepo := repositories.NewRoomEntryMediaRepository(pool)
	guardEntryRepo := repositories.NewGuardEntryRepository(pool)
	tokenColorRepo := repositories.NewTokenColorRepository(pool)
	ledgerRepo := repositories.NewLedgerRepository(pool)
	debtRequestRepo := repositories.NewDebtRequestRepository(pool)
	familyMemberRepo := repositories.NewFamilyMemberRepository(pool)
	onlineTransactionRepo := repositories.NewOnlineTransactionRepository(pool)
	pendingSettingChangeRepo := repositories.NewPendingSettingChangeRepository(pool)
	totpRepo := repositories.NewTOTPRepository(pool)

	// Initialize middleware (needed for both modes)
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, userRepo)
	operationModeMiddleware := middleware.NewOperationModeMiddleware(systemSettingRepo)
	corsMiddleware := middleware.NewCORS(cfg)
	pageHandler := handlers.NewPageHandler()
	healthHandler := handlers.NewHealthHandler(healthChecker)

	var handler http.Handler

	if *mode == "customer" {
		log.Println("Starting in CUSTOMER PORTAL mode")

		// Initialize OTP repository and SMS service
		otpRepo := repositories.NewOTPRepository(pool)

		// Initialize customer activity log repository
		customerActivityLogRepo := repositories.NewCustomerActivityLogRepository(pool)

		// Use Fast2SMS for production, fallback to MockSMS if API key not set
		fast2smsAPIKey := os.Getenv("FAST2SMS_API_KEY")
		var smsService sms.SMSProvider
		if fast2smsAPIKey != "" {
			log.Println("Using Fast2SMS for OTP delivery")
			smsService = sms.NewFast2SMSService(fast2smsAPIKey)
		} else {
			log.Println("WARNING: FAST2SMS_API_KEY not set, using MockSMS (OTP will only print to logs)")
			smsService = sms.NewMockSMSService()
		}

		// Initialize OTP service with configurable settings and activity logging
		otpService := services.NewOTPService(otpRepo, customerRepo, smsService)
		otpService.SetSettingRepo(systemSettingRepo)
		otpService.SetActivityLogRepo(customerActivityLogRepo)

		// Initialize customer portal service
		customerPortalService := services.NewCustomerPortalService(
			customerRepo,
			entryRepo,
			roomEntryRepo,
			gatePassRepo,
			rentPaymentRepo,
			systemSettingRepo,
			gatePassPickupRepo,
			ledgerRepo,
		)

		// Initialize customer portal handler
		customerPortalHandler := handlers.NewCustomerPortalHandler(
			otpService,
			customerPortalService,
			jwtManager,
		)

		// Initialize Razorpay service and handler for online payments
		razorpayService := services.NewRazorpayService(
			cfg.Razorpay.KeyID,
			cfg.Razorpay.KeySecret,
			cfg.Razorpay.WebhookSecret,
			onlineTransactionRepo,
			rentPaymentRepo,
			ledgerRepo,
			customerRepo,
			systemSettingRepo,
		)
		razorpayHandler := handlers.NewRazorpayHandler(razorpayService, customerRepo)

		// Create customer router
		router := h.NewCustomerRouter(customerPortalHandler, pageHandler, healthHandler, authMiddleware, razorpayHandler)

		// Wrap with panic recovery and metrics middleware
		handler = middleware.PanicRecovery(middleware.MetricsMiddleware(corsMiddleware(router)))

	} else if *mode == "website" {
		log.Println("Starting in WEBSITE mode")

		// Serve static files correctly at /static/
		// Files are in ./static directory
		fs := http.FileServer(http.Dir("./static"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))

		// Serve index.html for root path only
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.ServeFile(w, r, "./static/index.html")
				return
			}
			http.NotFound(w, r)
		})

		handler = corsMiddleware(http.DefaultServeMux)
		cfg.Server.Port = 8082 // Override port for website mode

	} else {
		log.Println("Starting in EMPLOYEE mode")

		// ... existing employee mode code ...
		// Initialize services (employee mode)
		userService := services.NewUserService(userRepo, jwtManager)
		customerService := services.NewCustomerService(customerRepo)
		entryService := services.NewEntryService(entryRepo, customerRepo, entryEventRepo)
		entryService.SetSettingRepo(systemSettingRepo)     // Wire SettingRepo for skip thock ranges
		entryService.SetFamilyMemberRepo(familyMemberRepo) // Wire FamilyMemberRepo for family member auto-assign
		printerService := services.NewPrinterService()
		printerHandler := handlers.NewPrinterHandler(printerService)
		roomEntryService := services.NewRoomEntryService(roomEntryRepo, roomEntryGatarRepo, entryRepo, entryEventRepo, printerService, roomEntryMediaRepo)
		systemSettingService := services.NewSystemSettingService(systemSettingRepo)
		rentPaymentService := services.NewRentPaymentService(rentPaymentRepo)
		invoiceService := services.NewInvoiceService(invoiceRepo)
		gatePassService := services.NewGatePassService(gatePassRepo, entryRepo, entryEventRepo, gatePassPickupRepo, roomEntryRepo, gatePassMediaRepo)
		ledgerService := services.NewLedgerService(ledgerRepo)
		debtService := services.NewDebtService(debtRequestRepo, ledgerService)

		// Initialize SMS logging and notification service
		smsLogRepo := repositories.NewSMSLogRepository(pool)

		// Initialize SMS service for employee mode (for bulk SMS, notifications)
		fast2smsAPIKey := os.Getenv("FAST2SMS_API_KEY")
		var baseSMSService sms.SMSProvider
		if fast2smsAPIKey != "" {
			baseSMSService = sms.NewFast2SMSService(fast2smsAPIKey)
		} else {
			baseSMSService = sms.NewMockSMSService()
		}

		// Create unified messaging service (WhatsApp-first with SMS fallback)
		employeeSMSService := sms.NewUnifiedMessagingService(baseSMSService)
		employeeSMSService.SetLogRepository(smsLogRepo)

		// Load WhatsApp settings from database
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Check if WhatsApp is enabled
			enabledSetting, _ := systemSettingRepo.Get(ctx, "whatsapp_enabled")
			if enabledSetting != nil && enabledSetting.SettingValue == "true" {
				providerSetting, _ := systemSettingRepo.Get(ctx, "whatsapp_provider")
				apiKeySetting, _ := systemSettingRepo.Get(ctx, "whatsapp_api_key")
				phoneIDSetting, _ := systemSettingRepo.Get(ctx, "whatsapp_phone_number_id")
				costSetting, _ := systemSettingRepo.Get(ctx, "whatsapp_cost_per_msg")

				provider := "generic"
				if providerSetting != nil {
					provider = providerSetting.SettingValue
				}

				apiKey := ""
				if apiKeySetting != nil {
					apiKey = apiKeySetting.SettingValue
				}

				phoneID := ""
				if phoneIDSetting != nil {
					phoneID = phoneIDSetting.SettingValue
				}

				cost := 0.08
				if costSetting != nil {
					if c, err := strconv.ParseFloat(costSetting.SettingValue, 64); err == nil {
						cost = c
					}
				}

				if apiKey != "" {
					employeeSMSService.SetWhatsAppConfig(&sms.WhatsAppConfig{
						Enabled:       true,
						Provider:      provider,
						APIKey:        apiKey,
						PhoneNumberID: phoneID,
						CostPerMsg:    cost,
					})
					log.Printf("WhatsApp enabled with provider: %s", provider)
				}
			}
		}()

		// Initialize notification service for transaction SMS
		notificationService := services.NewNotificationService(employeeSMSService, systemSettingRepo)

		// Initialize handlers (employee mode)
		userHandler := handlers.NewUserHandler(userService, adminActionLogRepo)
		authHandler := handlers.NewAuthHandler(userService, loginLogRepo)
		customerHandler := handlers.NewCustomerHandler(customerService, entryManagementLogRepo)
		customerHandler.SetLedgerRepo(ledgerRepo) // Cascade phone changes to ledger
		entryHandler := handlers.NewEntryHandler(entryService, entryEditLogRepo, entryManagementLogRepo, adminActionLogRepo)
		roomEntryHandler := handlers.NewRoomEntryHandler(roomEntryService, roomEntryEditLogRepo)
		entryEventHandler := handlers.NewEntryEventHandler(entryEventRepo)
		systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingService)
		entryHandler.SetSettingService(systemSettingService) // Wire SettingService for skip thock ranges
		rentPaymentHandler := handlers.NewRentPaymentHandler(rentPaymentService, ledgerService, adminActionLogRepo)
		rentPaymentHandler.SetNotificationService(notificationService)
		rentPaymentHandler.SetCustomerService(customerService)
		rentPaymentHandler.SetEntryRepo(entryRepo) // For family member name validation
		invoiceHandler := handlers.NewInvoiceHandler(invoiceService)
		loginLogHandler := handlers.NewLoginLogHandler(loginLogRepo)
		// Set OTP repo for customer login logs in admin panel
		otpRepo := repositories.NewOTPRepository(pool)
		loginLogHandler.SetOTPRepo(otpRepo)
		roomEntryEditLogHandler := handlers.NewRoomEntryEditLogHandler(roomEntryEditLogRepo)
		entryEditLogHandler := handlers.NewEntryEditLogHandler(entryEditLogRepo)
		entryManagementLogHandler := handlers.NewEntryManagementLogHandler(entryManagementLogRepo)
		adminActionLogHandler := handlers.NewAdminActionLogHandler(adminActionLogRepo)
		gatePassHandler := handlers.NewGatePassHandler(gatePassService, adminActionLogRepo)

		// Initialize media sync service (3-2-1 backup: Local + NAS + R2)
		mediaSyncRepo := repositories.NewMediaSyncRepository(pool)
		var r2MediaBackend, nasMediaBackend *services.S3Backend

		// Determine bulk storage root (same logic as file manager)
		bulkRoot := "/mass-pool/shared"
		if _, err := os.Stat("/mass-pool"); os.IsNotExist(err) {
			home, _ := os.UserHomeDir()
			bulkRoot = filepath.Join(home, "cold-storage", "shared")
		}

		// Initialize R2 media backend
		if backend, err := services.NewR2MediaBackend(context.Background()); err != nil {
			log.Printf("[MediaSync] R2 backend unavailable: %v", err)
		} else {
			r2MediaBackend = backend
			log.Println("[MediaSync] R2 media backend initialized")
		}

		// Initialize NAS backend (MinIO on TrueNAS)
		nasCfg := config.LoadNASConfig()
		if nasCfg.Enabled {
			if backend, err := services.NewNASBackend(context.Background(), nasCfg); err != nil {
				log.Printf("[MediaSync] NAS backend unavailable: %v", err)
			} else {
				nasMediaBackend = backend
				log.Println("[MediaSync] NAS media backend initialized")
			}
		} else {
			log.Println("[MediaSync] NAS not configured (set NAS_S3_ENDPOINT to enable)")
		}

		mediaSyncService := services.NewMediaSyncService(mediaSyncRepo, r2MediaBackend, nasMediaBackend, bulkRoot)
		mediaSyncService.Start()

		// Wire sync service into media handlers
		roomEntryHandler.SetSyncService(mediaSyncService)
		gatePassHandler.SetSyncService(mediaSyncService)

		// Initialize guard entry service and handler
		guardEntryService := services.NewGuardEntryService(guardEntryRepo)
		guardEntryHandler := handlers.NewGuardEntryHandler(guardEntryService, adminActionLogRepo)

		// Initialize token color handler
		tokenColorHandler := handlers.NewTokenColorHandler(tokenColorRepo)

		// Initialize family member handler
		familyMemberHandler := handlers.NewFamilyMemberHandler(familyMemberRepo)

		// Initialize season repository
		seasonRequestRepo := repositories.NewSeasonRequestRepository(pool)

		// Initialize TimescaleDB connection for metrics (optional - degrades gracefully)
		var timescaleStore *monitoring.TimescaleStore
		var apiLoggingMiddleware *middleware.APILoggingMiddleware
		var monitoringService *monitoring.MonitoringService

		tsdbPool := connectTimescaleDB()
		if tsdbPool != nil {
			defer tsdbPool.Close()
		}

		// Always initialize TimescaleStore (handles nil pool with in-memory fallback)
		timescaleStore = monitoring.NewTimescaleStore(tsdbPool)

		// Always enable API logging (will use memory or DB)
		apiLoggingMiddleware = middleware.NewAPILoggingMiddleware(timescaleStore)

		if tsdbPool != nil {
			log.Println("[Monitoring] Initializing TimescaleDB monitoring components...")

			// Start system metrics collection service
			monitoringService = monitoring.NewMonitoringService(timescaleStore)
			monitoringService.StartCollection()

			log.Println("[Monitoring] TimescaleDB metrics access enabled")
		} else {
			log.Println("[Monitoring] TimescaleDB not available, using in-memory log buffer")
		}

		// Initialize TOTP service and handler (2FA for admin users)
		totpService := services.NewTOTPService(userRepo, totpRepo)
		totpHandler := handlers.NewTOTPHandler(totpService, userRepo, jwtManager)

		// Initialize file manager handler
		fileManagerHandler := handlers.NewFileManagerHandler(userService, totpService, cfg.BackupDir)

		// Wire S3 backends into file manager for R2/NAS browsing
		if r2MediaBackend != nil {
			fileManagerHandler.SetR2Backend(r2MediaBackend)
		}
		if nasMediaBackend != nil {
			fileManagerHandler.SetNASBackend(nasMediaBackend)
		}

		// Initialize point-in-time restore service (also used for backups)
		// Reverted to Old Pattern: Direct file access
		restoreService := services.NewRestoreService(pool, connStr, cfg.BackupDir, cfg.EnvTag(), systemSettingRepo)
		restoreHandler := handlers.NewRestoreHandler(restoreService)

		// Initialize deleted entries handler (soft delete recovery)
		deletedEntriesHandler := handlers.NewDeletedEntriesHandler(pool)

		// Always initialize monitoring handler
		monitoringHandler := handlers.NewMonitoringHandler(timescaleStore, pool, cfg.BackupDir)
		infraHandler := handlers.NewInfrastructureHandler(pool)
		// R2 backup scheduler
		restoreService.StartScheduler(context.Background())
		// handlers.StartBackupSchedulers(restoreService)

		log.Println("[Monitoring] Core monitoring features enabled")

		// Initialize season service and handler (needs tsdbPool for archiving timeseries data)
		seasonService := services.NewSeasonService(seasonRequestRepo, userRepo, pool, tsdbPool, jwtManager)
		seasonHandler := handlers.NewSeasonHandler(seasonService)

		// Initialize node provisioning (infrastructure management)
		infraRepo := repositories.NewInfrastructureRepository(pool)
		nodeProvisioningService := services.NewNodeProvisioningService(infraRepo)
		nodeProvisioningHandler := handlers.NewNodeProvisioningHandler(nodeProvisioningService)

		// Initialize deployment service (one-click deploy from UI)
		deploymentService := services.NewDeploymentService(infraRepo)
		deploymentHandler := handlers.NewDeploymentHandler(deploymentService)

		// Initialize report service (bulk PDF/CSV export with parallel processing)
		reportService := services.NewReportService(pool, customerRepo, entryRepo, roomEntryRepo, rentPaymentRepo, systemSettingRepo)
		reportHandler := handlers.NewReportHandler(reportService)

		// Initialize account handler (optimized single-call endpoint for Account Management)
		accountHandler := handlers.NewAccountHandler(pool, entryRepo, roomEntryRepo, rentPaymentRepo, gatePassRepo, systemSettingRepo, ledgerRepo)

		// Initialize entry room handler (optimized single-call endpoint for Entry Room page)
		entryRoomHandler := handlers.NewEntryRoomHandler(pool, entryRepo, roomEntryRepo, customerRepo, guardEntryRepo)

		// Initialize room visualization handler (visual storage occupancy map)
		roomVisualizationHandler := handlers.NewRoomVisualizationHandler(pool)

		// Initialize items in stock handler (current inventory overview)
		itemsInStockHandler := handlers.NewItemsInStockHandler(pool)

		// Initialize customer activity log handler (for admin to view customer portal logs)
		customerActivityLogRepo := repositories.NewCustomerActivityLogRepository(pool)
		customerActivityLogHandler := handlers.NewCustomerActivityLogHandler(customerActivityLogRepo)

		// Initialize SMS handler (for bulk SMS, logs, settings)
		smsHandler := handlers.NewSMSHandler(smsLogRepo, systemSettingRepo, employeeSMSService)

		// Initialize setup handler (disaster recovery - R2 restore)
		setupHandler := handlers.NewSetupHandler(cfg.BackupDir)

		// Initialize ledger and debt handlers
		ledgerHandler := handlers.NewLedgerHandler(ledgerService)
		debtHandler := handlers.NewDebtHandler(debtService)

		// Initialize merge history handler
		mergeHistoryHandler := handlers.NewMergeHistoryHandler(customerRepo, entryRepo, entryManagementLogRepo)

		// Initialize Razorpay service and handler for online payments (admin view)
		razorpayService := services.NewRazorpayService(
			cfg.Razorpay.KeyID,
			cfg.Razorpay.KeySecret,
			cfg.Razorpay.WebhookSecret,
			onlineTransactionRepo,
			rentPaymentRepo,
			ledgerRepo,
			customerRepo,
			systemSettingRepo,
		)
		razorpayHandler := handlers.NewRazorpayHandler(razorpayService, customerRepo)

		// Initialize pending setting change handler (dual admin approval for sensitive settings)
		pendingSettingHandler := handlers.NewPendingSettingHandler(
			pendingSettingChangeRepo,
			systemSettingRepo,
			userRepo,
			totpService,
		)

		// Point-in-time restore service and handler already initialized above for monitoring integration

		// Initialize media sync admin handler (3-2-1 backup dashboard)
		mediaSyncHandler := handlers.NewMediaSyncHandler(mediaSyncService)

		// Initialize pool sync service (sync all storage pools to NAS/RustFS)
		poolSyncRepo := repositories.NewPoolSyncRepository(pool)
		poolPaths := make(map[string]string)
		for k, v := range fileManagerHandler.RootPaths {
			if k == "trash" {
				continue // exclude trash from sync
			}
			poolPaths[k] = v
		}
		var poolSyncService *services.PoolSyncService
		var poolSyncHandler *handlers.PoolSyncHandler
		if nasMediaBackend != nil {
			poolSyncService = services.NewPoolSyncService(poolSyncRepo, nasMediaBackend, poolPaths)
			poolSyncService.Start()
			poolSyncHandler = handlers.NewPoolSyncHandler(poolSyncService)
			log.Println("[PoolSync] Pool sync service started")
		} else {
			log.Println("[PoolSync] NAS backend not available, pool sync disabled")
		}

		// Create employee router
		router := h.NewRouter(userHandler, authHandler, customerHandler, entryHandler, roomEntryHandler, entryEventHandler, systemSettingHandler, rentPaymentHandler, invoiceHandler, loginLogHandler, roomEntryEditLogHandler, entryEditLogHandler, entryManagementLogHandler, adminActionLogHandler, gatePassHandler, seasonHandler, guardEntryHandler, tokenColorHandler, pageHandler, healthHandler, authMiddleware, operationModeMiddleware, monitoringHandler, infraHandler, apiLoggingMiddleware, nodeProvisioningHandler, deploymentHandler, reportHandler, accountHandler, entryRoomHandler, roomVisualizationHandler, itemsInStockHandler, setupHandler, ledgerHandler, debtHandler, mergeHistoryHandler, customerActivityLogHandler, smsHandler, familyMemberHandler, razorpayHandler, pendingSettingHandler, totpHandler, restoreHandler, printerHandler, fileManagerHandler, deletedEntriesHandler, mediaSyncHandler, poolSyncHandler)

		// Add gallery routes if enabled
		if cfg.G.Enabled {
			gPool := db.ConnectG(cfg)
			gRepo := g.NewRepository(gPool)
			gService := g.NewService(gRepo)
			gHandler := g.NewHandler(gService)

			gRouter := router.PathPrefix("/g").Subrouter()
			gRouter.HandleFunc("/auth", gHandler.Auth).Methods("POST")
			gRouter.HandleFunc("/", gHandler.Dashboard).Methods("GET")
			gRouter.HandleFunc("/entry", gHandler.EntryPage).Methods("GET")
			gRouter.HandleFunc("/config", gHandler.ConfigPage).Methods("GET")
			gRouter.HandleFunc("/pass", gHandler.PassPage).Methods("GET")
			gRouter.HandleFunc("/search", gHandler.SearchPage).Methods("GET")
			gRouter.HandleFunc("/accounts", gHandler.AccountsPage).Methods("GET")
			gRouter.HandleFunc("/events", gHandler.EventsPage).Methods("GET")
			gRouter.HandleFunc("/unload", gHandler.UnloadPage).Methods("GET")
			gRouter.HandleFunc("/reports", gHandler.ReportsPage).Methods("GET")

			gAPI := gRouter.PathPrefix("/api").Subrouter()
			gAPI.Use(gHandler.AuthMiddleware)
			gAPI.HandleFunc("/items", gHandler.ListItems).Methods("GET")
			gAPI.HandleFunc("/items", gHandler.AddItem).Methods("POST")
			gAPI.HandleFunc("/items", gHandler.UpdateItem).Methods("PUT")
			gAPI.HandleFunc("/items", gHandler.DeleteItem).Methods("DELETE")
			gAPI.HandleFunc("/in", gHandler.StockIn).Methods("POST")
			gAPI.HandleFunc("/out", gHandler.StockOut).Methods("POST")
			gAPI.HandleFunc("/txns", gHandler.ListTxns).Methods("GET")
			gAPI.HandleFunc("/summary", gHandler.Summary).Methods("GET")
			gAPI.HandleFunc("/logout", gHandler.Logout).Methods("POST")
		}

		// Wrap with panic recovery and metrics middleware
		handler = middleware.PanicRecovery(middleware.MetricsMiddleware(corsMiddleware(router)))

		// Pre-warm cache in background (non-blocking)
		// This runs after handlers are initialized since they register pre-warm callbacks
		go cache.PreWarmCache()
		log.Println("[Redis] Pre-warming cache in background...")
	}

	// Start server
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.Port)
	log.Printf("Server running on %s (mode: %s)", addr, *mode)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
