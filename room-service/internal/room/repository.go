package room

import "context"

type Repository interface {
	GetAll(ctx context.Context) ([]*Room, error)
	GetByID(ctx context.Context, id string) (*Room, error)
	// Create inserts the room and its inventories in one transaction
	Create(ctx context.Context, room *Room, inventories []*Inventory) (*Room, error)
	Update(ctx context.Context, room *Room) (*Room, error)
}
