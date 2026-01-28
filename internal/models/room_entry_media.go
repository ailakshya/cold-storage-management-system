package models

import "time"

type RoomEntryMedia struct {
	ID               int       `json:"id"`
	RoomEntryID      int       `json:"room_entry_id"`
	ThockNumber      string    `json:"thock_number"`
	MediaType        string    `json:"media_type"`
	FilePath         string    `json:"file_path"`
	FileName         string    `json:"file_name"`
	FileType         string    `json:"file_type"`
	FileSize         *int64    `json:"file_size,omitempty"`
	UploadedByUserID *int      `json:"uploaded_by_user_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`

	// Computed fields (from JOINs)
	UploadedByUserName string `json:"uploaded_by_user_name,omitempty"`
	DownloadURL        string `json:"download_url,omitempty"`
}
