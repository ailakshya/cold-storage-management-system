package http

import (
	"net/http"
	"github.com/gorilla/mux"
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
	adminActionLogHandler *handlers.AdminActionLogHandler,
	gatePassHandler *handlers.GatePassHandler,
	pageHandler *handlers.PageHandler,
	authMiddleware *middleware.AuthMiddleware,
) *mux.Router {
	r := mux.NewRouter()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public HTML pages
	r.HandleFunc("/", pageHandler.LoginPage).Methods("GET")
	r.HandleFunc("/login", pageHandler.LoginPage).Methods("GET")

	// API routes - Authentication
	r.HandleFunc("/auth/signup", authHandler.Signup).Methods("POST")
	r.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	// Logout route
	r.HandleFunc("/logout", pageHandler.LogoutPage).Methods("GET")

	// Protected HTML pages
	r.HandleFunc("/dashboard", pageHandler.DashboardPage).Methods("GET")
	r.HandleFunc("/admin/dashboard", pageHandler.AdminDashboardPage).Methods("GET")
	r.HandleFunc("/accountant/dashboard", pageHandler.AccountantDashboardPage).Methods("GET")
	r.HandleFunc("/item-search", pageHandler.ItemSearchPage).Methods("GET")
	r.HandleFunc("/events", pageHandler.EventTracerPage).Methods("GET")
	r.HandleFunc("/entry-room", pageHandler.EntryRoomPage).Methods("GET")
	r.HandleFunc("/main-entry", pageHandler.MainEntryPage).Methods("GET")
	r.HandleFunc("/room-config-1", pageHandler.RoomConfig1Page).Methods("GET")
	r.HandleFunc("/room-form-2", pageHandler.RoomForm2Page).Methods("GET")
	r.HandleFunc("/loading-invoice", pageHandler.LoadingInvoicePage).Methods("GET")
	r.HandleFunc("/rent", pageHandler.RentPage).Methods("GET")
	r.HandleFunc("/rent-management", pageHandler.RentManagementPage).Methods("GET")
	r.HandleFunc("/room-entry-edit", pageHandler.RoomEntryEditPage).Methods("GET")
	r.HandleFunc("/payment-receipt", pageHandler.PaymentReceiptPage).Methods("GET")
	r.HandleFunc("/verify-receipt", pageHandler.VerifyReceiptPage).Methods("GET")

	// Unloading mode pages (for gate pass system)
	r.HandleFunc("/gate-pass-entry", pageHandler.GatePassEntryPage).Methods("GET")
	r.HandleFunc("/unloading-tickets", pageHandler.UnloadingTicketsPage).Methods("GET")

	// Employee management page (admin only)
	r.HandleFunc("/employees", pageHandler.EmployeesPage).Methods("GET")

	// System settings page (admin only)
	r.HandleFunc("/system-settings", pageHandler.SystemSettingsPage).Methods("GET")

	// Admin reports page (admin only)
	r.HandleFunc("/admin/report", pageHandler.AdminReportPage).Methods("GET")

	// Protected API routes - System Settings
	settingsAPI := r.PathPrefix("/api/settings").Subrouter()
	settingsAPI.Use(authMiddleware.Authenticate)
	settingsAPI.HandleFunc("", systemSettingHandler.ListSettings).Methods("GET")
	settingsAPI.HandleFunc("/{key}", systemSettingHandler.GetSetting).Methods("GET")
	settingsAPI.HandleFunc("/{key}", systemSettingHandler.UpdateSetting).Methods("PUT")

	// Protected API routes - Users
	usersAPI := r.PathPrefix("/api/users").Subrouter()
	usersAPI.Use(authMiddleware.Authenticate)
	usersAPI.HandleFunc("", userHandler.ListUsers).Methods("GET")
	usersAPI.HandleFunc("", userHandler.CreateUser).Methods("POST")
	usersAPI.HandleFunc("/{id}", userHandler.GetUser).Methods("GET")
	usersAPI.HandleFunc("/{id}", userHandler.UpdateUser).Methods("PUT")
	usersAPI.HandleFunc("/{id}", userHandler.DeleteUser).Methods("DELETE")
	usersAPI.HandleFunc("/{id}/toggle-active", userHandler.ToggleActiveStatus).Methods("PATCH")

	// Protected API routes - Customers
	customersAPI := r.PathPrefix("/api/customers").Subrouter()
	customersAPI.Use(authMiddleware.Authenticate)
	customersAPI.HandleFunc("", customerHandler.ListCustomers).Methods("GET")
	customersAPI.HandleFunc("", customerHandler.CreateCustomer).Methods("POST")
	customersAPI.HandleFunc("/search", customerHandler.SearchByPhone).Methods("GET")
	customersAPI.HandleFunc("/{id}", customerHandler.GetCustomer).Methods("GET")
	customersAPI.HandleFunc("/{id}", customerHandler.UpdateCustomer).Methods("PUT")
	customersAPI.HandleFunc("/{id}", customerHandler.DeleteCustomer).Methods("DELETE")

	// Protected API routes - Entries (employees and admins only for creation)
	entriesAPI := r.PathPrefix("/api/entries").Subrouter()
	entriesAPI.Use(authMiddleware.Authenticate)
	entriesAPI.HandleFunc("", entryHandler.ListEntries).Methods("GET") // All authenticated users can view
	entriesAPI.HandleFunc("", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(entryHandler.CreateEntry)).ServeHTTP).Methods("POST")
	entriesAPI.HandleFunc("/count", entryHandler.GetCountByCategory).Methods("GET")
	entriesAPI.HandleFunc("/unassigned", roomEntryHandler.GetUnassignedEntries).Methods("GET")
	entriesAPI.HandleFunc("/{id}", entryHandler.GetEntry).Methods("GET")
	entriesAPI.HandleFunc("/customer/{customer_id}", entryHandler.ListEntriesByCustomer).Methods("GET")

	// Protected API routes - Room Entries (employees and admins only for creation/update)
	roomEntriesAPI := r.PathPrefix("/api/room-entries").Subrouter()
	roomEntriesAPI.Use(authMiddleware.Authenticate)
	roomEntriesAPI.HandleFunc("", roomEntryHandler.ListRoomEntries).Methods("GET") // All authenticated users can view
	roomEntriesAPI.HandleFunc("", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(roomEntryHandler.CreateRoomEntry)).ServeHTTP).Methods("POST")
	roomEntriesAPI.HandleFunc("/{id}", roomEntryHandler.GetRoomEntry).Methods("GET")
	roomEntriesAPI.HandleFunc("/{id}", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(roomEntryHandler.UpdateRoomEntry)).ServeHTTP).Methods("PUT")

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

	// Protected API routes - Admin Action Logs (admin only)
	adminActionLogsAPI := r.PathPrefix("/api/admin-action-logs").Subrouter()
	adminActionLogsAPI.Use(authMiddleware.Authenticate)
	adminActionLogsAPI.HandleFunc("", authMiddleware.RequireRole("admin")(http.HandlerFunc(adminActionLogHandler.ListActionLogs)).ServeHTTP).Methods("GET")

	// Protected API routes - Gate Passes (for unloading mode)
	gatePassAPI := r.PathPrefix("/api/gate-passes").Subrouter()
	gatePassAPI.Use(authMiddleware.Authenticate)
	gatePassAPI.HandleFunc("", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.CreateGatePass)).ServeHTTP).Methods("POST")
	gatePassAPI.HandleFunc("", gatePassHandler.ListAllGatePasses).Methods("GET")
	gatePassAPI.HandleFunc("/pending", gatePassHandler.ListPendingGatePasses).Methods("GET")
	gatePassAPI.HandleFunc("/approved", gatePassHandler.ListApprovedGatePasses).Methods("GET")
	gatePassAPI.HandleFunc("/expired", gatePassHandler.GetExpiredGatePasses).Methods("GET")
	gatePassAPI.HandleFunc("/{id}/approve", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.ApproveGatePass)).ServeHTTP).Methods("PUT")
	gatePassAPI.HandleFunc("/{id}/complete", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.CompleteGatePass)).ServeHTTP).Methods("POST")
	gatePassAPI.HandleFunc("/{id}/pickups", gatePassHandler.GetPickupHistory).Methods("GET")
	gatePassAPI.HandleFunc("/pickup", authMiddleware.RequireRole("employee", "admin")(http.HandlerFunc(gatePassHandler.RecordPickup)).ServeHTTP).Methods("POST")

	return r
}

// NewCustomerRouter creates a router for customer portal (port 8081)
func NewCustomerRouter(
	customerPortalHandler *handlers.CustomerPortalHandler,
	pageHandler *handlers.PageHandler,
	authMiddleware *middleware.AuthMiddleware,
) *mux.Router {
	r := mux.NewRouter()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public routes - Customer portal login
	r.HandleFunc("/", pageHandler.CustomerPortalLoginPage).Methods("GET")
	r.HandleFunc("/login", pageHandler.CustomerPortalLoginPage).Methods("GET")
	r.HandleFunc("/dashboard", pageHandler.CustomerPortalDashboardPage).Methods("GET")

	// Public API - Simple authentication (phone + truck number)
	r.HandleFunc("/auth/login", customerPortalHandler.SimpleLogin).Methods("POST")

	// Public API - OTP authentication (for future use when SMS is ready)
	r.HandleFunc("/auth/send-otp", customerPortalHandler.SendOTP).Methods("POST")
	r.HandleFunc("/auth/verify-otp", customerPortalHandler.VerifyOTP).Methods("POST")
	r.HandleFunc("/auth/validate-session", customerPortalHandler.ValidateSession).Methods("GET")
	r.HandleFunc("/auth/logout", customerPortalHandler.Logout).Methods("POST")

	// Protected API routes - Customer portal (requires customer JWT)
	customerAPI := r.PathPrefix("/api").Subrouter()
	customerAPI.Use(authMiddleware.AuthenticateCustomer)
	customerAPI.HandleFunc("/dashboard", customerPortalHandler.GetDashboard).Methods("GET")
	customerAPI.HandleFunc("/gate-pass-requests", customerPortalHandler.CreateGatePassRequest).Methods("POST")

	return r
}
