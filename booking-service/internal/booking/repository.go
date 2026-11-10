package booking

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Booking, error)
	GetByUserID(ctx context.Context, userID string) ([]*Booking, error)
	// Create inserts the booking and returns the stored row as a new value; the argument is not modified.
	// Idempotent by saga_id: a repeated call returns the existing row as stored, even if the rest differs
	Create(ctx context.Context, b *Booking) (*Booking, error)
}
