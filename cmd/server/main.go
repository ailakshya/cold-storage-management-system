package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cold-backend/internal/auth"
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

	"github.com/jackc/pgx/v5/pgxpool"
)

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
		password = "MetricsDB2025!"
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

func main() {
	// Parse command-line flags
	mode := flag.String("mode", "employee", "Server mode: employee or customer")
	port := flag.Int("port", 0, "Server port (overrides config)")
	flag.Parse()

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

	// Connect to database
	pool := db.Connect(cfg)
	defer pool.Close()

	// Run database migrations
	// This automatically creates all required tables on startup
	log.Println("Running database migrations...")
	migrator := database.NewMigrator(pool)
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
	systemSettingRepo := repositories.NewSystemSettingRepository(pool)
	rentPaymentRepo := repositories.NewRentPaymentRepository(pool)
	gatePassRepo := repositories.NewGatePassRepository(pool)
	invoiceRepo := repositories.NewInvoiceRepository(pool)
	loginLogRepo := repositories.NewLoginLogRepository(pool)
	roomEntryEditLogRepo := repositories.NewRoomEntryEditLogRepository(pool)
	adminActionLogRepo := repositories.NewAdminActionLogRepository(pool)
	gatePassPickupRepo := repositories.NewGatePassPickupRepository(pool)

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
		entryHandler := handlers.NewEntryHandler(entryService)
		roomEntryHandler := handlers.NewRoomEntryHandler(roomEntryService, roomEntryEditLogRepo)
		entryEventHandler := handlers.NewEntryEventHandler(entryEventRepo)
		systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingService)
		rentPaymentHandler := handlers.NewRentPaymentHandler(rentPaymentService)
		invoiceHandler := handlers.NewInvoiceHandler(invoiceService)
		loginLogHandler := handlers.NewLoginLogHandler(loginLogRepo)
		roomEntryEditLogHandler := handlers.NewRoomEntryEditLogHandler(roomEntryEditLogRepo)
		adminActionLogHandler := handlers.NewAdminActionLogHandler(adminActionLogRepo)
		gatePassHandler := handlers.NewGatePassHandler(gatePassService, adminActionLogRepo)

		// Initialize season repository
		seasonRequestRepo := repositories.NewSeasonRequestRepository(pool)

		// Initialize TimescaleDB connection for metrics (optional - degrades gracefully)
		var metricsRepo *repositories.MetricsRepository
		var monitoringHandler *handlers.MonitoringHandler
		var apiLoggingMiddleware *middleware.APILoggingMiddleware
		var metricsCollector *services.MetricsCollector

		tsdbPool := connectTimescaleDB()
		if tsdbPool != nil {
			defer tsdbPool.Close()
			log.Println("[Monitoring] Initializing monitoring components...")

			// Initialize metrics repository
			metricsRepo = repositories.NewMetricsRepository(tsdbPool)

			// Initialize API logging middleware
			apiLoggingMiddleware = middleware.NewAPILoggingMiddleware(metricsRepo)

			// Initialize monitoring handler
			monitoringHandler = handlers.NewMonitoringHandler(metricsRepo)

			// Initialize and start metrics collector
			metricsCollector = services.NewMetricsCollector(metricsRepo)
			metricsCollector.Start()
			defer metricsCollector.Stop()

			log.Println("[Monitoring] Monitoring components initialized successfully")
		} else {
			log.Println("[Monitoring] TimescaleDB not available, monitoring features disabled")
		}

		// Initialize season service and handler (needs tsdbPool for archiving timeseries data)
		seasonService := services.NewSeasonService(seasonRequestRepo, userRepo, pool, tsdbPool, jwtManager)
		seasonHandler := handlers.NewSeasonHandler(seasonService)

		// Create employee router
		router := h.NewRouter(userHandler, authHandler, customerHandler, entryHandler, roomEntryHandler, entryEventHandler, systemSettingHandler, rentPaymentHandler, invoiceHandler, loginLogHandler, roomEntryEditLogHandler, adminActionLogHandler, gatePassHandler, seasonHandler, pageHandler, healthHandler, authMiddleware, operationModeMiddleware, monitoringHandler, apiLoggingMiddleware)

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
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server running on %s (mode: %s)", addr, *mode)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
