package http

import (
	"io/fs"
	"net/http"

	"cold-backend/internal/handlers"
	"cold-backend/internal/middleware"
	"cold-backend/static"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	entryManagementLogHandler *handlers.EntryManagementLogHandler,
	adminActionLogHandler *handlers.AdminActionLogHandler,
	gatePassHandler *handlers.GatePassHandler,
	seasonHandler *handlers.SeasonHandler,
	guardEntryHandler *handlers.GuardEntryHandler,
	tokenColorHandler *handlers.TokenColorHandler,
	pageHandler *handlers.PageHandler,
	healthHandler *handlers.HealthHandler,
	authMiddleware *middleware.AuthMiddleware,
	operationModeMiddleware *middleware.OperationModeMiddleware,
	monitoringHandler *handlers.MonitoringHandler,
	apiLoggingMiddleware *middleware.APILoggingMiddleware,
	nodeProvisioningHandler *handlers.NodeProvisioningHandler,
	deploymentHandler *handlers.DeploymentHandler,
	reportHandler *handlers.ReportHandler,
	accountHandler *handlers.AccountHandler,
	entryRoomHandler *handlers.EntryRoomHandler,
	roomVisualizationHandler *handlers.RoomVisualizationHandler,
	setupHandler *handlers.SetupHandler,
	ledgerHandler *handlers.LedgerHandler,
	debtHandler *handlers.DebtHandler,
	mergeHistoryHandler *handlers.MergeHistoryHandler,
) *mux.Router {
	r := mux.NewRouter()

	// Apply security middlewares first
	r.Use(middleware.HTTPSRedirect)
	r.Use(middleware.SecurityHeaders)

	// Apply API logging middleware to all routes (if enabled)
	if apiLoggingMiddleware != nil {
		r.Use(apiLoggingMiddleware.Handler)
	}

	// Serve static files from embedded filesystem
	staticFS, _ := fs.Sub(static.FS, ".")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Public HTML pages (NO AUTHENTICATION REQUIRED)
	r.HandleFunc("/", pageHandler.LoginPage).Methods("GET")
	r.HandleFunc("/login", pageHandler.LoginPage).Methods("GET")
	r.HandleFunc("/logout", pageHandler.LogoutPage).Methods("GET")

	// Public API routes - Authentication (with rate limiting)
	r.HandleFunc("/auth/signup", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(authHandler.Signup)).ServeHTTP).Methods("POST")
	r.HandleFunc("/auth/login", middleware.LoginRateLimiter.Middleware(http.HandlerFunc(authHandler.Login)).ServeHTTP).Methods("POST")

	// Setup routes - Always available for disaster recovery
	// Allows restoring from R2 backup even when DB is connected
	if setupHandler != nil {
		r.HandleFunc("/setup", setupHandler.SetupPage).Methods("GET")
		r.HandleFunc("/setup/test", setupHandler.TestConnection).Methods("POST")
		r.HandleFunc("/setup/save", setupHandler.SaveConfig).Methods("POST")
		r.HandleFunc("/setup/r2-check", setupHandler.CheckR2Connection).Methods("GET")
		r.HandleFunc("/setup/backups", setupHandler.ListBackups).Methods("GET")
		r.HandleFunc("/setup/restore", setupHandler.RestoreFromR2).Methods("POST")
	}

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
	r.HandleFunc("/account/audit", pageHandler.AccountAuditPage).Methods("GET")
	r.HandleFunc("/account/debtors", pageHandler.AccountDebtorsPage).Methods("GET")
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
	r.HandleFunc("/room-visualization", pageHandler.RoomVisualizationPage).Methods("GET")
	r.HandleFunc("/customer-export", pageHandler.CustomerPDFExportPage).Methods("GET")
	r.HandleFunc("/customer-edit", pageHandler.CustomerEditPage).Methods("GET")
	r.HandleFunc("/merge-history", pageHandler.MergeHistoryPage).Methods("GET")

	// Guard pages (auth handled client-side via localStorage token)
	r.HandleFunc("/guard/dashboard", pageHandler.GuardDashboardPage).Methods("GET")
	r.HandleFunc("/guard/register", pageHandler.GuardRegisterPage).Methods("GET")

	// Protected API routes - System Settings
	settingsAPI := r.PathPrefix("/api/settings").Subrouter()
	settingsAPI.Use(authMiddleware.Authenticate)
	settingsAPI.HandleFunc("", systemSettingHandler.ListSettings).Methods("GET")
	settingsAPI.HandleFunc("/operation_mode", systemSettingHandler.GetOperationMode).Methods("GET")
	settingsAPI.HandleFunc("/skip_thock_ranges", systemSettingHandler.GetSkipThockRanges).Methods("GET")
	settingsAPI.HandleFunc("/skip_thock_ranges", authMiddleware.RequireAdmin(http.HandlerFunc(systemSettingHandler.UpdateSkipThockRanges)).ServeHTTP).Methods("PUT")
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
	customersAPI.HandleFunc("/merge", customerHandler.MergeCustomers).Methods("POST") // Requires can_manage_entries permission
	customersAPI.HandleFunc("/{id}", customerHandler.GetCustomer).Methods("GET")
	customersAPI.HandleFunc("/{id}", customerHandler.UpdateCustomer).Methods("PUT")
	customersAPI.HandleFunc("/{id}", customerHandler.DeleteCustomer).Methods("DELETE")
	customersAPI.HandleFunc("/{id}/entry-count", customerHandler.GetCustomerEntryCount).Methods("GET")

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
	entriesAPI.HandleFunc("/next-thock-preview", entryHandler.GetNextThockPreview).Methods("GET")
	entriesAPI.HandleFunc("/{id}", entryHandler.GetEntry).Methods("GET")
	entriesAPI.HandleFunc("/{id}", entryHandler.UpdateEntry).Methods("PUT")
	entriesAPI.HandleFunc("/{id}/reassign", entryHandler.ReassignEntry).Methods("PUT") // Requires can_manage_entries permission
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

	// Protected API routes - Entry Management Logs (admin only) - for reassignments and merges
	entryManagementLogsAPI := r.PathPrefix("/api/entry-management-logs").Subrouter()
	entryManagementLogsAPI.Use(authMiddleware.Authenticate)
	entryManagementLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(entryManagementLogHandler.List)).ServeHTTP).Methods("GET")

	// Protected API routes - Admin Action Logs (admin only)
	adminActionLogsAPI := r.PathPrefix("/api/admin-action-logs").Subrouter()
	adminActionLogsAPI.Use(authMiddleware.Authenticate)
	adminActionLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(adminActionLogHandler.ListActionLogs)).ServeHTTP).Methods("GET")

	// Protected API routes - Merge History (admin only)
	if mergeHistoryHandler != nil {
		mergeHistoryAPI := r.PathPrefix("/api/merge-history").Subrouter()
		mergeHistoryAPI.Use(authMiddleware.Authenticate)
		mergeHistoryAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(mergeHistoryHandler.GetMergeHistory)).ServeHTTP).Methods("GET")
		mergeHistoryAPI.HandleFunc("/undo-merge", authMiddleware.RequireRole("admin")(http.HandlerFunc(mergeHistoryHandler.UndoMerge)).ServeHTTP).Methods("POST")
		mergeHistoryAPI.HandleFunc("/undo-transfer", authMiddleware.RequireRole("admin")(http.HandlerFunc(mergeHistoryHandler.UndoTransfer)).ServeHTTP).Methods("POST")
	}

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
	// API accessible by guard, employee, admin - but guard pages only for guard role
	if guardEntryHandler != nil {
		guardAPI := r.PathPrefix("/api/guard").Subrouter()
		guardAPI.Use(authMiddleware.Authenticate)

		// Create and list entries - accessible by guard, employee, admin
		guardAPI.HandleFunc("/entries", authMiddleware.RequireRole("guard", "employee", "admin")(
			http.HandlerFunc(guardEntryHandler.CreateGuardEntry),
		).ServeHTTP).Methods("POST")
		guardAPI.HandleFunc("/entries", authMiddleware.RequireRole("guard", "employee", "admin")(
			http.HandlerFunc(guardEntryHandler.ListMyEntries),
		).ServeHTTP).Methods("GET")
		guardAPI.HandleFunc("/stats", authMiddleware.RequireRole("guard", "employee", "admin")(
			http.HandlerFunc(guardEntryHandler.GetMyStats),
		).ServeHTTP).Methods("GET")

		// Pending entries - accessible by guard, employee, admin
		guardAPI.HandleFunc("/entries/pending", authMiddleware.RequireRole("guard", "employee", "admin")(
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

		// Process portion (seed or sell) - only employee or admin
		guardAPI.HandleFunc("/entries/{id}/process/{portion}", authMiddleware.RequireRole("employee", "admin")(
			http.HandlerFunc(guardEntryHandler.ProcessPortion),
		).ServeHTTP).Methods("PUT")

		// Delete entry - admin only
		guardAPI.HandleFunc("/entries/{id}", authMiddleware.RequireRole("admin")(
			http.HandlerFunc(guardEntryHandler.DeleteGuardEntry),
		).ServeHTTP).Methods("DELETE")
	}

	// Token Color API routes
	if tokenColorHandler != nil {
		tokenColorAPI := r.PathPrefix("/api/token-color").Subrouter()

		// Public endpoint for guards to get today's color (requires only auth)
		tokenColorAPI.HandleFunc("/today", authMiddleware.Authenticate(
			http.HandlerFunc(tokenColorHandler.GetTodayColor),
		).ServeHTTP).Methods("GET")

		// Get color by date (requires auth)
		tokenColorAPI.HandleFunc("", authMiddleware.Authenticate(
			http.HandlerFunc(tokenColorHandler.GetColorByDate),
		).ServeHTTP).Methods("GET")

		// Get upcoming colors (requires auth)
		tokenColorAPI.HandleFunc("/upcoming", authMiddleware.Authenticate(
			http.HandlerFunc(tokenColorHandler.GetUpcoming),
		).ServeHTTP).Methods("GET")

		// Set color for a date - admin only
		tokenColorAPI.HandleFunc("", authMiddleware.Authenticate(
			authMiddleware.RequireAdmin(http.HandlerFunc(tokenColorHandler.SetColor)),
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
	infraAPI.HandleFunc("/backup-history", infraHandler.GetBackupHistory).Methods("GET")
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
	infraAPI.HandleFunc("/download-database", authMiddleware.RequireAdmin(http.HandlerFunc(infraHandler.DownloadDatabase)).ServeHTTP).Methods("GET")

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

		// R2 Cloud Storage Status
		monitoringAPI.HandleFunc("/r2-status", monitoringHandler.GetR2Status).Methods("GET")
		monitoringAPI.HandleFunc("/backup-r2", monitoringHandler.BackupToR2).Methods("POST")
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

	// Protected API routes - Reports (admin and accountant access)
	if reportHandler != nil {
		reportAPI := r.PathPrefix("/api/reports").Subrouter()
		reportAPI.Use(authMiddleware.Authenticate)
		reportAPI.Use(authMiddleware.RequireAccountantAccess)

		// Customer reports
		reportAPI.HandleFunc("/customers/csv", reportHandler.GetCustomersCSV).Methods("GET")
		reportAPI.HandleFunc("/customers/pdf", reportHandler.GetCustomersPDFZip).Methods("GET")
		reportAPI.HandleFunc("/customers/{phone}/pdf", reportHandler.GetSingleCustomerPDF).Methods("GET")

		// Daily summary reports
		reportAPI.HandleFunc("/daily-summary/csv", reportHandler.GetDailySummaryCSV).Methods("GET")
		reportAPI.HandleFunc("/daily-summary/pdf", reportHandler.GetDailySummaryPDF).Methods("GET")

		// Report stats (for UI)
		reportAPI.HandleFunc("/stats", reportHandler.GetReportStats).Methods("GET")
	}

	// Protected API routes - Account Summary (optimized single-call endpoint)
	if accountHandler != nil {
		accountAPI := r.PathPrefix("/api/accounts").Subrouter()
		accountAPI.Use(authMiddleware.Authenticate)
		accountAPI.HandleFunc("/summary", authMiddleware.RequireAccountantAccess(http.HandlerFunc(accountHandler.GetAccountSummary)).ServeHTTP).Methods("GET")
	}

	// Protected API routes - Ledger (accounting ledger)
	if ledgerHandler != nil {
		ledgerAPI := r.PathPrefix("/api/ledger").Subrouter()
		ledgerAPI.Use(authMiddleware.Authenticate)
		// Customer ledger - any authenticated user can view their ledger
		ledgerAPI.HandleFunc("/customer/{phone}", ledgerHandler.GetCustomerLedger).Methods("GET")
		ledgerAPI.HandleFunc("/balance/{phone}", ledgerHandler.GetCustomerBalance).Methods("GET")
		ledgerAPI.HandleFunc("/summary/{phone}", ledgerHandler.GetCustomerSummary).Methods("GET")
		// Admin/accountant only endpoints
		ledgerAPI.HandleFunc("/audit", authMiddleware.RequireAccountantAccess(http.HandlerFunc(ledgerHandler.GetAuditTrail)).ServeHTTP).Methods("GET")
		ledgerAPI.HandleFunc("/debtors", authMiddleware.RequireAccountantAccess(http.HandlerFunc(ledgerHandler.GetDebtors)).ServeHTTP).Methods("GET")
		ledgerAPI.HandleFunc("/balances", authMiddleware.RequireAccountantAccess(http.HandlerFunc(ledgerHandler.GetAllBalances)).ServeHTTP).Methods("GET")
		ledgerAPI.HandleFunc("/totals", authMiddleware.RequireAccountantAccess(http.HandlerFunc(ledgerHandler.GetTotalsByType)).ServeHTTP).Methods("GET")
		// Admin only - create manual entries
		ledgerAPI.HandleFunc("/entry", authMiddleware.RequireAdmin(http.HandlerFunc(ledgerHandler.CreateEntry)).ServeHTTP).Methods("POST")
	}

	// Protected API routes - Debt Requests (debt approval workflow)
	if debtHandler != nil {
		debtAPI := r.PathPrefix("/api/debt-requests").Subrouter()
		debtAPI.Use(authMiddleware.Authenticate)
		// Employee/admin can create debt requests
		debtAPI.HandleFunc("", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(debtHandler.CreateDebtRequest)).ServeHTTP).Methods("POST")
		// Admin only - pending requests and management
		debtAPI.HandleFunc("/pending", authMiddleware.RequireAdmin(http.HandlerFunc(debtHandler.GetPendingRequests)).ServeHTTP).Methods("GET")
		debtAPI.HandleFunc("/summary", debtHandler.GetPendingSummary).Methods("GET")
		// Check for approved debt (used by gate pass)
		debtAPI.HandleFunc("/check", debtHandler.CheckDebtApproval).Methods("GET")
		// Get requests by customer
		debtAPI.HandleFunc("/customer/{phone}", debtHandler.GetCustomerRequests).Methods("GET")
		// Get single request
		debtAPI.HandleFunc("/{id}", debtHandler.GetDebtRequest).Methods("GET")
		// Admin approval/rejection
		debtAPI.HandleFunc("/{id}/approve", authMiddleware.RequireAdmin(http.HandlerFunc(debtHandler.ApproveDebtRequest)).ServeHTTP).Methods("PUT")
		debtAPI.HandleFunc("/{id}/reject", authMiddleware.RequireAdmin(http.HandlerFunc(debtHandler.RejectDebtRequest)).ServeHTTP).Methods("PUT")
		debtAPI.HandleFunc("/{id}/use", authMiddleware.RequireAdmin(http.HandlerFunc(debtHandler.UseDebtApproval)).ServeHTTP).Methods("PUT")
		// Admin/accountant - all requests with filters (permission checked in handler)
		debtAPI.HandleFunc("", debtHandler.GetAllRequests).Methods("GET")
	}

	// Protected API routes - Entry Room (optimized single-call endpoint)
	if entryRoomHandler != nil {
		entryRoomAPI := r.PathPrefix("/api/entry-room").Subrouter()
		entryRoomAPI.Use(authMiddleware.Authenticate)
		entryRoomAPI.HandleFunc("/summary", entryRoomHandler.GetSummary).Methods("GET")
		entryRoomAPI.HandleFunc("/since", entryRoomHandler.GetDelta).Methods("GET")
	}

	// Protected API routes - Room Visualization (all authenticated users)
	if roomVisualizationHandler != nil {
		vizAPI := r.PathPrefix("/api/room-visualization").Subrouter()
		vizAPI.Use(authMiddleware.Authenticate)
		vizAPI.HandleFunc("/stats", roomVisualizationHandler.GetRoomStats).Methods("GET")
		vizAPI.HandleFunc("/gatar", roomVisualizationHandler.GetGatarOccupancy).Methods("GET")
		vizAPI.HandleFunc("/gatar-details", roomVisualizationHandler.GetGatarDetails).Methods("GET")
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

	// Serve static files from embedded filesystem
	staticFS, _ := fs.Sub(static.FS, ".")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

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
