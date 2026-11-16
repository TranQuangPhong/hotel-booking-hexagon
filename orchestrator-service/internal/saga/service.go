package saga

import "time"

// Orchestrator use cases, called by the driving adapters:
//   - StartBooking: POST /bookings, phase A (ReserveRoom -> CreateBooking)

// StartRequest: POST /bookings, filled by the handler
type StartRequest struct {
	User                      User
	RoomID                    string
	CheckInDate, CheckOutDate string // YYYY-MM-DD
	NumberOfGuests            int
}

// Started: the data of the 201
type Started struct {
	BookingID, BookingStatus string // as CreateBooking returned them (PENDING for a new saga)
	TotalAmount              int64
	Currency                 string
	ExpiresAt                time.Time // the saga's deadline_at
}
