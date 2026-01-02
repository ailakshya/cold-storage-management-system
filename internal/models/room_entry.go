package models

import "time"

type RoomEntry struct {
	ID                int               `json:"id"`
	EntryID           int               `json:"entry_id"`
	ThockNumber       string            `json:"thock_number"`
	RoomNo            string            `json:"room_no"`
	Floor             string            `json:"floor"`
	GateNo            string            `json:"gate_no"`
	Remark            string            `json:"remark"`
	Quantity          int               `json:"quantity"`
	QuantityBreakdown string            `json:"quantity_breakdown"`
	CreatedByUserID   int               `json:"created_by_user_id"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	Variety           string            `json:"variety"` // From joined entries table (entries.remark)
	Gatars            []RoomEntryGatar  `json:"gatars,omitempty"`
}

// RoomEntryGatar represents per-gatar quantity breakdown for a room entry
type RoomEntryGatar struct {
	ID          int       `json:"id"`
	RoomEntryID int       `json:"room_entry_id"`
	GatarNo     int       `json:"gatar_no"`
	Quantity    int       `json:"quantity"`
	Quality     string    `json:"quality"` // N=Normal, U=Unka, D=Damaged, G=Good
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GatarInput is used for creating/updating gatar quantities
type GatarInput struct {
	GatarNo  int    `json:"gatar_no"`
	Quantity int    `json:"quantity"`
	Quality  string `json:"quality"`
	Remark   string `json:"remark"`
}

type CreateRoomEntryRequest struct {
	EntryID           int          `json:"entry_id"`
	ThockNumber       string       `json:"thock_number"`
	RoomNo            string       `json:"room_no"`
	Floor             string       `json:"floor"`
	GateNo            string       `json:"gate_no"`
	Remark            string       `json:"remark"`
	Quantity          int          `json:"quantity"`
	QuantityBreakdown string       `json:"quantity_breakdown"`
	Gatars            []GatarInput `json:"gatars,omitempty"` // Per-gatar quantity breakdown
}

type UpdateRoomEntryRequest struct {
	RoomNo            string       `json:"room_no"`
	Floor             string       `json:"floor"`
	GateNo            string       `json:"gate_no"`
	Remark            string       `json:"remark"`
	Quantity          int          `json:"quantity"`
	QuantityBreakdown string       `json:"quantity_breakdown"`
	Gatars            []GatarInput `json:"gatars,omitempty"` // Per-gatar quantity breakdown
}
