package models

import "time"

// GatePassMedia represents a photo or video attached to a gate pass
type GatePassMedia struct {
	ID               int        `json:"id" db:"id"`
	GatePassID       *int       `json:"gate_pass_id,omitempty" db:"gate_pass_id"`
	GatePassPickupID *int       `json:"gate_pass_pickup_id,omitempty" db:"gate_pass_pickup_id"`
	ThockNumber      string     `json:"thock_number" db:"thock_number"`
	MediaType        string     `json:"media_type" db:"media_type"` // "entry" or "pickup"
	FilePath         string     `json:"file_path" db:"file_path"`
	FileName         string     `json:"file_name" db:"file_name"`
	FileType         string     `json:"file_type" db:"file_type"` // "image" or "video"
	FileSize         *int64     `json:"file_size,omitempty" db:"file_size"`
	UploadedByUserID *int       `json:"uploaded_by_user_id,omitempty" db:"uploaded_by_user_id"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`

	// Computed fields (from JOINs, not in database)
	UploadedByUserName string `json:"uploaded_by_user_name,omitempty" db:"uploaded_by_user_name"`
	DownloadURL        string `json:"download_url,omitempty"` // Computed field, not stored in DB
}
