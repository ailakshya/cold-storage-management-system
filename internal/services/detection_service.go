package services

import (
	"context"
	"fmt"
	"time"

	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
)

type DetectionService struct {
	Repo *repositories.DetectionRepository
}

func NewDetectionService(repo *repositories.DetectionRepository) *DetectionService {
	return &DetectionService{Repo: repo}
}

// CreateSession validates and stores a detection session from the Python service.
func (s *DetectionService) CreateSession(ctx context.Context, req *models.CreateDetectionSessionRequest) (*models.DetectionSession, error) {
	if req.GateID == "" {
		return nil, fmt.Errorf("gate_id is required")
	}
	if req.EstimatedTotal < 0 {
		return nil, fmt.Errorf("estimated_total must be non-negative")
	}

	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid started_at: %w", err)
	}

	var endedAt *time.Time
	var durationSec *int
	if req.EndedAt != "" {
		t, err := time.Parse(time.RFC3339, req.EndedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid ended_at: %w", err)
		}
		endedAt = &t
		d := int(t.Sub(startedAt).Seconds())
		durationSec = &d
	} else if req.DurationSeconds > 0 {
		durationSec = &req.DurationSeconds
	}

	var videoPath *string
	if req.VideoPath != "" {
		videoPath = &req.VideoPath
	}

	session := &models.DetectionSession{
		GateID:            req.GateID,
		StartedAt:         startedAt,
		EndedAt:           endedAt,
		DurationSeconds:   durationSec,
		EstimatedTotal:    req.EstimatedTotal,
		UniqueBagCount:    req.UniqueBagCount,
		BagClusterCount:   req.BagClusterCount,
		PeakBagsInFrame:   req.PeakBagsInFrame,
		VehicleConfidence: req.VehicleConfidence,
		AvgBagConfidence:  req.AvgBagConfidence,
		VideoPath:         videoPath,
		VideoSizeBytes:    req.VideoSizeBytes,
		Status:            "completed",
	}

	if err := s.Repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create detection session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a detection session by ID (with linked room entries).
func (s *DetectionService) GetSession(ctx context.Context, id int) (*models.DetectionSession, error) {
	return s.Repo.GetSessionByID(ctx, id)
}

// ListSessions returns paginated detection sessions with optional filters.
func (s *DetectionService) ListSessions(ctx context.Context, gateID, status string, limit, offset int) ([]*models.DetectionSession, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.Repo.ListSessions(ctx, gateID, status, limit, offset)
}

// UpdateSession updates a detection session (link guard entry, add manual count, etc).
func (s *DetectionService) UpdateSession(ctx context.Context, id int, req *models.UpdateDetectionSessionRequest) error {
	_, err := s.Repo.GetSessionByID(ctx, id)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	return s.Repo.UpdateSession(ctx, id, req)
}

// LinkRoomEntry links a detection session to a room entry (thock).
func (s *DetectionService) LinkRoomEntry(ctx context.Context, sessionID int, req *models.LinkRoomEntryRequest, userID int) (*models.DetectionRoomEntry, error) {
	if req.RoomEntryID <= 0 {
		return nil, fmt.Errorf("room_entry_id is required")
	}
	_, err := s.Repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return s.Repo.LinkRoomEntry(ctx, sessionID, req, userID)
}

// UnlinkRoomEntry removes a link between session and room entry.
func (s *DetectionService) UnlinkRoomEntry(ctx context.Context, sessionID, roomEntryID int) error {
	return s.Repo.UnlinkRoomEntry(ctx, sessionID, roomEntryID)
}

// GetSessionsByRoomEntry finds all detection sessions for a thock (for item locator media view).
func (s *DetectionService) GetSessionsByRoomEntry(ctx context.Context, roomEntryID int) ([]*models.DetectionSession, error) {
	return s.Repo.GetSessionsByRoomEntry(ctx, roomEntryID)
}

// GetRecentByGate returns the latest sessions for a given gate.
func (s *DetectionService) GetRecentByGate(ctx context.Context, gateID string, limit int) ([]*models.DetectionSession, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.Repo.GetRecentByGate(ctx, gateID, limit)
}

// GetDailySummary returns aggregated detection stats for a date range.
func (s *DetectionService) GetDailySummary(ctx context.Context, from, to time.Time) ([]map[string]interface{}, error) {
	return s.Repo.GetDailySummary(ctx, from, to)
}
