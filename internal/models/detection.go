package models

import (
	"encoding/json"
	"time"
)

// DetectionSession represents a single vehicle unloading event at a gate,
// as detected by the YOLOv8 inference service.
type DetectionSession struct {
	ID                int        `json:"id" db:"id"`
	GateID            string     `json:"gate_id" db:"gate_id"`
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
	MatchedGatePassID *int       `json:"matched_gate_pass_id,omitempty" db:"matched_gate_pass_id"`
	ManualCount       *int       `json:"manual_count,omitempty" db:"manual_count"`
	CountDiscrepancy  *int       `json:"count_discrepancy,omitempty" db:"count_discrepancy"`
	Notes             *string    `json:"notes,omitempty" db:"notes"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
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
}

// UpdateDetectionSessionRequest is used to link a session to a gate pass or add manual count.
type UpdateDetectionSessionRequest struct {
	MatchedGatePassID *int    `json:"matched_gate_pass_id"`
	ManualCount       *int    `json:"manual_count"`
	Notes             *string `json:"notes"`
	Status            string  `json:"status"`
}
