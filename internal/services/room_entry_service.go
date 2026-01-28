package services

import (
	"context"
	"errors"
	"strconv"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type RoomEntryService struct {
	RoomEntryRepo      *repositories.RoomEntryRepository
	RoomEntryGatarRepo *repositories.RoomEntryGatarRepository
	EntryRepo          *repositories.EntryRepository
	EntryEventRepo     *repositories.EntryEventRepository
	PrinterService     *PrinterService
	MediaRepo          *repositories.RoomEntryMediaRepository
}

func NewRoomEntryService(roomEntryRepo *repositories.RoomEntryRepository, roomEntryGatarRepo *repositories.RoomEntryGatarRepository, entryRepo *repositories.EntryRepository, entryEventRepo *repositories.EntryEventRepository, printerService *PrinterService, mediaRepo *repositories.RoomEntryMediaRepository) *RoomEntryService {
	return &RoomEntryService{
		RoomEntryRepo:      roomEntryRepo,
		RoomEntryGatarRepo: roomEntryGatarRepo,
		EntryRepo:          entryRepo,
		EntryEventRepo:     entryEventRepo,
		PrinterService:     printerService,
		MediaRepo:          mediaRepo,
	}
}

func (s *RoomEntryService) GetMediaByRoomEntryID(ctx context.Context, roomEntryID int) (map[string][]models.RoomEntryMedia, error) {
	allMedia, err := s.MediaRepo.ListByRoomEntryID(ctx, roomEntryID)
	if err != nil {
		return nil, err
	}

	entryMedia := []models.RoomEntryMedia{}
	editMedia := []models.RoomEntryMedia{}
	for _, m := range allMedia {
		if m.MediaType == "edit" {
			editMedia = append(editMedia, m)
		} else {
			entryMedia = append(entryMedia, m)
		}
	}

	return map[string][]models.RoomEntryMedia{
		"entry_media": entryMedia,
		"edit_media":  editMedia,
	}, nil
}

func (s *RoomEntryService) GetMediaByThockNumber(ctx context.Context, thockNumber string) ([]models.RoomEntryMedia, error) {
	return s.MediaRepo.ListByThockNumber(ctx, thockNumber)
}

func (s *RoomEntryService) SaveMediaMetadata(ctx context.Context, media *models.RoomEntryMedia) error {
	return s.MediaRepo.Create(ctx, media)
}

func (s *RoomEntryService) CreateRoomEntry(ctx context.Context, req *models.CreateRoomEntryRequest, userID int) (*models.RoomEntry, error) {
	// Validate required fields
	if req.ThockNumber == "" {
		return nil, errors.New("thock number is required")
	}
	if req.RoomNo == "" {
		return nil, errors.New("room number is required")
	}
	if req.Floor == "" {
		return nil, errors.New("floor is required")
	}
	if req.GateNo == "" {
		return nil, errors.New("gatar number is required")
	}
	if req.Quantity < 1 {
		return nil, errors.New("quantity must be at least 1")
	}

	// Check if entry exists
	entry, err := s.EntryRepo.Get(ctx, req.EntryID)
	if err != nil {
		return nil, errors.New("entry not found")
	}

	// CRITICAL FIX: Validate that room entry quantity doesn't exceed entry quantity
	// Get total quantity already assigned to rooms for this truck number
	totalAssigned, err := s.RoomEntryRepo.GetTotalQuantityByThockNumber(ctx, req.ThockNumber)
	if err != nil {
		// If error is just "no records", treat as 0
		totalAssigned = 0
	}

	// Validate that new quantity + existing assignments don't exceed entry quantity
	if totalAssigned+req.Quantity > entry.ExpectedQuantity {
		return nil, errors.New("total room assignments would exceed entry quantity - " +
			"entry has " + strconv.Itoa(entry.ExpectedQuantity) + " items, " +
			strconv.Itoa(totalAssigned) + " already assigned, " +
			"trying to assign " + strconv.Itoa(req.Quantity) + " more")
	}

	// Check if room entry already exists for this entry
	existing, err := s.RoomEntryRepo.GetByEntryID(ctx, req.EntryID)
	if err == nil && existing != nil {
		return nil, errors.New("room entry already exists for this entry")
	}

	// Create room entry
	roomEntry := &models.RoomEntry{
		EntryID:           req.EntryID,
		ThockNumber:       req.ThockNumber,
		RoomNo:            req.RoomNo,
		Floor:             req.Floor,
		GateNo:            req.GateNo,
		Remark:            req.Remark,
		Quantity:          req.Quantity,
		QuantityBreakdown: req.QuantityBreakdown,
		CreatedByUserID:   userID,
	}

	if err := s.RoomEntryRepo.Create(ctx, roomEntry); err != nil {
		return nil, err
	}

	// Save per-gatar quantities if provided
	if len(req.Gatars) > 0 {
		if err := s.RoomEntryGatarRepo.CreateBatch(ctx, roomEntry.ID, req.Gatars); err != nil {
			// Log error but don't fail the whole operation
			// The main room entry is already created
		}
	}

	// Create event to track room entry completion
	event := &models.EntryEvent{
		EntryID:         entry.ID,
		EventType:       models.EventTypeInStorage,
		Status:          models.StatusInStorage,
		Notes:           "Items stored in Room " + req.RoomNo + ", Floor " + req.Floor + ", Gatar " + req.GateNo,
		CreatedByUserID: userID,
	}

	// Create event (don't fail if this fails)
	s.EntryEventRepo.Create(ctx, event)

	// Print label with thock number and customer name (if label count > 0)
	if s.PrinterService != nil && req.LabelCount > 0 {
		labelCount := req.LabelCount
		thockNumber := req.ThockNumber
		customerName := entry.Name
		go func() {
			_ = s.PrinterService.Print2Up(thockNumber, customerName, labelCount)
		}()
	}

	return roomEntry, nil
}

func (s *RoomEntryService) GetRoomEntry(ctx context.Context, id int) (*models.RoomEntry, error) {
	roomEntry, err := s.RoomEntryRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Fetch gatars for this room entry
	gatars, err := s.RoomEntryGatarRepo.GetByRoomEntryID(ctx, id)
	if err == nil && len(gatars) > 0 {
		roomEntry.Gatars = gatars
	}

	return roomEntry, nil
}

func (s *RoomEntryService) ListRoomEntries(ctx context.Context) ([]*models.RoomEntry, error) {
	roomEntries, err := s.RoomEntryRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch gatars for each room entry
	for _, re := range roomEntries {
		gatars, err := s.RoomEntryGatarRepo.GetByRoomEntryID(ctx, re.ID)
		if err == nil && len(gatars) > 0 {
			re.Gatars = gatars
		}
	}

	return roomEntries, nil
}

func (s *RoomEntryService) GetUnassignedEntries(ctx context.Context) ([]*models.Entry, error) {
	return s.EntryRepo.ListUnassigned(ctx)
}

func (s *RoomEntryService) UpdateRoomEntry(ctx context.Context, id int, req *models.UpdateRoomEntryRequest) (*models.RoomEntry, error) {
	// Get existing room entry
	roomEntry, err := s.RoomEntryRepo.Get(ctx, id)
	if err != nil {
		return nil, errors.New("room entry not found")
	}

	// Validate required fields
	if req.RoomNo == "" {
		return nil, errors.New("room number is required")
	}
	if req.Floor == "" {
		return nil, errors.New("floor is required")
	}
	if req.GateNo == "" {
		return nil, errors.New("gatar number is required")
	}
	if req.Quantity < 1 {
		return nil, errors.New("quantity must be at least 1")
	}

	// Update fields
	roomEntry.RoomNo = req.RoomNo
	roomEntry.Floor = req.Floor
	roomEntry.GateNo = req.GateNo
	roomEntry.Remark = req.Remark
	roomEntry.Quantity = req.Quantity
	roomEntry.QuantityBreakdown = req.QuantityBreakdown

	// Update in database
	if err := s.RoomEntryRepo.Update(ctx, id, roomEntry); err != nil {
		return nil, err
	}

	// Update per-gatar quantities if provided
	if len(req.Gatars) > 0 {
		if err := s.RoomEntryGatarRepo.UpdateBatch(ctx, id, req.Gatars); err != nil {
			// Log error but don't fail the whole operation
		}
	}

	return roomEntry, nil
}
