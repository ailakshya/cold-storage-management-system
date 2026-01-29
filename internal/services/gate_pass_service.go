package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/timeutil"
)

type GatePassService struct {
	GatePassRepo   *repositories.GatePassRepository
	EntryRepo      *repositories.EntryRepository
	EntryEventRepo *repositories.EntryEventRepository
	PickupRepo     *repositories.GatePassPickupRepository
	RoomEntryRepo  *repositories.RoomEntryRepository
	MediaRepo      *repositories.GatePassMediaRepository
}

func NewGatePassService(
	gatePassRepo *repositories.GatePassRepository,
	entryRepo *repositories.EntryRepository,
	entryEventRepo *repositories.EntryEventRepository,
	pickupRepo *repositories.GatePassPickupRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
	mediaRepo *repositories.GatePassMediaRepository,
) *GatePassService {
	return &GatePassService{
		GatePassRepo:   gatePassRepo,
		EntryRepo:      entryRepo,
		EntryEventRepo: entryEventRepo,
		PickupRepo:     pickupRepo,
		RoomEntryRepo:  roomEntryRepo,
		MediaRepo:      mediaRepo,
	}
}

// CreateGatePass creates a gate pass and logs the event
func (s *GatePassService) CreateGatePass(ctx context.Context, req *models.CreateGatePassRequest, userID int) (*models.GatePass, error) {
	// Verify payment if required
	if !req.PaymentVerified {
		return nil, errors.New("payment must be verified before issuing gate pass")
	}

	// CRITICAL FIX: Verify customer has enough stock if entry_id is provided
	// Check both total entry quantity AND previously approved gate passes
	if req.EntryID != nil {
		entry, err := s.EntryRepo.Get(ctx, *req.EntryID)
		if err != nil {
			return nil, errors.New("entry not found")
		}

		// Calculate total already approved/picked up from previous gate passes
		totalApproved, err := s.GatePassRepo.GetTotalApprovedQuantityForEntry(ctx, *req.EntryID)
		if err != nil {
			return nil, errors.New("failed to calculate available stock")
		}

		// Calculate available quantity (entry quantity - already approved)
		availableQuantity := entry.ExpectedQuantity - totalApproved

		// Validate requested quantity doesn't exceed available stock
		if req.RequestedQuantity > availableQuantity {
			return nil, errors.New("requested quantity exceeds available stock - customer has already withdrawn " +
				strconv.Itoa(totalApproved) + " out of " + strconv.Itoa(entry.ExpectedQuantity) +
				" items. Only " + strconv.Itoa(availableQuantity) + " items available.")
		}
	}

	gatePass := &models.GatePass{
		CustomerID:        req.CustomerID,
		ThockNumber:       req.ThockNumber,
		EntryID:           req.EntryID,
		FamilyMemberID:    req.FamilyMemberID,
		FamilyMemberName:  req.FamilyMemberName,
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
			Notes:           "Gate pass issued for " + strconv.Itoa(req.RequestedQuantity) + " items",
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

	// Allow rejecting approved gate passes if no pickups yet
	if req.Status == "rejected" {
		if gatePass.Status != "pending" && gatePass.Status != "approved" {
			return errors.New("gate pass cannot be rejected - status is " + gatePass.Status)
		}
		if gatePass.TotalPickedUp > 0 {
			return errors.New("cannot reject gate pass - items already picked up")
		}
	} else if gatePass.Status != "pending" {
		return errors.New("gate pass is not pending")
	}

	// Check if gate pass has expired (30 hours from issue time)
	if gatePass.ExpiresAt != nil && timeutil.Now().After(*gatePass.ExpiresAt) {
		// Auto-expire the gate pass
		s.GatePassRepo.UpdateGatePass(ctx, id, 0, "", "expired", "Auto-expired: Not approved within 30 hours", userID)
		return errors.New("gate pass has expired - not approved within 30 hours")
	}

	// Validate approved quantity against available inventory
	if req.Status == "approved" && gatePass.EntryID != nil {
		// Get current inventory from room entries
		currentInventory, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, gatePass.ThockNumber)
		if err != nil {
			currentInventory = 0
		}

		// Calculate total already allocated to other gate passes (excluding this one)
		// This includes pending, approved, and partially_completed gate passes
		totalAllocated, err := s.GatePassRepo.GetTotalApprovedQuantityForEntry(ctx, *gatePass.EntryID)
		if err != nil {
			totalAllocated = 0
		}
		// Subtract this gate pass's requested quantity since it's already included in the total
		totalAllocated -= gatePass.RequestedQuantity

		// Calculate available inventory
		availableInventory := currentInventory - totalAllocated
		if availableInventory < 0 {
			availableInventory = 0
		}

		// Validate approved quantity doesn't exceed available stock
		if req.ApprovedQuantity > availableInventory {
			return errors.New("insufficient inventory: approved quantity (" +
				strconv.Itoa(req.ApprovedQuantity) + ") exceeds available stock (" +
				strconv.Itoa(availableInventory) + "). Current inventory: " +
				strconv.Itoa(currentInventory) + ", already allocated: " +
				strconv.Itoa(totalAllocated) + ")")
		}
	}

	// Determine expiration time for approval
	var expiresAt *time.Time
	if req.Status == "approved" {
		if req.ExpiresAt != nil && *req.ExpiresAt != "" {
			// Custom expiration time set by employee
			parsedTime, parseErr := time.Parse(time.RFC3339, *req.ExpiresAt)
			if parseErr != nil {
				// Try parsing without timezone
				parsedTime, parseErr = time.Parse("2006-01-02T15:04", *req.ExpiresAt)
				if parseErr != nil {
					return errors.New("invalid expiration time format")
				}
			}
			expiresAt = &parsedTime
		} else if gatePass.RequestSource == "customer_portal" {
			// Customer-issued gate pass: 40 hours from now
			expTime := timeutil.Now().Add(40 * time.Hour)
			expiresAt = &expTime
		} else {
			// Employee-issued: 30 hours from now (should already be set, but update if needed)
			expTime := timeutil.Now().Add(30 * time.Hour)
			expiresAt = &expTime
		}
	}

	// Use UpdateGatePassWithExpiration if expiration is set
	if expiresAt != nil {
		err = s.GatePassRepo.UpdateGatePassWithExpiration(ctx, id, req.ApprovedQuantity, req.GateNo, req.Status, req.Remarks, userID, expiresAt)
	} else if req.RequestSource != "" {
		err = s.GatePassRepo.UpdateGatePassWithSource(ctx, id, req.ApprovedQuantity, req.GateNo, req.Status, req.RequestSource, req.Remarks, userID)
	} else {
		err = s.GatePassRepo.UpdateGatePass(ctx, id, req.ApprovedQuantity, req.GateNo, req.Status, req.Remarks, userID)
	}

	if err != nil {
		return err
	}

	// Log GATE_PASS_REJECTED event if status is rejected
	if req.Status == "rejected" && gatePass.EntryID != nil {
		event := &models.EntryEvent{
			EntryID:         *gatePass.EntryID,
			EventType:       "GATE_PASS_REJECTED",
			Status:          "rejected",
			Notes:           "Gate pass rejected by employee. " + req.Remarks,
			CreatedByUserID: userID,
		}
		s.EntryEventRepo.Create(ctx, event)
	}

	return nil
}

// CompleteGatePass marks items as taken out (LAST event)
func (s *GatePassService) CompleteGatePass(ctx context.Context, id int, userID int) error {
	gatePass, err := s.GatePassRepo.GetGatePass(ctx, id)
	if err != nil {
		return err
	}

	// Allow completion from approved or partially_completed status
	if gatePass.Status != "approved" && gatePass.Status != "partially_completed" {
		return errors.New("gate pass must be approved or partially completed before completion")
	}

	// CRITICAL FIX: Validate that items were actually picked up via RecordPickup
	// This prevents completing gate passes with 0 pickup, which would cause inventory mismatch
	if gatePass.TotalPickedUp == 0 {
		return errors.New("cannot complete: no items picked up yet. Use Record Pickup to log items before completing")
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

		notes := "Items out: " + strconv.Itoa(approvedQty) + " items physically taken by customer"

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
// Returns the created pickup ID and any error
func (s *GatePassService) RecordPickup(ctx context.Context, req *models.RecordPickupRequest, userID int) (int, error) {
	// Check expiration before allowing pickup
	err := s.CheckAndExpireGatePasses(ctx)
	if err != nil {
		return 0, err
	}

	// Get gate pass details
	gatePass, err := s.GatePassRepo.GetGatePass(ctx, req.GatePassID)
	if err != nil {
		return 0, err
	}

	// Validate gate pass status
	if gatePass.Status != "approved" && gatePass.Status != "partially_completed" {
		return 0, errors.New("gate pass must be approved to record pickup")
	}

	// Check if expired
	if gatePass.ApprovalExpiresAt != nil && timeutil.Now().After(*gatePass.ApprovalExpiresAt) {
		return 0, errors.New("gate pass has expired - pickup window closed")
	}

	// Validate pickup quantity against APPROVED quantity (not requested)
	maxAllowed := gatePass.RequestedQuantity
	if gatePass.ApprovedQuantity != nil {
		maxAllowed = *gatePass.ApprovedQuantity
	}
	remainingQty := maxAllowed - gatePass.TotalPickedUp
	if req.PickupQuantity > remainingQty {
		return 0, fmt.Errorf("pickup quantity (%d) exceeds remaining approved quantity (%d)", req.PickupQuantity, remainingQty)
	}

	if req.PickupQuantity <= 0 {
		return 0, errors.New("pickup quantity must be greater than zero")
	}

	// CRITICAL FIX: Auto-fill storage location from room_entries if not provided
	// This ensures inventory is ALWAYS reduced when pickup is recorded
	roomNo := req.RoomNo
	floor := req.Floor

	if roomNo == "" || floor == "" {
		// Get actual storage location from room_entries
		roomEntries, err := s.RoomEntryRepo.ListByThockNumber(ctx, gatePass.ThockNumber)
		if err != nil {
			return 0, errors.New("failed to get storage location: " + err.Error())
		}
		if len(roomEntries) == 0 {
			return 0, errors.New("no storage location found for truck " + gatePass.ThockNumber + " - items must be assigned to storage first")
		}

		// Use the first room entry with available quantity
		for _, re := range roomEntries {
			if re.Quantity >= req.PickupQuantity {
				roomNo = re.RoomNo
				floor = re.Floor
				break
			}
		}

		// If no single room has enough, use the first one (will reduce what's available)
		if roomNo == "" || floor == "" {
			roomNo = roomEntries[0].RoomNo
			floor = roomEntries[0].Floor
		}
	}

	// Create pickup record with the resolved storage location
	pickup := &models.GatePassPickup{
		GatePassID:       req.GatePassID,
		PickupQuantity:   req.PickupQuantity,
		PickedUpByUserID: userID,
	}

	pickup.RoomNo = &roomNo
	pickup.Floor = &floor

	if req.Remarks != "" {
		pickup.Remarks = &req.Remarks
	}

	// CRITICAL FIX: Execute all database operations in sequence with proper error handling
	// TODO: Implement proper database transactions to ensure atomicity

	// Step 1: Create pickup record
	err = s.PickupRepo.CreatePickup(ctx, pickup)
	if err != nil {
		return 0, errors.New("failed to create pickup record: " + err.Error())
	}

	// Step 1b: Save gatar breakdown if provided
	if len(req.GatarBreakdown) > 0 {
		err = s.PickupRepo.CreateGatarBreakdown(ctx, pickup.ID, req.GatarBreakdown)
		if err != nil {
			// Non-critical - log but don't fail the pickup
			// The main pickup is already recorded
		}
	}

	// Step 2: Update gate pass total_picked_up and status
	err = s.GatePassRepo.UpdatePickupQuantity(ctx, req.GatePassID, req.PickupQuantity)
	if err != nil {
		return 0, errors.New("CRITICAL ERROR: pickup created but gate pass update failed - " +
			"manual intervention required for gate pass ID " + strconv.Itoa(req.GatePassID) + ": " + err.Error())
	}

	// NOTE: We intentionally do NOT reduce room_entries.quantity here
	// room_entries.quantity represents the ORIGINAL entered quantity (used for rent calculation)
	// Current inventory is calculated as: room_entries.quantity - total_picked_up
	// This prevents double-counting in account reports where outgoing is shown separately

	// Return the created pickup ID for media upload
	return pickup.ID, nil
}

// GetPickupHistory retrieves all pickups for a gate pass
func (s *GatePassService) GetPickupHistory(ctx context.Context, gatePassID int) ([]models.GatePassPickup, error) {
	return s.PickupRepo.GetPickupsByGatePassID(ctx, gatePassID)
}

// GetAllPickups retrieves all pickups with customer info for activity log
func (s *GatePassService) GetAllPickups(ctx context.Context) ([]map[string]interface{}, error) {
	return s.PickupRepo.GetAllPickups(ctx)
}

// GetPickupHistoryByThockNumber retrieves all pickups for a thock number (across all gate passes)
func (s *GatePassService) GetPickupHistoryByThockNumber(ctx context.Context, thockNumber string) ([]models.GatePassPickup, error) {
	return s.PickupRepo.GetPickupsByThockNumber(ctx, thockNumber)
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

// GetMediaByThockNumber retrieves all media (entry + pickup) for a thock number
// Returns a map with "entry_media" and "pickup_media" arrays
func (s *GatePassService) GetMediaByThockNumber(ctx context.Context, thockNumber string) (map[string][]models.GatePassMedia, error) {
	allMedia, err := s.MediaRepo.ListByThockNumber(ctx, thockNumber)
	if err != nil {
		return nil, err
	}

	// Separate entry and pickup media
	entryMedia := []models.GatePassMedia{}
	pickupMedia := []models.GatePassMedia{}

	for _, m := range allMedia {
		if m.MediaType == "entry" {
			entryMedia = append(entryMedia, m)
		} else if m.MediaType == "pickup" {
			pickupMedia = append(pickupMedia, m)
		}
	}

	return map[string][]models.GatePassMedia{
		"entry_media":  entryMedia,
		"pickup_media": pickupMedia,
	}, nil
}

// SaveMediaMetadata saves media metadata to the database
func (s *GatePassService) SaveMediaMetadata(ctx context.Context, media *models.GatePassMedia) error {
	return s.MediaRepo.Create(ctx, media)
}

// GetMediaByGatePassID retrieves all media for a specific gate pass
func (s *GatePassService) GetMediaByGatePassID(ctx context.Context, gatePassID int) ([]models.GatePassMedia, error) {
	return s.MediaRepo.ListByGatePassID(ctx, gatePassID)
}
