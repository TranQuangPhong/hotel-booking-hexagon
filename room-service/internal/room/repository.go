package room

import (
	"booking/room-service/internal/inventory"
	"context"
)

type Repository interface {
	GetAll(ctx context.Context) ([]*Room, error)
	GetByID(ctx context.Context, id string) (*Room, error)
	// Create inserts the room and its inventories in one transaction
	Create(ctx context.Context, room *Room, inventories []*inventory.Inventory) (*Room, error)
	Update(ctx context.Context, Room *Room) (*Room, error)
}
