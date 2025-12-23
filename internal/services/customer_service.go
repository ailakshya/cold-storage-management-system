package services

import (
	"context"
	"errors"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type CustomerService struct {
	Repo *repositories.CustomerRepository
}

func NewCustomerService(repo *repositories.CustomerRepository) *CustomerService {
	return &CustomerService{Repo: repo}
}

func (s *CustomerService) CreateCustomer(ctx context.Context, req *models.CreateCustomerRequest) (*models.Customer, error) {
	// Validate input
	if req.Name == "" || req.Phone == "" {
		return nil, errors.New("name and phone are required")
	}

	customer := &models.Customer{
		Name:    req.Name,
		Phone:   req.Phone,
		SO:      req.SO,
		Village: req.Village,
		Address: req.Address,
	}

	if err := s.Repo.Create(ctx, customer); err != nil {
		return nil, err
	}

	return customer, nil
}

func (s *CustomerService) GetCustomer(ctx context.Context, id int) (*models.Customer, error) {
	return s.Repo.Get(ctx, id)
}

func (s *CustomerService) SearchByPhone(ctx context.Context, phone string) (*models.Customer, error) {
	if phone == "" {
		return nil, errors.New("phone number is required")
	}
	return s.Repo.GetByPhone(ctx, phone)
}

// FuzzySearchByPhone searches customers by phone number (partial match)
func (s *CustomerService) FuzzySearchByPhone(ctx context.Context, phone string) ([]*models.Customer, error) {
	if phone == "" {
		return nil, errors.New("phone number is required")
	}
	return s.Repo.FuzzySearchByPhone(ctx, phone)
}

func (s *CustomerService) ListCustomers(ctx context.Context) ([]*models.Customer, error) {
	return s.Repo.List(ctx)
}

func (s *CustomerService) UpdateCustomer(ctx context.Context, id int, req *models.UpdateCustomerRequest) (*models.Customer, error) {
	// Validate input
	if req.Name == "" || req.Phone == "" {
		return nil, errors.New("name and phone are required")
	}

	customer := &models.Customer{
		ID:      id,
		Name:    req.Name,
		Phone:   req.Phone,
		SO:      req.SO,
		Village: req.Village,
		Address: req.Address,
	}

	if err := s.Repo.Update(ctx, customer); err != nil {
		return nil, err
	}

	return s.Repo.Get(ctx, id)
}

func (s *CustomerService) DeleteCustomer(ctx context.Context, id int) error {
	return s.Repo.Delete(ctx, id)
}
