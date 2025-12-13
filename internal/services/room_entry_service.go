package services

import (
	"context"
	"errors"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type RoomEntryService struct {
	RoomEntryRepo  *repositories.RoomEntryRepository
	EntryRepo      *repositories.EntryRepository
	EntryEventRepo *repositories.EntryEventRepository
}

func NewRoomEntryService(roomEntryRepo *repositories.RoomEntryRepository, entryRepo *repositories.EntryRepository, entryEventRepo *repositories.EntryEventRepository) *RoomEntryService {
	return &RoomEntryService{
		RoomEntryRepo:  roomEntryRepo,
		EntryRepo:      entryRepo,
		EntryEventRepo: entryEventRepo,
	}
}

func (s *RoomEntryService) CreateRoomEntry(ctx context.Context, req *models.CreateRoomEntryRequest, userID int) (*models.RoomEntry, error) {
	// Validate required fields
	if req.TruckNumber == "" {
		return nil, errors.New("truck number is required")
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

	// Check if room entry already exists for this entry
	existing, err := s.RoomEntryRepo.GetByEntryID(ctx, req.EntryID)
	if err == nil && existing != nil {
		return nil, errors.New("room entry already exists for this entry")
	}

	// Create room entry
	roomEntry := &models.RoomEntry{
		EntryID:         req.EntryID,
		TruckNumber:     req.TruckNumber,
		RoomNo:          req.RoomNo,
		Floor:           req.Floor,
		GateNo:          req.GateNo,
		Remark:          req.Remark,
		Quantity:        req.Quantity,
		CreatedByUserID: userID,
	}

	if err := s.RoomEntryRepo.Create(ctx, roomEntry); err != nil {
		return nil, err
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

	return roomEntry, nil
}

func (s *RoomEntryService) GetRoomEntry(ctx context.Context, id int) (*models.RoomEntry, error) {
	return s.RoomEntryRepo.Get(ctx, id)
}

func (s *RoomEntryService) ListRoomEntries(ctx context.Context) ([]*models.RoomEntry, error) {
	return s.RoomEntryRepo.List(ctx)
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

	// Update in database
	if err := s.RoomEntryRepo.Update(ctx, id, roomEntry); err != nil {
		return nil, err
	}

	return roomEntry, nil
}
