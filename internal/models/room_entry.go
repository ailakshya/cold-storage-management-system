package models

import "time"

type RoomEntry struct {
	ID                int       `json:"id"`
	EntryID           int       `json:"entry_id"`
	ThockNumber       string    `json:"thock_number"`
	RoomNo            string    `json:"room_no"`
	Floor             string    `json:"floor"`
	GateNo            string    `json:"gate_no"`
	Remark            string    `json:"remark"`
	Quantity          int       `json:"quantity"`
	QuantityBreakdown string    `json:"quantity_breakdown"`
	CreatedByUserID   int       `json:"created_by_user_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Variety           string    `json:"variety"` // From joined entries table (entries.remark)
}

type CreateRoomEntryRequest struct {
	EntryID           int    `json:"entry_id"`
	ThockNumber       string `json:"thock_number"`
	RoomNo            string `json:"room_no"`
	Floor             string `json:"floor"`
	GateNo            string `json:"gate_no"`
	Remark            string `json:"remark"`
	Quantity          int    `json:"quantity"`
	QuantityBreakdown string `json:"quantity_breakdown"`
}

type UpdateRoomEntryRequest struct {
	RoomNo            string `json:"room_no"`
	Floor             string `json:"floor"`
	GateNo            string `json:"gate_no"`
	Remark            string `json:"remark"`
	Quantity          int    `json:"quantity"`
	QuantityBreakdown string `json:"quantity_breakdown"`
}
