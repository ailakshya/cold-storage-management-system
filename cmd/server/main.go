package main

import (
	"fmt"
	"log"
	"net/http"

	"cold-backend/internal/auth"
	"cold-backend/internal/config"
	"cold-backend/internal/db"
	h "cold-backend/internal/http"
	"cold-backend/internal/handlers"
	"cold-backend/internal/middleware"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database
	pool := db.Connect(cfg)
	defer pool.Close()

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
	invoiceRepo := repositories.NewInvoiceRepository(pool)

	// Initialize services
	userService := services.NewUserService(userRepo, jwtManager)
	customerService := services.NewCustomerService(customerRepo)
	entryService := services.NewEntryService(entryRepo, customerRepo, entryEventRepo)
	roomEntryService := services.NewRoomEntryService(roomEntryRepo, entryRepo, entryEventRepo)
	systemSettingService := services.NewSystemSettingService(systemSettingRepo)
	rentPaymentService := services.NewRentPaymentService(rentPaymentRepo)
	invoiceService := services.NewInvoiceService(invoiceRepo)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)
	authHandler := handlers.NewAuthHandler(userService)
	customerHandler := handlers.NewCustomerHandler(customerService)
	entryHandler := handlers.NewEntryHandler(entryService)
	roomEntryHandler := handlers.NewRoomEntryHandler(roomEntryService)
	entryEventHandler := handlers.NewEntryEventHandler(entryEventRepo)
	systemSettingHandler := handlers.NewSystemSettingHandler(systemSettingService)
	rentPaymentHandler := handlers.NewRentPaymentHandler(rentPaymentService)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceService)
	pageHandler := handlers.NewPageHandler()

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)
	corsMiddleware := middleware.NewCORS(cfg)

	// Create router
	router := h.NewRouter(userHandler, authHandler, customerHandler, entryHandler, roomEntryHandler, entryEventHandler, systemSettingHandler, rentPaymentHandler, invoiceHandler, pageHandler, authMiddleware)

	// Wrap router with CORS
	handler := corsMiddleware(router)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("Server running on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
