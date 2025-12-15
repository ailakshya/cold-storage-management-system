package services

import (
	"context"
	"errors"
	"time"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type GatePassService struct {
	GatePassRepo       *repositories.GatePassRepository
	EntryRepo          *repositories.EntryRepository
	EntryEventRepo     *repositories.EntryEventRepository
	PickupRepo         *repositories.GatePassPickupRepository
	RoomEntryRepo      *repositories.RoomEntryRepository
}

func NewGatePassService(
	gatePassRepo *repositories.GatePassRepository,
	entryRepo *repositories.EntryRepository,
	entryEventRepo *repositories.EntryEventRepository,
	pickupRepo *repositories.GatePassPickupRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
) *GatePassService {
	return &GatePassService{
		GatePassRepo:   gatePassRepo,
		EntryRepo:      entryRepo,
		EntryEventRepo: entryEventRepo,
		PickupRepo:     pickupRepo,
		RoomEntryRepo:  roomEntryRepo,
	}
}

// CreateGatePass creates a gate pass and logs the event
func (s *GatePassService) CreateGatePass(ctx context.Context, req *models.CreateGatePassRequest, userID int) (*models.GatePass, error) {
	// Verify payment if required
	if !req.PaymentVerified {
		return nil, errors.New("payment must be verified before issuing gate pass")
	}

	// Verify customer has enough stock if entry_id is provided
	if req.EntryID != nil {
		entry, err := s.EntryRepo.Get(ctx, *req.EntryID)
		if err != nil {
			return nil, errors.New("entry not found")
		}

		// TODO: Check if customer has enough stock
		// This would involve checking current stock vs requested quantity
		if req.RequestedQuantity > entry.ExpectedQuantity {
			return nil, errors.New("requested quantity exceeds available stock")
		}
	}

	gatePass := &models.GatePass{
		CustomerID:        req.CustomerID,
		TruckNumber:       req.TruckNumber,
		EntryID:           req.EntryID,
		RequestedQuantity: req.RequestedQuantity,
		PaymentVerified:   req.PaymentVerified,
		PaymentAmount:     &req.PaymentAmount,
		IssuedByUserID:    &userID,
		Status:            "pending",
	}

	if req.Remarks != "" {
		gatePass.Remarks = &req.Remarks
	}

	err := s.GatePassRepo.CreateGatePass(ctx, gatePass)
	if err != nil {
		return nil, err
	}

	// Log GATE_PASS_ISSUED event (2nd last event)
	if req.EntryID != nil {
		event := &models.EntryEvent{
			EntryID:         *req.EntryID,
			EventType:       "GATE_PASS_ISSUED",
			Status:          "pending",
			Notes:           "Gate pass issued for " + string(rune(req.RequestedQuantity)) + " items",
			CreatedByUserID: userID,
		}
		s.EntryEventRepo.Create(ctx, event)
	}

	return gatePass, nil
}

// ListAllGatePasses retrieves all gate passes
func (s *GatePassService) ListAllGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	return s.GatePassRepo.ListAllGatePasses(ctx)
}

// ListPendingGatePasses retrieves pending gate passes for unloading tickets
func (s *GatePassService) ListPendingGatePasses(ctx context.Context) ([]map[string]interface{}, error) {
	return s.GatePassRepo.ListPendingGatePasses(ctx)
}

// ApproveGatePass approves a gate pass and updates quantity/gate
func (s *GatePassService) ApproveGatePass(ctx context.Context, id int, req *models.UpdateGatePassRequest, userID int) error {
	gatePass, err := s.GatePassRepo.GetGatePass(ctx, id)
	if err != nil {
		return err
	}

	if gatePass.Status != "pending" {
		return errors.New("gate pass is not pending")
	}

	// Check if gate pass has expired (30 hours from issue time)
	if gatePass.ExpiresAt != nil && time.Now().After(*gatePass.ExpiresAt) {
		// Auto-expire the gate pass
		s.GatePassRepo.UpdateGatePass(ctx, id, 0, "", "expired", "Auto-expired: Not approved within 30 hours", userID)
		return errors.New("gate pass has expired - not approved within 30 hours")
	}

	// Use UpdateGatePassWithSource if request_source is provided
	if req.RequestSource != "" {
		err = s.GatePassRepo.UpdateGatePassWithSource(ctx, id, req.ApprovedQuantity, req.GateNo, req.Status, req.RequestSource, req.Remarks, userID)
	} else {
		err = s.GatePassRepo.UpdateGatePass(ctx, id, req.ApprovedQuantity, req.GateNo, req.Status, req.Remarks, userID)
	}

	if err != nil {
		return err
	}

	return nil
}

// CompleteGatePass marks items as taken out (LAST event)
func (s *GatePassService) CompleteGatePass(ctx context.Context, id int, userID int) error {
	gatePass, err := s.GatePassRepo.GetGatePass(ctx, id)
	if err != nil {
		return err
	}

	if gatePass.Status != "approved" {
		return errors.New("gate pass must be approved before completion")
	}

	err = s.GatePassRepo.CompleteGatePass(ctx, id)
	if err != nil {
		return err
	}

	// Log ITEMS_OUT event (LAST event)
	if gatePass.EntryID != nil {
		approvedQty := gatePass.RequestedQuantity
		if gatePass.ApprovedQuantity != nil {
			approvedQty = *gatePass.ApprovedQuantity
		}

		notes := "Items out: " + string(rune(approvedQty)) + " items physically taken by customer"

		// Check if this is partial or full withdrawal
		entry, _ := s.EntryRepo.Get(ctx, *gatePass.EntryID)
		if entry != nil && approvedQty < entry.ExpectedQuantity {
			notes += " (PARTIAL withdrawal)"
		} else {
			notes += " (FULL withdrawal - ALL items taken)"
		}

		event := &models.EntryEvent{
			EntryID:         *gatePass.EntryID,
			EventType:       "ITEMS_OUT",
			Status:          "completed",
			Notes:           notes,
			CreatedByUserID: userID,
		}
		s.EntryEventRepo.Create(ctx, event)
	}

	return nil
}

// RecordPickup records a partial pickup and updates inventory
func (s *GatePassService) RecordPickup(ctx context.Context, req *models.RecordPickupRequest, userID int) error {
	// Check expiration before allowing pickup
	err := s.CheckAndExpireGatePasses(ctx)
	if err != nil {
		return err
	}

	// Get gate pass details
	gatePass, err := s.GatePassRepo.GetGatePass(ctx, req.GatePassID)
	if err != nil {
		return err
	}

	// Validate gate pass status
	if gatePass.Status != "approved" && gatePass.Status != "partially_completed" {
		return errors.New("gate pass must be approved to record pickup")
	}

	// Check if expired
	if gatePass.ApprovalExpiresAt != nil && time.Now().After(*gatePass.ApprovalExpiresAt) {
		return errors.New("gate pass has expired - pickup window closed")
	}

	// Validate pickup quantity
	remainingQty := gatePass.RequestedQuantity - gatePass.TotalPickedUp
	if req.PickupQuantity > remainingQty {
		return errors.New("pickup quantity exceeds remaining quantity")
	}

	if req.PickupQuantity <= 0 {
		return errors.New("pickup quantity must be greater than zero")
	}

	// Create pickup record
	pickup := &models.GatePassPickup{
		GatePassID:       req.GatePassID,
		PickupQuantity:   req.PickupQuantity,
		PickedUpByUserID: userID,
	}

	if req.RoomNo != "" {
		pickup.RoomNo = &req.RoomNo
	}
	if req.Floor != "" {
		pickup.Floor = &req.Floor
	}
	if req.Remarks != "" {
		pickup.Remarks = &req.Remarks
	}

	err = s.PickupRepo.CreatePickup(ctx, pickup)
	if err != nil {
		return err
	}

	// Update gate pass total_picked_up and status
	err = s.GatePassRepo.UpdatePickupQuantity(ctx, req.GatePassID, req.PickupQuantity)
	if err != nil {
		return err
	}

	// Update room inventory - reduce quantity
	if req.RoomNo != "" && req.Floor != "" {
		err = s.RoomEntryRepo.ReduceQuantity(ctx, gatePass.TruckNumber, req.RoomNo, req.Floor, req.PickupQuantity)
		if err != nil {
			// Log error but don't fail - inventory can be adjusted manually
			// TODO: Add proper logging
		}
	}

	return nil
}

// GetPickupHistory retrieves all pickups for a gate pass
func (s *GatePassService) GetPickupHistory(ctx context.Context, gatePassID int) ([]models.GatePassPickup, error) {
	return s.PickupRepo.GetPickupsByGatePassID(ctx, gatePassID)
}

// CheckAndExpireGatePasses checks for and expires gate passes past their 15-hour window
func (s *GatePassService) CheckAndExpireGatePasses(ctx context.Context) error {
	return s.GatePassRepo.ExpireGatePasses(ctx)
}

// GetExpiredGatePassLogs retrieves recently expired gate passes for admin reporting
func (s *GatePassService) GetExpiredGatePassLogs(ctx context.Context) ([]map[string]interface{}, error) {
	// First check and expire any that need expiring
	err := s.CheckAndExpireGatePasses(ctx)
	if err != nil {
		return nil, err
	}

	return s.GatePassRepo.GetExpiredGatePasses(ctx)
}
