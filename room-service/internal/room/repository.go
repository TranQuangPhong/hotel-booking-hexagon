package room

import (
	"context"
	"time"
)

type Repository interface {
	GetAll(ctx context.Context) ([]*Room, error)
	// GetByID returns ErrNotFound when no room has this id
	GetByID(ctx context.Context, id string) (*Room, error)
	// Create inserts the room and its inventories in one transaction; ErrNumberTaken when another room has the number
	Create(ctx context.Context, room *Room, inventories []*Inventory) (*Room, error)
	// Update returns ErrNotFound when no room has room.ID, ErrNumberTaken when another room has the number
	Update(ctx context.Context, room *Room) (*Room, error)

	// the rows of every month from firstNight to lastNight, ordered by (year, month): setRates takes the currency
	// from the first row. A missing month is just absent, not an error
	GetInventories(ctx context.Context, roomID string, firstNight, lastNight time.Time) ([]*Inventory, error)

	// SaveInventories writes back each row's days by id and refreshes its UpdatedAt from the DB.
	// One transaction: a database error saves no row
	SaveInventories(ctx context.Context, inventories []*Inventory) error

	// HasReservation reports whether a RESERVED or CONFIRMED reservation holds any night from firstNight to lastNight
	HasReservation(ctx context.Context, roomID string, firstNight, lastNight time.Time) (bool, error)

	// nil, nil when the saga has no reservation yet: the normal first call, not an error
	GetReservationBySagaID(ctx context.Context, sagaID string) (*Reservation, error)
	// returns the stored row; ErrRoomUnavailable on an overlap
	CreateReservation(ctx context.Context, reservation *Reservation) (*Reservation, error)
}
