package services

import (
	"context"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type RentPaymentService struct {
	Repo *repositories.RentPaymentRepository
}

func NewRentPaymentService(repo *repositories.RentPaymentRepository) *RentPaymentService {
	return &RentPaymentService{Repo: repo}
}

func (s *RentPaymentService) CreatePayment(ctx context.Context, payment *models.RentPayment) error {
	return s.Repo.Create(ctx, payment)
}

func (s *RentPaymentService) GetPaymentsByEntryID(ctx context.Context, entryID int) ([]*models.RentPayment, error) {
	return s.Repo.GetByEntryID(ctx, entryID)
}

func (s *RentPaymentService) GetPaymentsByPhone(ctx context.Context, phone string) ([]*models.RentPayment, error) {
	return s.Repo.GetByPhone(ctx, phone)
}

func (s *RentPaymentService) ListPayments(ctx context.Context) ([]*models.RentPayment, error) {
	return s.Repo.List(ctx)
}

func (s *RentPaymentService) GetPaymentByReceiptNumber(ctx context.Context, receiptNumber string) (*models.RentPayment, error) {
	return s.Repo.GetByReceiptNumber(ctx, receiptNumber)
}
