package room

import "time"

type Room struct {
	ID        string     `json:"id" db:"id"`
	Number    string     `json:"number" db:"number"`
	Type      Type   `json:"type" db:"type"`
	Status    Status `json:"status" db:"status"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

type Status string
type Type string

const (
	Active   Status = "ACTIVE"   // Room is open for business
	Inactive Status = "INACTIVE" // Out of order indefinitely
	Archived Status = "ARCHIVED" // Physically removed/deleted
)

const (
	Standard Type = "STANDARD"
	Deluxe   Type = "DELUXE"
	Suite    Type = "SUITE"
)

func (s Status) isValid() bool {
	switch s {
	case Active, Inactive, Archived:
		return true
	}
	return false
}

func (t Type) isValid() bool {
	switch t {
	case Standard, Deluxe, Suite:
		return true
	}
	return false
}
