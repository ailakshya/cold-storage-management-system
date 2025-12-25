package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
)

type PageHandler struct {
	templates *template.Template
}

func NewPageHandler() *PageHandler {
	// Parse all templates
	templates := template.Must(template.ParseGlob(filepath.Join("templates", "*.html")))

	return &PageHandler{
		templates: templates,
	}
}

// LoginPage serves the login page
func (h *PageHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "user_login.html", nil)
}

// DashboardPage serves the dashboard (check session/auth first)
func (h *PageHandler) DashboardPage(w http.ResponseWriter, r *http.Request) {
	// TODO: Check if user is authenticated via session or JWT
	h.templates.ExecuteTemplate(w, "dashboard_employee.html", nil)
}

// AdminDashboardPage serves admin dashboard
func (h *PageHandler) AdminDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "dashboard_admin.html", nil)
}

// AccountantDashboardPage serves accountant dashboard
func (h *PageHandler) AccountantDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "dashboard_accountant.html", nil)
}

// ItemSearchPage serves item search page
func (h *PageHandler) ItemSearchPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "itam_serch.html", nil)
}

// EventTracerPage serves event tracer page
func (h *PageHandler) EventTracerPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "event_tracer.html", nil)
}

// EntryRoomPage serves entry room page
func (h *PageHandler) EntryRoomPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "entry_room.html", nil)
}

// MainEntryPage serves main entry page
func (h *PageHandler) MainEntryPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "main_entry.html", nil)
}

// RoomForm1Page serves room form 1
func (h *PageHandler) RoomForm1Page(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "room_form_1.html", nil)
}

// RoomForm2Page serves room form 2
func (h *PageHandler) RoomForm2Page(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "room_form_2.html", nil)
}

// LoadingInvoicePage serves loading invoice page
func (h *PageHandler) LoadingInvoicePage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "loding_invoice.html", nil)
}

// RoomConfig1Page serves room configuration 1
func (h *PageHandler) RoomConfig1Page(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "room-config-1.html", nil)
}

// LogoutPage handles logout
func (h *PageHandler) LogoutPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "logout.html", nil)
}

// EmployeesPage serves employee management page
func (h *PageHandler) EmployeesPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "employees.html", nil)
}

// SystemSettingsPage serves system settings page
func (h *PageHandler) SystemSettingsPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "system_settings.html", nil)
}

// RentPage serves rent payment page
func (h *PageHandler) RentPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "rent.html", nil)
}

// RentManagementPage serves rent management page
func (h *PageHandler) RentManagementPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "rent_management.html", nil)
}

// RoomEntryEditPage serves room entry edit page
func (h *PageHandler) RoomEntryEditPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "room_entry_edit.html", nil)
}

// PaymentReceiptPage serves payment receipt page
func (h *PageHandler) PaymentReceiptPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "payment_receipt.html", nil)
}

// VerifyReceiptPage serves receipt verification page
func (h *PageHandler) VerifyReceiptPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "verify_receipt.html", nil)
}

// AdminReportPage serves admin reports and logs page
func (h *PageHandler) AdminReportPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "admin_report.html", nil)
}

// AdminLogsPage serves system logs page
func (h *PageHandler) AdminLogsPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "admin_logs.html", nil)
}

// GatePassEntryPage serves gate pass entry page (unloading mode)
func (h *PageHandler) GatePassEntryPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "gate_pass_entry.html", nil)
}

// UnloadingTicketsPage serves unloading tickets page (unloading mode)
func (h *PageHandler) UnloadingTicketsPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "unloading_tickets.html", nil)
}

// CustomerPortalLoginPage serves customer portal login page
func (h *PageHandler) CustomerPortalLoginPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "customer_portal_login.html", nil)
}

// CustomerPortalDashboardPage serves customer portal dashboard page
func (h *PageHandler) CustomerPortalDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "customer_portal_dashboard.html", nil)
}

// InfrastructureMonitoringPage serves infrastructure monitoring page
func (h *PageHandler) InfrastructureMonitoringPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "infrastructure_monitoring.html", nil)
}

// NodeProvisioningPage serves the node provisioning and cluster management page
func (h *PageHandler) NodeProvisioningPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "node_provisioning.html", nil)
}

// MonitoringDashboardPage serves the enhanced monitoring dashboard
func (h *PageHandler) MonitoringDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "monitoring_dashboard.html", nil)
}

// RoomVisualizationPage serves the room visualization page for storage occupancy
func (h *PageHandler) RoomVisualizationPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "room_visualization.html", nil)
}

// CustomerPDFExportPage serves the customer PDF export page
func (h *PageHandler) CustomerPDFExportPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "customer_pdf_export.html", nil)
}

// CustomerEditPage serves the customer edit/management page
func (h *PageHandler) CustomerEditPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "customer_edit.html", nil)
}

// GuardDashboardPage serves the guard dashboard page
func (h *PageHandler) GuardDashboardPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "dashboard_guard.html", nil)
}

// GuardRegisterPage serves the guard register page
func (h *PageHandler) GuardRegisterPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "guard_register.html", nil)
}
