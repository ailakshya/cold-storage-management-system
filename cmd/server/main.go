package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"cold-backend/internal/auth"
	"cold-backend/internal/cache"
	"cold-backend/internal/config"
	"cold-backend/internal/database"
	"cold-backend/internal/db"
	"cold-backend/internal/g"
	h "cold-backend/internal/http"
	"cold-backend/internal/handlers"
	"cold-backend/internal/health"
	"cold-backend/internal/middleware"
	"cold-backend/internal/monitoring"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"
	"cold-backend/internal/sms"
	"cold-backend/migrations"
	"cold-backend/static"

	"github.com/jackc/pgx/v5/pgxpool"
)

// startSetupMode starts the server in setup mode when no database is available
func startSetupMode(cfg *config.Config) {
	setupHandler := handlers.NewSetupHandler()

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
		user, password, host, port, database)

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

// runInstall sets up PostgreSQL and the database automatically
func runInstall() {
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║     COLD STORAGE - AUTOMATIC SETUP                         ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	// Check if running as root
	if os.Geteuid() != 0 {
		log.Fatal("Please run with sudo: sudo ./server --install")
	}

	// Step 1: Install PostgreSQL
	log.Println("[1/4] Installing PostgreSQL...")
	cmd := exec.Command("bash", "-c", `
		if command -v psql &> /dev/null; then
			echo "PostgreSQL already installed"
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
		log.Fatalf("Failed to install PostgreSQL: %v", err)
	}

	// Step 2: Create database
	log.Println("[2/4] Creating database...")
	cmd = exec.Command("bash", "-c", `
		if sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw cold_db; then
			echo "Database 'cold_db' already exists"
		else
			sudo -u postgres psql -c "CREATE DATABASE cold_db;"
			echo "Database 'cold_db' created"
		fi
	`)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	// Step 3: Setup systemd service
	log.Println("[3/4] Setting up systemd service...")

	// Get current binary path
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	// Copy to /opt
	cmd = exec.Command("bash", "-c", fmt.Sprintf(`
		mkdir -p /opt/cold-backend
		cp "%s" /opt/cold-backend/server
		chmod +x /opt/cold-backend/server
	`, execPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// Create systemd service file
	serviceContent := `[Unit]
Description=Cold Storage Backend
After=postgresql.service network.target
Wants=postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/cold-backend
ExecStart=/opt/cold-backend/server
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/cold-backend.service", []byte(serviceContent), 0644); err != nil {
		log.Printf("Warning: Failed to create service file: %v", err)
	}

	cmd = exec.Command("bash", "-c", "systemctl daemon-reload && systemctl enable cold-backend")
	cmd.Run()

	// Step 4: Start the service
	log.Println("[4/4] Starting server...")
	cmd = exec.Command("systemctl", "start", "cold-backend")
	cmd.Run()

	// Wait and check status
	time.Sleep(10 * time.Second)

	cmd = exec.Command("systemctl", "is-active", "cold-backend")
	if err := cmd.Run(); err != nil {
		log.Println("Server may have failed to start. Checking logs...")
		cmd = exec.Command("journalctl", "-u", "cold-backend", "-n", "30", "--no-pager")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
		os.Exit(1)
	}

	log.Println("")
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║                 INSTALLATION COMPLETE!                     ║")
	log.Println("╠════════════════════════════════════════════════════════════╣")
	log.Println("║  Server running on :8080                                   ║")
	log.Println("║                                                            ║")
	log.Println("║  Commands:                                                 ║")
	log.Println("║    View logs:  journalctl -u cold-backend -f               ║")
	log.Println("║    Stop:       systemctl stop cold-backend                 ║")
	log.Println("║    Restart:    systemctl restart cold-backend              ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	// Show recent logs
	log.Println("\nRecent logs:")
	cmd = exec.Command("journalctl", "-u", "cold-backend", "-n", "20", "--no-pager")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

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
	pool, connectedTo, connStr := db.TryConnectWithFallback()
	if pool == nil {
		// Check if running as root - if so, auto-install PostgreSQL
		if os.Geteuid() == 0 {
			log.Println("╔════════════════════════════════════════════════════════════╗")
			log.Println("║  NO DATABASE - INSTALLING POSTGRESQL AUTOMATICALLY         ║")
			log.Println("╚════════════════════════════════════════════════════════════╝")
			autoInstallPostgreSQL()
			// Try connecting again after install
			pool, connectedTo, connStr = db.TryConnectWithFallback()
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

	// Automatic disaster recovery: restore from R2 if database is empty
	// This only runs when connecting to localhost (disaster recovery scenario)
	if connectedTo == "Localhost (Disaster Recovery)" {
		handlers.AutoRestoreFromR2(connStr)
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
	systemSettingRepo := repositories.NewSystemSettingRepository(pool)
	rentPaymentRepo := repositories.NewRentPaymentRepository(pool)
	gatePassRepo := repositories.NewGatePassRepository(pool)
	invoiceRepo := repositories.NewInvoiceRepository(pool)
	loginLogRepo := repositories.NewLoginLogRepository(pool)
	roomEntryEditLogRepo := repositories.NewRoomEntryEditLogRepository(pool)
	entryEditLogRepo := repositories.NewEntryEditLogRepository(pool)
	adminActionLogRepo := repositories.NewAdminActionLogRepository(pool)
	gatePassPickupRepo := repositories.NewGatePassPickupRepository(pool)
	guardEntryRepo := repositories.NewGuardEntryRepository(pool)
	tokenColorRepo := repositories.NewTokenColorRepository(pool)

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

		// Use MockSMSService for testing (prints OTP to console)
		// For production, use: smsService := sms.NewFast2SMSService(cfg.SMS.APIKey)
		smsService := sms.NewMockSMSService()

		// Initialize OTP service
		otpService := services.NewOTPService(otpRepo, customerRepo, smsService)

		// Initialize customer portal service
		customerPortalService := services.NewCustomerPortalService(
			customerRepo,
			entryRepo,
			roomEntryRepo,
			gatePassRepo,
			rentPaymentRepo,
			systemSettingRepo,
			gatePassPickupRepo,
		)

		// Initialize customer portal handler
		customerPortalHandler := handlers.NewCustomerPortalHandler(
			otpService,
			customerPortalService,
			jwtManager,
		)

		// Create customer router
		router := h.NewCustomerRouter(customerPortalHandler, pageHandler, healthHandler, authMiddleware)

		// Wrap with panic recovery and metrics middleware
		handler = middleware.PanicRecovery(middleware.MetricsMiddleware(corsMiddleware(router)))

	} else {
		log.Println("Starting in EMPLOYEE mode")

		// Initialize services (employee mode)
		userService := services.NewUserService(userRepo, jwtManager)
		customerService := services.NewCustomerService(customerRepo)
		entryService := services.NewEntryService(entryRepo, customerRepo, entryEventRepo)
		roomEntryService := services.NewRoomEntryService(roomEntryRepo, entryRepo, entryEventRepo)
		systemSettingService := services.NewSystemSettingService(systemSettingRepo)
		rentPaymentService := services.NewRentPaymentService(rentPaymentRepo)
		invoiceService := services.NewInvoiceService(invoiceRepo)
		gatePassService := services.NewGatePassService(gatePassRepo, entryRepo, entryEventRepo, gatePassPickupRepo, roomEntryRepo)

		// Initialize handlers (employee mode)
		userHandler := handlers.NewUserHandler(userService, adminActionLogRepo)
		authHandler := handlers.NewAuthHandler(userService, loginLogRepo)
		customerHandler := handlers.NewCustomerHandler(customerService)
		entryHandler := handlers.NewEntryHandler(entryService, entryEditLogRepo)
		roomEntryHandler := handlers.NewRoomEntryHandler(roomEntryService, roomEntryEditLogRepo)
		entryEventHandler := handlers.NewEntryEventHandler(entryEventRepo)
		systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingService)
		rentPaymentHandler := handlers.NewRentPaymentHandler(rentPaymentService)
		invoiceHandler := handlers.NewInvoiceHandler(invoiceService)
		loginLogHandler := handlers.NewLoginLogHandler(loginLogRepo)
		roomEntryEditLogHandler := handlers.NewRoomEntryEditLogHandler(roomEntryEditLogRepo)
		entryEditLogHandler := handlers.NewEntryEditLogHandler(entryEditLogRepo)
		adminActionLogHandler := handlers.NewAdminActionLogHandler(adminActionLogRepo)
		gatePassHandler := handlers.NewGatePassHandler(gatePassService, adminActionLogRepo)

		// Initialize guard entry service and handler
		guardEntryService := services.NewGuardEntryService(guardEntryRepo)
		guardEntryHandler := handlers.NewGuardEntryHandler(guardEntryService, adminActionLogRepo)

		// Initialize token color handler
		tokenColorHandler := handlers.NewTokenColorHandler(tokenColorRepo)

		// Initialize season repository
		seasonRequestRepo := repositories.NewSeasonRequestRepository(pool)

		// Initialize TimescaleDB connection for metrics (optional - degrades gracefully)
		var metricsRepo *repositories.MetricsRepository
		var apiLoggingMiddleware *middleware.APILoggingMiddleware
		var metricsCollector *services.MetricsCollector

		tsdbPool := connectTimescaleDB()
		if tsdbPool != nil {
			defer tsdbPool.Close()
			log.Println("[Monitoring] Initializing TimescaleDB monitoring components...")

			// Initialize metrics repository
			metricsRepo = repositories.NewMetricsRepository(tsdbPool)

			// Initialize API logging middleware
			apiLoggingMiddleware = middleware.NewAPILoggingMiddleware(metricsRepo)

			// Initialize and start metrics collector
			metricsCollector = services.NewMetricsCollector(metricsRepo)
			metricsCollector.Start()
			defer metricsCollector.Stop()

			log.Println("[Monitoring] TimescaleDB monitoring components initialized")
		} else {
			log.Println("[Monitoring] TimescaleDB not available, time-series metrics disabled")
		}

		// Always initialize monitoring handler (R2 backup/restore works without TimescaleDB)
		monitoringHandler := handlers.NewMonitoringHandler(metricsRepo)

		// Start R2 automatic backup scheduler (for near-zero data loss)
		handlers.StartR2BackupScheduler(pool)
		defer handlers.StopR2BackupScheduler()

		log.Println("[Monitoring] Core monitoring features enabled (R2 backups active)")

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
		accountHandler := handlers.NewAccountHandler(pool, entryRepo, roomEntryRepo, rentPaymentRepo, gatePassRepo, systemSettingRepo)

		// Initialize entry room handler (optimized single-call endpoint for Entry Room page)
		entryRoomHandler := handlers.NewEntryRoomHandler(pool, entryRepo, roomEntryRepo, customerRepo, guardEntryRepo)

		// Initialize room visualization handler (visual storage occupancy map)
		roomVisualizationHandler := handlers.NewRoomVisualizationHandler(pool)

		// Initialize setup handler (disaster recovery - R2 restore)
		setupHandler := handlers.NewSetupHandler()

		// Create employee router
		router := h.NewRouter(userHandler, authHandler, customerHandler, entryHandler, roomEntryHandler, entryEventHandler, systemSettingHandler, rentPaymentHandler, invoiceHandler, loginLogHandler, roomEntryEditLogHandler, entryEditLogHandler, adminActionLogHandler, gatePassHandler, seasonHandler, guardEntryHandler, tokenColorHandler, pageHandler, healthHandler, authMiddleware, operationModeMiddleware, monitoringHandler, apiLoggingMiddleware, nodeProvisioningHandler, deploymentHandler, reportHandler, accountHandler, entryRoomHandler, roomVisualizationHandler, setupHandler)

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
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server running on %s (mode: %s)", addr, *mode)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
