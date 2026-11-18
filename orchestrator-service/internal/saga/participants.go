package saga

import "context"

// Driven ports for the synchronous calls made while the user is waiting
// (phase A): RoomClient and BookingClient. Each wraps one RPC (ReserveRoom,
// CreateBooking); the port methods drop the entity like every core method
// (Reserve, Create). Implemented by adapter/grpcclient.

// Reservation is ReserveRoom's answer; StartBooking copies it into the saga's Context
type Reservation struct {
	ID, RoomNumber, RoomType, Currency string
	TotalAmount                        int64 // minor units
	NightlyRates                       []NightlyRate
}

// RoomClient wraps room-service's ReserveRoom
type RoomClient interface {
	// Reserve calls ReserveRoom with s.ID, the room and dates from s.Context, s.DeadlineAt as expires_at.
	// Definite rejection: an error wrapping ErrRoomNotFound, ErrInvalidInput or ErrRoomUnavailable (nothing is
	// held). Any other error: outcome unknown, a hold may exist
	Reserve(ctx context.Context, s *Saga) (*Reservation, error)
}

// BookingClient wraps booking-service's CreateBooking
type BookingClient interface {
	// Create calls CreateBooking from s after RoomReserved (context incl. reservation_id, DeadlineAt as expires_at).
	// No domain errors: in M3 every error means the same (503, the hold stays)
	Create(ctx context.Context, s *Saga) (bookingID, status string, err error)
}
