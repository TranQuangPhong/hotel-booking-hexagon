package room

import (
	"context"
	"time"
)

type Repository interface {
	GetAll(ctx context.Context) ([]*Room, error)
	// GetByID returns ErrNotFound when no room has this id
	GetByID(ctx context.Context, id string) (*Room, error)
	// Create inserts the room and its inventories in one transaction
	Create(ctx context.Context, room *Room, inventories []*Inventory) (*Room, error)
	// Update returns ErrNotFound when no room has room.ID
	Update(ctx context.Context, room *Room) (*Room, error)

	// the rows of every month from firstNight to lastNight, ordered by (year, month): setRates takes the currency
	// from the first row. A missing month is just absent, not an error
	GetInventories(ctx context.Context, roomID string, firstNight, lastNight time.Time) ([]*Inventory, error)

	// nil, nil when the saga has no reservation yet: the normal first call, not an error
	GetReservationBySagaID(ctx context.Context, sagaID string) (*Reservation, error)
	// returns the stored row; ErrRoomUnavailable on an overlap
	CreateReservation(ctx context.Context, reservation *Reservation) (*Reservation, error)
}
