package room

import "time"

type Room struct {
	ID        string     `json:"id" db:"id"`
	Number    string     `json:"number" db:"number"`
	Type      RoomType   `json:"type" db:"type"`
	Status    RoomStatus `json:"status" db:"status"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

type RoomStatus string
type RoomType string

const (
	Active   RoomStatus = "ACTIVE"   // Room is open for business
	Inactive RoomStatus = "INACTIVE" // Out of order indefinitely
	Archived RoomStatus = "ARCHIVED" // Physically removed/deleted
)

const (
	Standard RoomType = "STANDARD"
	Deluxe   RoomType = "DELUXE"
	Suite    RoomType = "SUITE"
)

func (s RoomStatus) isValid() bool {
	switch s {
	case Active, Inactive, Archived:
		return true
	}
	return false
}

func (t RoomType) isValid() bool {
	switch t {
	case Standard, Deluxe, Suite:
		return true
	}
	return false
}
