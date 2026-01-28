package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"cold-backend/internal/cache"
	"cold-backend/internal/middleware"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"
	"cold-backend/internal/services"

	"github.com/gorilla/mux"
)

type RoomEntryHandler struct {
	Service     *services.RoomEntryService
	EditLogRepo *repositories.RoomEntryEditLogRepository
}

func NewRoomEntryHandler(s *services.RoomEntryService, editLogRepo *repositories.RoomEntryEditLogRepository) *RoomEntryHandler {
	return &RoomEntryHandler{
		Service:     s,
		EditLogRepo: editLogRepo,
	}
}

func (h *RoomEntryHandler) CreateRoomEntry(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRoomEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	roomEntry, err := h.Service.CreateRoomEntry(context.Background(), &req, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalidate room visualization cache
	cache.InvalidateRoomCache(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomEntry)
}

func (h *RoomEntryHandler) GetRoomEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, _ := strconv.Atoi(idStr)

	roomEntry, err := h.Service.GetRoomEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomEntry)
}

func (h *RoomEntryHandler) ListRoomEntries(w http.ResponseWriter, r *http.Request) {
	roomEntries, err := h.Service.ListRoomEntries(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomEntries)
}

func (h *RoomEntryHandler) GetUnassignedEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.Service.GetUnassignedEntries(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *RoomEntryHandler) UpdateRoomEntry(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid room entry ID", http.StatusBadRequest)
		return
	}

	// Get user ID from JWT context
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	var req models.UpdateRoomEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get old room entry before updating
	oldEntry, err := h.Service.GetRoomEntry(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Perform update
	roomEntry, err := h.Service.UpdateRoomEntry(context.Background(), id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create edit log if there were changes
	editLog := &models.RoomEntryEditLog{
		RoomEntryID:    id,
		EditedByUserID: userID,
	}

	// Track changes (create copies of values for pointers)
	if oldEntry.RoomNo != roomEntry.RoomNo {
		oldRoomNo := oldEntry.RoomNo
		newRoomNo := roomEntry.RoomNo
		editLog.OldRoomNo = &oldRoomNo
		editLog.NewRoomNo = &newRoomNo
	}
	if oldEntry.Floor != roomEntry.Floor {
		oldFloor := oldEntry.Floor
		newFloor := roomEntry.Floor
		editLog.OldFloor = &oldFloor
		editLog.NewFloor = &newFloor
	}
	if oldEntry.GateNo != roomEntry.GateNo {
		oldGateNo := oldEntry.GateNo
		newGateNo := roomEntry.GateNo
		editLog.OldGateNo = &oldGateNo
		editLog.NewGateNo = &newGateNo
	}
	if oldEntry.Quantity != roomEntry.Quantity {
		oldQuantity := oldEntry.Quantity
		newQuantity := roomEntry.Quantity
		editLog.OldQuantity = &oldQuantity
		editLog.NewQuantity = &newQuantity
	}
	if oldEntry.Remark != roomEntry.Remark {
		oldRemark := oldEntry.Remark
		newRemark := roomEntry.Remark
		editLog.OldRemark = &oldRemark
		editLog.NewRemark = &newRemark
	}

	// Only log if there were actual changes
	if editLog.OldRoomNo != nil || editLog.OldFloor != nil || editLog.OldGateNo != nil ||
	   editLog.OldQuantity != nil || editLog.OldRemark != nil {
		if err := h.EditLogRepo.CreateEditLog(context.Background(), editLog); err != nil {
			// Log error but don't fail the update
			// TODO: Add proper logging
		}
	}

	// Invalidate room visualization cache
	cache.InvalidateRoomCache(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roomEntry)
}

func (h *RoomEntryHandler) ListRoomEntryMedia(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid room entry ID", http.StatusBadRequest)
		return
	}

	media, err := h.Service.GetMediaByRoomEntryID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(media)
}

func (h *RoomEntryHandler) ListRoomEntryMediaByThock(w http.ResponseWriter, r *http.Request) {
	thockNumber := r.URL.Query().Get("thock_number")
	if thockNumber == "" {
		http.Error(w, "thock_number query parameter is required", http.StatusBadRequest)
		return
	}

	media, err := h.Service.GetMediaByThockNumber(r.Context(), thockNumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if media == nil {
		media = []models.RoomEntryMedia{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"edit_media": media,
	})
}

func (h *RoomEntryHandler) SaveRoomEntryMedia(w http.ResponseWriter, r *http.Request) {
	var media models.RoomEntryMedia
	if err := json.NewDecoder(r.Body).Decode(&media); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	media.UploadedByUserID = &userID

	if media.MediaType != "entry" && media.MediaType != "edit" {
		http.Error(w, "Invalid media_type. Must be 'entry' or 'edit'", http.StatusBadRequest)
		return
	}

	if strings.Contains(media.FilePath, "..") {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	if err := h.Service.SaveMediaMetadata(r.Context(), &media); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(media)
}
