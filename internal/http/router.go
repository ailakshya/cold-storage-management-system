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

	// Employee management page (admin only)
	r.HandleFunc("/employees", pageHandler.EmployeesPage).Methods("GET")

	// System settings page (admin only)
	r.HandleFunc("/system-settings", pageHandler.SystemSettingsPage).Methods("GET")

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

	return r
}
