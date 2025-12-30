package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"cold-backend/internal/cache"
	"cold-backend/internal/models"
	"cold-backend/internal/repositories"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Cache constants for entry room
const (
	entryRoomCacheKey = "entry_room:summary"
	entryRoomCacheTTL = 30 * time.Second // Short TTL - real-time data
)

// EntryRoomHandler handles optimized entry room endpoints
type EntryRoomHandler struct {
	DB              *pgxpool.Pool
	EntryRepo       *repositories.EntryRepository
	RoomEntryRepo   *repositories.RoomEntryRepository
	CustomerRepo    *repositories.CustomerRepository
	GuardEntryRepo  *repositories.GuardEntryRepository
}

// CustomerPhone represents minimal customer info for autocomplete
type CustomerPhone struct {
	ID      int    `json:"id"`
	Phone   string `json:"phone"`
	Name    string `json:"name"`
	Village string `json:"village"`
	SO      string `json:"so"`
}

// EntryRoomSummary is the combined response for the entry room page
type EntryRoomSummary struct {
	Entries        []*models.Entry      `json:"entries"`
	RoomEntries    []*models.RoomEntry  `json:"room_entries"`
	GuardEntries   []*models.GuardEntry `json:"guard_entries"`
	CustomerPhones []CustomerPhone      `json:"customer_phones"`
	Stats          EntryRoomStats       `json:"stats"`
	GeneratedAt    time.Time            `json:"generated_at"`
}

// EntryRoomStats contains summary statistics
type EntryRoomStats struct {
	TotalEntries     int `json:"total_entries"`
	TotalRoomEntries int `json:"total_room_entries"`
	PendingGuard     int `json:"pending_guard"`
}

// DeltaResponse contains only new entries since the last refresh
type DeltaResponse struct {
	Entries      []*models.Entry     `json:"entries"`
	RoomEntries  []*models.RoomEntry `json:"room_entries"`
	GuardEntries []*models.GuardEntry `json:"guard_entries"`
	Timestamp    time.Time           `json:"timestamp"`
}

func NewEntryRoomHandler(
	db *pgxpool.Pool,
	entryRepo *repositories.EntryRepository,
	roomEntryRepo *repositories.RoomEntryRepository,
	customerRepo *repositories.CustomerRepository,
	guardEntryRepo *repositories.GuardEntryRepository,
) *EntryRoomHandler {
	return &EntryRoomHandler{
		DB:             db,
		EntryRepo:      entryRepo,
		RoomEntryRepo:  roomEntryRepo,
		CustomerRepo:   customerRepo,
		GuardEntryRepo: guardEntryRepo,
	}
}

// GetSummary returns all data needed for entry room page in a single request
// Replaces 4 sequential API calls with 1 parallel fetch
// Uses Redis cache with 30s TTL for fast response
func (h *EntryRoomHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Try cache first
	if data, ok := cache.GetCached(ctx, entryRoomCacheKey); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write(data)
		return
	}

	// Cache miss - fetch from database
	data, err := h.fetchSummaryData(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache the result
	cache.SetCached(ctx, entryRoomCacheKey, data, entryRoomCacheTTL)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

// fetchSummaryData fetches all entry room data from database
// This is extracted as a helper so it can be used for pre-warming
func (h *EntryRoomHandler) fetchSummaryData(ctx context.Context) ([]byte, error) {
	var (
		entries      []*models.Entry
		roomEntries  []*models.RoomEntry
		customers    []*models.Customer
		guardEntries []*models.GuardEntry
		wg           sync.WaitGroup
		entriesErr   error
		roomErr      error
		custErr      error
		guardErr     error
	)

	wg.Add(4)

	// Fetch entries (with optimized N+1 fix)
	go func() {
		defer wg.Done()
		entries, entriesErr = h.EntryRepo.List(ctx)
	}()

	// Fetch room entries
	go func() {
		defer wg.Done()
		roomEntries, roomErr = h.RoomEntryRepo.List(ctx)
	}()

	// Fetch customers (for autocomplete)
	go func() {
		defer wg.Done()
		customers, custErr = h.CustomerRepo.List(ctx)
	}()

	// Fetch pending guard entries
	go func() {
		defer wg.Done()
		if h.GuardEntryRepo != nil {
			guardEntries, guardErr = h.GuardEntryRepo.ListPending(ctx)
		}
	}()

	wg.Wait()

	// Check for errors
	if entriesErr != nil {
		return nil, entriesErr
	}
	if roomErr != nil {
		return nil, roomErr
	}
	// Customer and guard errors are non-fatal
	if custErr != nil {
		customers = []*models.Customer{}
	}
	if guardErr != nil {
		guardEntries = []*models.GuardEntry{}
	}

	// Convert customers to minimal phone list
	customerPhones := make([]CustomerPhone, 0, len(customers))
	for _, c := range customers {
		customerPhones = append(customerPhones, CustomerPhone{
			ID:      c.ID,
			Phone:   c.Phone,
			Name:    c.Name,
			Village: c.Village,
			SO:      c.SO,
		})
	}

	summary := EntryRoomSummary{
		Entries:        entries,
		RoomEntries:    roomEntries,
		GuardEntries:   guardEntries,
		CustomerPhones: customerPhones,
		Stats: EntryRoomStats{
			TotalEntries:     len(entries),
			TotalRoomEntries: len(roomEntries),
			PendingGuard:     len(guardEntries),
		},
		GeneratedAt: time.Now(),
	}

	return json.Marshal(summary)
}

// GetPreWarmFetcher returns a function that can be used to pre-warm the cache
// This is called by write handlers after invalidation
func (h *EntryRoomHandler) GetPreWarmFetcher() func(ctx context.Context) ([]byte, error) {
	return h.fetchSummaryData
}

// GetDelta returns only entries created since the given timestamp
// Used for 5-second refresh to avoid reloading entire dataset
func (h *EntryRoomHandler) GetDelta(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	since := r.URL.Query().Get("t")
	if since == "" {
		http.Error(w, "Missing 't' parameter (timestamp)", http.StatusBadRequest)
		return
	}

	var (
		entries      []*models.Entry
		roomEntries  []*models.RoomEntry
		guardEntries []*models.GuardEntry
		wg           sync.WaitGroup
		entriesErr   error
		roomErr      error
		guardErr     error
	)

	wg.Add(3)

	// Fetch new entries since timestamp
	go func() {
		defer wg.Done()
		entries, entriesErr = h.EntryRepo.ListSince(ctx, since)
	}()

	// Fetch new room entries since timestamp
	go func() {
		defer wg.Done()
		roomEntries, roomErr = h.RoomEntryRepo.ListSince(ctx, since)
	}()

	// Fetch pending guard entries (always get all pending)
	go func() {
		defer wg.Done()
		if h.GuardEntryRepo != nil {
			guardEntries, guardErr = h.GuardEntryRepo.ListPending(ctx)
		}
	}()

	wg.Wait()

	// Check for errors
	if entriesErr != nil {
		http.Error(w, "Failed to load entries: "+entriesErr.Error(), http.StatusInternalServerError)
		return
	}
	if roomErr != nil {
		http.Error(w, "Failed to load room entries: "+roomErr.Error(), http.StatusInternalServerError)
		return
	}
	if guardErr != nil {
		guardEntries = []*models.GuardEntry{}
	}

	delta := DeltaResponse{
		Entries:      entries,
		RoomEntries:  roomEntries,
		GuardEntries: guardEntries,
		Timestamp:    time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(delta)
}
