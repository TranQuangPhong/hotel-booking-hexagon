package inventory

import "time"

type Inventory struct {
	ID        int64                 `json:"id" db:"id"`
	RoomID    string                `json:"room_id" db:"room_id"` // UUID from Room model
	Year      int16                 `json:"year" db:"year"`       // YYYY
	Month     int16                 `json:"month" db:"month"`     // 1 - 12
	Days      map[int8]Day `json:"days" db:"days"`       // Stored as json for each day
	CreatedAt time.Time             `json:"created_at" db:"created_at"`
	UpdatedAt time.Time             `json:"updated_at" db:"updated_at"`
}

type Day struct {
	Status    DayStatus `json:"status"`
	Price     int64              `json:"price"` // minor-unit value of currency. Eg: USA -> store CENT value
	Currency  string             `json:"currency"`
	BookingID string             `json:"booking_id,omitempty"` // reference purpose only
}

type DayStatus string

const (
	StatusAvailable DayStatus = "AVAILABLE"
	// PENDING status for rooms that are in the process of being booked but not yet confirmed.
	// This can help prevent race conditions where multiple users try to book the same room at the same time.
	StatusReserved    DayStatus = "RESERVED"
	StatusBooked      DayStatus = "BOOKED"
	StatusMaintenance DayStatus = "MAINTENANCE"
)

func (s DayStatus) IsValid() bool {
	switch s {
	case StatusAvailable, StatusBooked, StatusMaintenance, StatusReserved:
		return true
	}
	return false
}
