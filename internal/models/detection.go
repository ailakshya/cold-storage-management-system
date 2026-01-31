package models

import (
	"encoding/json"
	"time"
)

// DetectionSession represents a single vehicle unloading event at a gate,
// as detected by the YOLOv8 inference service.
// Linked to guard_entries (vehicle registration) and room_entries (thocks) via junction table.
type DetectionSession struct {
	ID                int        `json:"id" db:"id"`
	GateID            string     `json:"gate_id" db:"gate_id"`
	GuardEntryID      *int       `json:"guard_entry_id,omitempty" db:"guard_entry_id"`
	StartedAt         time.Time  `json:"started_at" db:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty" db:"ended_at"`
	DurationSeconds   *int       `json:"duration_seconds,omitempty" db:"duration_seconds"`
	EstimatedTotal    int        `json:"estimated_total" db:"estimated_total"`
	UniqueBagCount    int        `json:"unique_bag_count" db:"unique_bag_count"`
	BagClusterCount   int        `json:"bag_cluster_count" db:"bag_cluster_count"`
	PeakBagsInFrame   int        `json:"peak_bags_in_frame" db:"peak_bags_in_frame"`
	VehicleConfidence *float32   `json:"vehicle_confidence,omitempty" db:"vehicle_confidence"`
	AvgBagConfidence  *float32   `json:"avg_bag_confidence,omitempty" db:"avg_bag_confidence"`
	Status            string     `json:"status" db:"status"`
	ManualCount       *int       `json:"manual_count,omitempty" db:"manual_count"`
	CountDiscrepancy  *int       `json:"count_discrepancy,omitempty" db:"count_discrepancy"`
	VideoPath         *string    `json:"video_path,omitempty" db:"video_path"`
	VideoSizeBytes    *int64     `json:"video_size_bytes,omitempty" db:"video_size_bytes"`
	Notes             *string    `json:"notes,omitempty" db:"notes"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`

	// Joined fields — populated by certain queries
	LinkedRoomEntries []DetectionRoomEntry `json:"linked_room_entries,omitempty"`
}

// DetectionRoomEntry is the many-to-many junction between detection sessions and room entries (thocks).
// 1 vehicle can carry bags for multiple thocks (multiple customers).
// 1 large thock can arrive across multiple vehicles (multiple sessions).
type DetectionRoomEntry struct {
	ID              int       `json:"id" db:"id"`
	SessionID       int       `json:"session_id" db:"session_id"`
	RoomEntryID     int       `json:"room_entry_id" db:"room_entry_id"`
	BagCountForEntry *int     `json:"bag_count_for_entry,omitempty" db:"bag_count_for_entry"`
	LinkedByUserID  *int      `json:"linked_by_user_id,omitempty" db:"linked_by_user_id"`
	LinkedAt        time.Time `json:"linked_at" db:"linked_at"`

	// Joined fields
	ThockNumber  string `json:"thock_number,omitempty"`
	CustomerName string `json:"customer_name,omitempty"`
	RoomNo       string `json:"room_no,omitempty"`
}

// DetectionEvent represents a single frame-level detection result.
type DetectionEvent struct {
	ID              int64           `json:"id" db:"id"`
	SessionID       int             `json:"session_id" db:"session_id"`
	FrameTimestamp  time.Time       `json:"frame_timestamp" db:"frame_timestamp"`
	BagCount        int             `json:"bag_count" db:"bag_count"`
	ClusterCount    int             `json:"cluster_count" db:"cluster_count"`
	VehicleDetected bool            `json:"vehicle_detected" db:"vehicle_detected"`
	Detections      json.RawMessage `json:"detections,omitempty" db:"detections"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// CreateDetectionSessionRequest is the payload from the Python detection service.
type CreateDetectionSessionRequest struct {
	GateID            string   `json:"gate_id"`
	StartedAt         string   `json:"started_at"`
	EndedAt           string   `json:"ended_at"`
	DurationSeconds   int      `json:"duration_seconds"`
	EstimatedTotal    int      `json:"estimated_total"`
	UniqueBagCount    int      `json:"unique_bag_count"`
	BagClusterCount   int      `json:"bag_cluster_count"`
	PeakBagsInFrame   int      `json:"peak_bags_in_frame"`
	VehicleConfidence *float32 `json:"vehicle_confidence"`
	AvgBagConfidence  *float32 `json:"avg_bag_confidence"`
	VideoPath         string   `json:"video_path,omitempty"`
	VideoSizeBytes    *int64   `json:"video_size_bytes,omitempty"`
}

// UpdateDetectionSessionRequest is used to link a session to guard entry, add manual count, etc.
type UpdateDetectionSessionRequest struct {
	GuardEntryID *int    `json:"guard_entry_id"`
	ManualCount  *int    `json:"manual_count"`
	Notes        *string `json:"notes"`
	Status       string  `json:"status"`
}

// LinkRoomEntryRequest links a detection session to a room entry (thock).
type LinkRoomEntryRequest struct {
	RoomEntryID     int  `json:"room_entry_id"`
	BagCountForEntry *int `json:"bag_count_for_entry,omitempty"`
}
