package models

import "time"

// GatarBreakdown represents quantity picked from a specific gatar
type GatarBreakdown struct {
	ID        int       `json:"id,omitempty" db:"id"`
	PickupID  int       `json:"pickup_id,omitempty" db:"pickup_id"`
	GatarNo   int       `json:"gatar_no" db:"gatar_no"`
	Quantity  int       `json:"quantity" db:"quantity"`
	CreatedAt time.Time `json:"created_at,omitempty" db:"created_at"`
}

type GatePassPickup struct {
	ID                 int              `json:"id" db:"id"`
	GatePassID         int              `json:"gate_pass_id" db:"gate_pass_id"`
	PickupQuantity     int              `json:"pickup_quantity" db:"pickup_quantity"`
	PickedUpByUserID   int              `json:"picked_up_by_user_id" db:"picked_up_by_user_id"`
	PickupTime         time.Time        `json:"pickup_time" db:"pickup_time"`
	RoomNo             *string          `json:"room_no,omitempty" db:"room_no"`
	Floor              *string          `json:"floor,omitempty" db:"floor"`
	Remarks            *string          `json:"remarks,omitempty" db:"remarks"`
	CreatedAt          time.Time        `json:"created_at" db:"created_at"`
	PickedUpByUserName string           `json:"picked_up_by_user_name,omitempty" db:"picked_up_by_user_name"`
	GatarBreakdown     []GatarBreakdown `json:"gatar_breakdown,omitempty"`
}
