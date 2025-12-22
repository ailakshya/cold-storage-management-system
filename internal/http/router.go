package http

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"cold-backend/internal/handlers"
	"cold-backend/internal/middleware"
)

func NewRouter(
	userHandler *handlers.UserHandler,
	authHandler *handlers.AuthHandler,
	customerHandler *handlers.CustomerHandler,
	entryHandler *handlers.EntryHandler,
	roomEntryHandler *handlers.RoomEntryHandler,
	entryEventHandler *handlers.EntryEventHandler,
	systemSettingHandler *handlers.SystemSettingHandler,
	rentPaymentHandler *handlers.RentPaymentHandler,
	invoiceHandler *handlers.InvoiceHandler,
	loginLogHandler *handlers.LoginLogHandler,
	roomEntryEditLogHandler *handlers.RoomEntryEditLogHandler,
	entryEditLogHandler *handlers.EntryEditLogHandler,
	adminActionLogHandler *handlers.AdminActionLogHandler,
	gatePassHandler *handlers.GatePassHandler,
	seasonHandler *handlers.SeasonHandler,
	guardEntryHandler *handlers.GuardEntryHandler,
	pageHandler *handlers.PageHandler,
	healthHandler *handlers.HealthHandler,
	authMiddleware *middleware.AuthMiddleware,
	operationModeMiddleware *middleware.OperationModeMiddleware,
	monitoringHandler *handlers.MonitoringHandler,
	apiLoggingMiddleware *middleware.APILoggingMiddleware,
	nodeProvisioningHandler *handlers.NodeProvisioningHandler,
	deploymentHandler *handlers.DeploymentHandler,
) *mux.Router {
	r := mux.NewRouter()

	// Apply security middlewares first
	r.Use(middleware.HTTPSRedirect)
	r.Use(middleware.SecurityHeaders)

	// Apply API logging middleware to all routes (if enabled)
	if apiLoggingMiddleware != nil {
		r.Use(apiLoggingMiddleware.Handler)
	}

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public HTML pages (NO AUTHENTICATION REQUIRED)
	r.HandleFunc("/", pageHandler.LoginPage).Methods("GET")
	r.HandleFunc("/login", pageHandler.LoginPage).Methods("GET")
	r.HandleFunc("/logout", pageHandler.LogoutPage).Methods("GET")

	// Public API routes - Authentication (with rate limiting)
	r.HandleFunc("/auth/signup", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(authHandler.Signup)).ServeHTTP).Methods("POST")
	r.HandleFunc("/auth/login", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(authHandler.Login)).ServeHTTP).Methods("POST")

	// Protected HTML pages - Client-side authentication via localStorage
	// These pages load without server-side auth and use JavaScript to check localStorage
	// Common pages (employee, admin, accountant)
	r.HandleFunc("/dashboard", pageHandler.DashboardPage).Methods("GET")
	r.HandleFunc("/item-search", pageHandler.ItemSearchPage).Methods("GET")
	r.HandleFunc("/events", pageHandler.EventTracerPage).Methods("GET")
	r.HandleFunc("/entry-room", pageHandler.EntryRoomPage).Methods("GET")
	r.HandleFunc("/main-entry", pageHandler.MainEntryPage).Methods("GET")
	r.HandleFunc("/room-config-1", pageHandler.RoomConfig1Page).Methods("GET")
	r.HandleFunc("/room-form-2", pageHandler.RoomForm2Page).Methods("GET")
	r.HandleFunc("/loading-invoice", pageHandler.LoadingInvoicePage).Methods("GET")
	r.HandleFunc("/room-entry-edit", pageHandler.RoomEntryEditPage).Methods("GET")
	r.HandleFunc("/payment-receipt", pageHandler.PaymentReceiptPage).Methods("GET")
	r.HandleFunc("/verify-receipt", pageHandler.VerifyReceiptPage).Methods("GET")

	// Unloading mode pages
	r.HandleFunc("/gate-pass-entry", pageHandler.GatePassEntryPage).Methods("GET")
	r.HandleFunc("/unloading-tickets", pageHandler.UnloadingTicketsPage).Methods("GET")

	// Accountant pages
	r.HandleFunc("/rent", pageHandler.RentPage).Methods("GET")
	r.HandleFunc("/rent-management", pageHandler.RentManagementPage).Methods("GET")
	r.HandleFunc("/accountant/dashboard", pageHandler.AccountantDashboardPage).Methods("GET")

	// Admin-only pages
	r.HandleFunc("/admin/dashboard", pageHandler.AdminDashboardPage).Methods("GET")
	r.HandleFunc("/employees", pageHandler.EmployeesPage).Methods("GET")
	r.HandleFunc("/system-settings", pageHandler.SystemSettingsPage).Methods("GET")
	r.HandleFunc("/admin/report", pageHandler.AdminReportPage).Methods("GET")
	r.HandleFunc("/admin/logs", pageHandler.AdminLogsPage).Methods("GET")
	r.HandleFunc("/infrastructure", pageHandler.InfrastructureMonitoringPage).Methods("GET")
	r.HandleFunc("/infrastructure/nodes", pageHandler.NodeProvisioningPage).Methods("GET")
	r.HandleFunc("/monitoring", pageHandler.MonitoringDashboardPage).Methods("GET")
	r.HandleFunc("/customer-export", pageHandler.CustomerPDFExportPage).Methods("GET")
	r.HandleFunc("/customer-edit", pageHandler.CustomerEditPage).Methods("GET")

	// Guard pages
	r.HandleFunc("/guard/dashboard", pageHandler.GuardDashboardPage).Methods("GET")
	r.HandleFunc("/guard/register", pageHandler.GuardRegisterPage).Methods("GET")

	// Protected API routes - System Settings
	settingsAPI := r.PathPrefix("/api/settings").Subrouter()
	settingsAPI.Use(authMiddleware.Authenticate)
	settingsAPI.HandleFunc("", systemSettingHandler.ListSettings).Methods("GET")
	settingsAPI.HandleFunc("/operation_mode", systemSettingHandler.GetOperationMode).Methods("GET")
	settingsAPI.HandleFunc("/{key}", systemSettingHandler.GetSetting).Methods("GET")
	settingsAPI.HandleFunc("/{key}", authMiddleware.RequireAdmin(http.HandlerFunc(systemSettingHandler.UpdateSetting)).ServeHTTP).Methods("PUT")

	// Protected API routes - Users
	usersAPI := r.PathPrefix("/api/users").Subrouter()
	usersAPI.Use(authMiddleware.Authenticate)
	usersAPI.HandleFunc("", userHandler.ListUsers).Methods("GET")
	usersAPI.HandleFunc("", authMiddleware.RequireAdmin(http.HandlerFunc(userHandler.CreateUser)).ServeHTTP).Methods("POST")
	usersAPI.HandleFunc("/{id}", userHandler.GetUser).Methods("GET")
	usersAPI.HandleFunc("/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(userHandler.UpdateUser)).ServeHTTP).Methods("PUT")
	usersAPI.HandleFunc("/{id}", authMiddleware.RequireAdmin(http.HandlerFunc(userHandler.DeleteUser)).ServeHTTP).Methods("DELETE")
	usersAPI.HandleFunc("/{id}/toggle-active", authMiddleware.RequireAdmin(http.HandlerFunc(userHandler.ToggleActiveStatus)).ServeHTTP).Methods("PATCH")

	// Protected API routes - Customers
	customersAPI := r.PathPrefix("/api/customers").Subrouter()
	customersAPI.Use(authMiddleware.Authenticate)
	customersAPI.HandleFunc("", customerHandler.ListCustomers).Methods("GET")
	customersAPI.HandleFunc("", customerHandler.CreateCustomer).Methods("POST")
	customersAPI.HandleFunc("/search", customerHandler.SearchByPhone).Methods("GET")
	customersAPI.HandleFunc("/{id}", customerHandler.GetCustomer).Methods("GET")
	customersAPI.HandleFunc("/{id}", customerHandler.UpdateCustomer).Methods("PUT")
	customersAPI.HandleFunc("/{id}", customerHandler.DeleteCustomer).Methods("DELETE")

	// Protected API routes - Entries (employees and admins only for creation, LOADING MODE ONLY)
	entriesAPI := r.PathPrefix("/api/entries").Subrouter()
	entriesAPI.Use(authMiddleware.Authenticate)
	entriesAPI.HandleFunc("", entryHandler.ListEntries).Methods("GET") // All authenticated users can view
	// Entry creation requires loading mode (blocked in unloading mode for non-admins)
	entriesAPI.HandleFunc("", operationModeMiddleware.RequireLoadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(entryHandler.CreateEntry)),
	).ServeHTTP).Methods("POST")
	entriesAPI.HandleFunc("/count", entryHandler.GetCountByCategory).Methods("GET")
	entriesAPI.HandleFunc("/unassigned", roomEntryHandler.GetUnassignedEntries).Methods("GET")
	entriesAPI.HandleFunc("/{id}", entryHandler.GetEntry).Methods("GET")
	entriesAPI.HandleFunc("/{id}", entryHandler.UpdateEntry).Methods("PUT")
	entriesAPI.HandleFunc("/customer/{customer_id}", entryHandler.ListEntriesByCustomer).Methods("GET")

	// Protected API routes - Room Entries (employees and admins only for creation/update, LOADING MODE ONLY)
	roomEntriesAPI := r.PathPrefix("/api/room-entries").Subrouter()
	roomEntriesAPI.Use(authMiddleware.Authenticate)
	roomEntriesAPI.HandleFunc("", roomEntryHandler.ListRoomEntries).Methods("GET") // All authenticated users can view
	// Room entry creation/update requires loading mode
	roomEntriesAPI.HandleFunc("", operationModeMiddleware.RequireLoadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(roomEntryHandler.CreateRoomEntry)),
	).ServeHTTP).Methods("POST")
	roomEntriesAPI.HandleFunc("/{id}", roomEntryHandler.GetRoomEntry).Methods("GET")
	roomEntriesAPI.HandleFunc("/{id}", operationModeMiddleware.RequireLoadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(roomEntryHandler.UpdateRoomEntry)),
	).ServeHTTP).Methods("PUT")

	// Protected API routes - Entry Events
	entryEventsAPI := r.PathPrefix("/api/entry-events").Subrouter()
	entryEventsAPI.Use(authMiddleware.Authenticate)
	entryEventsAPI.HandleFunc("", entryEventHandler.CreateEntryEvent).Methods("POST")

	// Protected API routes - Rent Payments (accountants, admins, and employees with accountant access)
	rentPaymentsAPI := r.PathPrefix("/api/rent-payments").Subrouter()
	rentPaymentsAPI.Use(authMiddleware.Authenticate)
	rentPaymentsAPI.HandleFunc("", authMiddleware.RequireAccountantAccess(http.HandlerFunc(rentPaymentHandler.CreatePayment)).ServeHTTP).Methods("POST")
	rentPaymentsAPI.HandleFunc("", authMiddleware.RequireAccountantAccess(http.HandlerFunc(rentPaymentHandler.ListPayments)).ServeHTTP).Methods("GET")
	rentPaymentsAPI.HandleFunc("/entry/{entry_id}", authMiddleware.RequireAccountantAccess(http.HandlerFunc(rentPaymentHandler.GetPaymentsByEntry)).ServeHTTP).Methods("GET")
	rentPaymentsAPI.HandleFunc("/phone", authMiddleware.RequireAccountantAccess(http.HandlerFunc(rentPaymentHandler.GetPaymentsByPhone)).ServeHTTP).Methods("GET")
	rentPaymentsAPI.HandleFunc("/receipt/{receipt_number}", authMiddleware.RequireAccountantAccess(http.HandlerFunc(rentPaymentHandler.GetPaymentByReceiptNumber)).ServeHTTP).Methods("GET")

	// Protected API routes - Invoices (employees and admins can create, all can view)
	invoicesAPI := r.PathPrefix("/api/invoices").Subrouter()
	invoicesAPI.Use(authMiddleware.Authenticate)
	invoicesAPI.HandleFunc("", invoiceHandler.CreateInvoice).Methods("POST")
	invoicesAPI.HandleFunc("", invoiceHandler.ListInvoices).Methods("GET")
	invoicesAPI.HandleFunc("/{id}", invoiceHandler.GetInvoice).Methods("GET")
	invoicesAPI.HandleFunc("/number/{number}", invoiceHandler.GetInvoiceByNumber).Methods("GET")
	invoicesAPI.HandleFunc("/customer/{customer_id}", invoiceHandler.GetCustomerInvoices).Methods("GET")

	// Protected API routes - Login Logs (admin only)
	loginLogsAPI := r.PathPrefix("/api/login-logs").Subrouter()
	loginLogsAPI.Use(authMiddleware.Authenticate)
	loginLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(loginLogHandler.ListLoginLogs)).ServeHTTP).Methods("GET")

	// Protected API routes - Logout
	logoutAPI := r.PathPrefix("/api/logout").Subrouter()
	logoutAPI.Use(authMiddleware.Authenticate)
	logoutAPI.HandleFunc("", loginLogHandler.Logout).Methods("POST")

	// Protected API routes - Room Entry Edit Logs (admin only)
	editLogsAPI := r.PathPrefix("/api/edit-logs").Subrouter()
	editLogsAPI.Use(authMiddleware.Authenticate)
	editLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(roomEntryEditLogHandler.ListEditLogs)).ServeHTTP).Methods("GET")

	// Protected API routes - Entry Edit Logs (admin only)
	entryEditLogsAPI := r.PathPrefix("/api/entry-edit-logs").Subrouter()
	entryEditLogsAPI.Use(authMiddleware.Authenticate)
	entryEditLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(entryEditLogHandler.ListAll)).ServeHTTP).Methods("GET")
	entryEditLogsAPI.HandleFunc("/{id}", authMiddleware.RequireRole("admin")(http.HandlerFunc(entryEditLogHandler.ListByEntry)).ServeHTTP).Methods("GET")

	// Protected API routes - Admin Action Logs (admin only)
	adminActionLogsAPI := r.PathPrefix("/api/admin-action-logs").Subrouter()
	adminActionLogsAPI.Use(authMiddleware.Authenticate)
	adminActionLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(adminActionLogHandler.ListActionLogs)).ServeHTTP).Methods("GET")

	// Protected API routes - Season Management (admin only, dual approval)
	if seasonHandler != nil {
		seasonAPI := r.PathPrefix("/api/season").Subrouter()
		seasonAPI.Use(authMiddleware.Authenticate)
		seasonAPI.Use(authMiddleware.RequireRole("admin"))
		seasonAPI.HandleFunc("/initiate", seasonHandler.InitiateSeason).Methods("POST")
		seasonAPI.HandleFunc("/pending", seasonHandler.GetPending).Methods("GET")
		seasonAPI.HandleFunc("/history", seasonHandler.GetHistory).Methods("GET")
		seasonAPI.HandleFunc("/archived/{seasonName}", seasonHandler.GetArchivedData).Methods("GET")
		seasonAPI.HandleFunc("/{id}", seasonHandler.GetRequest).Methods("GET")
		seasonAPI.HandleFunc("/{id}/approve", seasonHandler.ApproveRequest).Methods("POST")
		seasonAPI.HandleFunc("/{id}/reject", seasonHandler.RejectRequest).Methods("POST")
	}

	// Protected API routes - Guard Entries (guard register feature)
	if guardEntryHandler != nil {
		guardAPI := r.PathPrefix("/api/guard").Subrouter()
		guardAPI.Use(authMiddleware.Authenticate)

		// Guard-only endpoints (create and list own entries)
		guardAPI.HandleFunc("/entries", authMiddleware.RequireRole("guard")(
			http.HandlerFunc(guardEntryHandler.CreateGuardEntry),
		).ServeHTTP).Methods("POST")
		guardAPI.HandleFunc("/entries", authMiddleware.RequireRole("guard")(
			http.HandlerFunc(guardEntryHandler.ListMyEntries),
		).ServeHTTP).Methods("GET")
		guardAPI.HandleFunc("/stats", authMiddleware.RequireRole("guard")(
			http.HandlerFunc(guardEntryHandler.GetMyStats),
		).ServeHTTP).Methods("GET")

		// Pending entries - accessible by employee, admin (for entry room)
		guardAPI.HandleFunc("/entries/pending", authMiddleware.RequireRole("employee", "admin")(
			http.HandlerFunc(guardEntryHandler.ListPendingEntries),
		).ServeHTTP).Methods("GET")

		// Get single entry - accessible by guard, employee, admin
		guardAPI.HandleFunc("/entries/{id}", authMiddleware.RequireRole("guard", "employee", "admin")(
			http.HandlerFunc(guardEntryHandler.GetGuardEntry),
		).ServeHTTP).Methods("GET")

		// Process entry - only employee or admin can mark as processed
		guardAPI.HandleFunc("/entries/{id}/process", authMiddleware.RequireRole("employee", "admin")(
			http.HandlerFunc(guardEntryHandler.ProcessGuardEntry),
		).ServeHTTP).Methods("PUT")
	}

	// Protected API routes - Gate Passes (UNLOADING MODE ONLY for operations)
	gatePassAPI := r.PathPrefix("/api/gate-passes").Subrouter()
	gatePassAPI.Use(authMiddleware.Authenticate)
	// Gate pass operations require unloading mode
	gatePassAPI.HandleFunc("", operationModeMiddleware.RequireUnloadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.CreateGatePass)),
	).ServeHTTP).Methods("POST")
	gatePassAPI.HandleFunc("", operationModeMiddleware.RequireUnloadingMode(
		http.HandlerFunc(gatePassHandler.ListAllGatePasses),
	).ServeHTTP).Methods("GET")
	gatePassAPI.HandleFunc("/pending", operationModeMiddleware.RequireUnloadingMode(
		http.HandlerFunc(gatePassHandler.ListPendingGatePasses),
	).ServeHTTP).Methods("GET")
	gatePassAPI.HandleFunc("/approved", operationModeMiddleware.RequireUnloadingMode(
		http.HandlerFunc(gatePassHandler.ListApprovedGatePasses),
	).ServeHTTP).Methods("GET")
	gatePassAPI.HandleFunc("/expired", operationModeMiddleware.RequireUnloadingMode(
		http.HandlerFunc(gatePassHandler.GetExpiredGatePasses),
	).ServeHTTP).Methods("GET")
	gatePassAPI.HandleFunc("/{id}/approve", operationModeMiddleware.RequireUnloadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.ApproveGatePass)),
	).ServeHTTP).Methods("PUT")
	gatePassAPI.HandleFunc("/{id}/complete", operationModeMiddleware.RequireUnloadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.CompleteGatePass)),
	).ServeHTTP).Methods("POST")
	// Static paths must come before dynamic {id} paths
	gatePassAPI.HandleFunc("/pickups/all", gatePassHandler.ListAllPickups).Methods("GET")   // All pickups for activity log
	gatePassAPI.HandleFunc("/pickups/by-thock", gatePassHandler.GetPickupHistoryByThock).Methods("GET") // Pickups by thock number
	gatePassAPI.HandleFunc("/{id}/pickups", gatePassHandler.GetPickupHistory).Methods("GET") // View only - allowed in any mode
	gatePassAPI.HandleFunc("/pickup", operationModeMiddleware.RequireUnloadingMode(
		authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.RecordPickup)),
	).ServeHTTP).Methods("POST")

	// Protected API routes - Infrastructure Monitoring
	infraHandler := handlers.NewInfrastructureHandler()
	infraAPI := r.PathPrefix("/api/infrastructure").Subrouter()
	infraAPI.Use(authMiddleware.Authenticate)
	// Read-only endpoints - any authenticated user can view
	infraAPI.HandleFunc("/backup-status", infraHandler.GetBackupStatus).Methods("GET")
	infraAPI.HandleFunc("/k3s-status", infraHandler.GetK3sStatus).Methods("GET")
	infraAPI.HandleFunc("/postgresql-status", infraHandler.GetPostgreSQLStatus).Methods("GET")
	infraAPI.HandleFunc("/postgresql-pods", infraHandler.GetPostgreSQLPods).Methods("GET")
	infraAPI.HandleFunc("/vip-status", infraHandler.GetVIPStatus).Methods("GET")
	infraAPI.HandleFunc("/backend-pods", infraHandler.GetBackendPods).Methods("GET")
	infraAPI.HandleFunc("/recovery-status", infraHandler.GetRecoveryStatus).Methods("GET")
	// Dangerous operations - admin only
	infraAPI.HandleFunc("/trigger-backup", authMiddleware.RequireAdmin(http.HandlerFunc(infraHandler.TriggerBackup)).ServeHTTP).Methods("POST")
	infraAPI.HandleFunc("/backup-schedule", authMiddleware.RequireAdmin(http.HandlerFunc(infraHandler.UpdateBackupSchedule)).ServeHTTP).Methods("POST")
	infraAPI.HandleFunc("/failover", authMiddleware.RequireAdmin(http.HandlerFunc(infraHandler.ExecuteFailover)).ServeHTTP).Methods("POST")
	infraAPI.HandleFunc("/recover-stuck-pods", authMiddleware.RequireAdmin(http.HandlerFunc(infraHandler.RecoverStuckPods)).ServeHTTP).Methods("POST")

	// Protected API routes - Node Provisioning (admin only)
	if nodeProvisioningHandler != nil {
		nodeAPI := r.PathPrefix("/api/infrastructure/nodes").Subrouter()
		nodeAPI.Use(authMiddleware.Authenticate)
		nodeAPI.Use(authMiddleware.RequireRole("admin"))

		// Node management
		nodeAPI.HandleFunc("", nodeProvisioningHandler.ListNodes).Methods("GET")
		nodeAPI.HandleFunc("", nodeProvisioningHandler.AddNode).Methods("POST")
		nodeAPI.HandleFunc("/test-connection", nodeProvisioningHandler.TestConnection).Methods("POST")
		nodeAPI.HandleFunc("/{id}", nodeProvisioningHandler.GetNode).Methods("GET")
		nodeAPI.HandleFunc("/{id}", nodeProvisioningHandler.RemoveNode).Methods("DELETE")

		// Provisioning
		nodeAPI.HandleFunc("/{id}/provision", nodeProvisioningHandler.ProvisionNode).Methods("POST")
		nodeAPI.HandleFunc("/{id}/provision/status", nodeProvisioningHandler.GetProvisionStatus).Methods("GET")
		nodeAPI.HandleFunc("/{id}/provision/logs", nodeProvisioningHandler.GetProvisionLogs).Methods("GET")

		// Node operations
		nodeAPI.HandleFunc("/{id}/drain", nodeProvisioningHandler.DrainNode).Methods("POST")
		nodeAPI.HandleFunc("/{id}/cordon", nodeProvisioningHandler.CordonNode).Methods("POST")
		nodeAPI.HandleFunc("/{id}/uncordon", nodeProvisioningHandler.UncordonNode).Methods("POST")
		nodeAPI.HandleFunc("/{id}/reboot", nodeProvisioningHandler.RebootNode).Methods("POST")
		nodeAPI.HandleFunc("/{id}/logs", nodeProvisioningHandler.GetNodeLogs).Methods("GET")

		// Configuration management
		configAPI := r.PathPrefix("/api/infrastructure/config").Subrouter()
		configAPI.Use(authMiddleware.Authenticate)
		configAPI.Use(authMiddleware.RequireRole("admin"))
		configAPI.HandleFunc("", nodeProvisioningHandler.ListConfigs).Methods("GET")
		configAPI.HandleFunc("", nodeProvisioningHandler.UpdateConfig).Methods("PUT")
	}

	// Protected API routes - Monitoring (TimescaleDB metrics)
	if monitoringHandler != nil {
		monitoringAPI := r.PathPrefix("/api/monitoring").Subrouter()
		monitoringAPI.Use(authMiddleware.Authenticate)
		monitoringAPI.Use(authMiddleware.RequireRole("admin"))

		// Dashboard overview
		monitoringAPI.HandleFunc("/dashboard", monitoringHandler.GetDashboardData).Methods("GET")

		// API Analytics
		monitoringAPI.HandleFunc("/api/analytics", monitoringHandler.GetAPIAnalytics).Methods("GET")
		monitoringAPI.HandleFunc("/api/top-endpoints", monitoringHandler.GetTopEndpoints).Methods("GET")
		monitoringAPI.HandleFunc("/api/slowest-endpoints", monitoringHandler.GetSlowestEndpoints).Methods("GET")
		monitoringAPI.HandleFunc("/api/logs", monitoringHandler.GetRecentAPILogs).Methods("GET")

		// Node Metrics
		monitoringAPI.HandleFunc("/nodes", monitoringHandler.GetLatestNodeMetrics).Methods("GET")
		monitoringAPI.HandleFunc("/nodes/{name}/history", monitoringHandler.GetNodeMetricsHistory).Methods("GET")
		monitoringAPI.HandleFunc("/cluster/overview", monitoringHandler.GetClusterOverview).Methods("GET")

		// PostgreSQL Metrics
		monitoringAPI.HandleFunc("/postgres", monitoringHandler.GetLatestPostgresMetrics).Methods("GET")
		monitoringAPI.HandleFunc("/postgres/overview", monitoringHandler.GetPostgresOverview).Methods("GET")

		// Alerts
		monitoringAPI.HandleFunc("/alerts/active", monitoringHandler.GetActiveAlerts).Methods("GET")
		monitoringAPI.HandleFunc("/alerts", monitoringHandler.GetRecentAlerts).Methods("GET")
		monitoringAPI.HandleFunc("/alerts/{id}/acknowledge", monitoringHandler.AcknowledgeAlert).Methods("POST")
		monitoringAPI.HandleFunc("/alerts/{id}/resolve", monitoringHandler.ResolveAlert).Methods("POST")
		monitoringAPI.HandleFunc("/alerts/summary", monitoringHandler.GetAlertSummary).Methods("GET")
		monitoringAPI.HandleFunc("/alerts/thresholds", monitoringHandler.GetAlertThresholds).Methods("GET")
		monitoringAPI.HandleFunc("/alerts/thresholds/{id}", monitoringHandler.UpdateAlertThreshold).Methods("PUT")

		// Backup History
		monitoringAPI.HandleFunc("/backups", monitoringHandler.GetRecentBackups).Methods("GET")
		monitoringAPI.HandleFunc("/backup-db", monitoringHandler.GetBackupDBStatus).Methods("GET")
	}

	// Protected API routes - Deployments (admin only)
	if deploymentHandler != nil {
		deployAPI := r.PathPrefix("/api/deployments").Subrouter()
		deployAPI.Use(authMiddleware.Authenticate)
		deployAPI.Use(authMiddleware.RequireRole("admin"))

		// Deployment configurations
		deployAPI.HandleFunc("", deploymentHandler.ListDeployments).Methods("GET")
		deployAPI.HandleFunc("/{id}", deploymentHandler.GetDeployment).Methods("GET")
		deployAPI.HandleFunc("/{id}/history", deploymentHandler.GetDeploymentHistory).Methods("GET")

		// Deployment operations
		deployAPI.HandleFunc("/{id}/deploy", deploymentHandler.Deploy).Methods("POST")           // SSE streaming
		deployAPI.HandleFunc("/{id}/deploy-sync", deploymentHandler.DeploySync).Methods("POST") // Non-streaming
		deployAPI.HandleFunc("/{id}/rollback", deploymentHandler.Rollback).Methods("POST")

		// Deployment status
		deployAPI.HandleFunc("/status/{historyId}", deploymentHandler.GetDeploymentStatus).Methods("GET") // SSE
	}

	// Health endpoints (basic health for K8s probes, detailed requires auth)
	r.HandleFunc("/health", healthHandler.BasicHealth).Methods("GET")
	r.HandleFunc("/health/ready", healthHandler.ReadinessHealth).Methods("GET")
	// Detailed health exposes internal info - require admin
	r.HandleFunc("/health/detailed", authMiddleware.Authenticate(authMiddleware.RequireAdmin(http.HandlerFunc(healthHandler.DetailedHealth))).ServeHTTP).Methods("GET")

	// Metrics endpoint - require admin authentication to protect internal metrics
	r.Handle("/metrics", authMiddleware.Authenticate(authMiddleware.RequireAdmin(promhttp.Handler())))

	return r
}

// NewCustomerRouter creates a router for customer portal (port 8081)
func NewCustomerRouter(
	customerPortalHandler *handlers.CustomerPortalHandler,
	pageHandler *handlers.PageHandler,
	healthHandler *handlers.HealthHandler,
	authMiddleware *middleware.AuthMiddleware,
) *mux.Router {
	r := mux.NewRouter()

	// Apply security middlewares
	r.Use(middleware.HTTPSRedirect)
	r.Use(middleware.SecurityHeaders)

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public routes - Customer portal login
	r.HandleFunc("/", pageHandler.CustomerPortalLoginPage).Methods("GET")
	r.HandleFunc("/login", pageHandler.CustomerPortalLoginPage).Methods("GET")
	r.HandleFunc("/dashboard", pageHandler.CustomerPortalDashboardPage).Methods("GET")

	// Public API - Simple authentication with rate limiting
	r.HandleFunc("/auth/login", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(customerPortalHandler.SimpleLogin)).ServeHTTP).Methods("POST")

	// Public API - OTP authentication with rate limiting
	r.HandleFunc("/auth/send-otp", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(customerPortalHandler.SendOTP)).ServeHTTP).Methods("POST")
	r.HandleFunc("/auth/verify-otp", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(customerPortalHandler.VerifyOTP)).ServeHTTP).Methods("POST")
	r.HandleFunc("/auth/validate-session", customerPortalHandler.ValidateSession).Methods("GET")
	r.HandleFunc("/auth/logout", customerPortalHandler.Logout).Methods("POST")

	// Protected API routes - Customer portal (requires customer JWT)
	customerAPI := r.PathPrefix("/api").Subrouter()
	customerAPI.Use(authMiddleware.AuthenticateCustomer)
	customerAPI.HandleFunc("/dashboard", customerPortalHandler.GetDashboard).Methods("GET")
	customerAPI.HandleFunc("/gate-pass-requests", customerPortalHandler.CreateGatePassRequest).Methods("POST")

	// Health endpoints - only basic health for K8s probes on customer portal
	// Detailed health and metrics are NOT exposed on customer portal for security
	r.HandleFunc("/health", healthHandler.BasicHealth).Methods("GET")
	r.HandleFunc("/health/ready", healthHandler.ReadinessHealth).Methods("GET")
	// Note: /health/detailed and /metrics are intentionally NOT exposed on customer portal

	return r
}
