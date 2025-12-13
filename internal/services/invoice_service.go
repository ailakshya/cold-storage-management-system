package services

import (
	"context"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type InvoiceService struct {
	repo *repositories.InvoiceRepository
}

func NewInvoiceService(repo *repositories.InvoiceRepository) *InvoiceService {
	return &InvoiceService{repo: repo}
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, req *models.CreateInvoiceRequest) (*models.Invoice, error) {
	invoice := &models.Invoice{
		CustomerID:  &req.CustomerID,
		EmployeeID:  &req.EmployeeID,
		TotalAmount: req.TotalAmount,
		Notes:       req.Notes,
	}

	err := s.repo.Create(ctx, invoice, req.Items)
	if err != nil {
		return nil, err
	}

	return invoice, nil
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id int) (*models.InvoiceWithDetails, error) {
	return s.repo.Get(ctx, id)
}

func (s *InvoiceService) GetInvoiceByNumber(ctx context.Context, invoiceNumber string) (*models.InvoiceWithDetails, error) {
	return s.repo.GetByInvoiceNumber(ctx, invoiceNumber)
}

func (s *InvoiceService) ListInvoices(ctx context.Context) ([]*models.InvoiceWithDetails, error) {
	return s.repo.List(ctx)
}

func (s *InvoiceService) GetCustomerInvoices(ctx context.Context, customerID int) ([]*models.Invoice, error) {
	return s.repo.GetByCustomer(ctx, customerID)
}
