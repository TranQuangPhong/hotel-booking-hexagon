package room

import "context"

type Repository interface {
	GetAll(ctx context.Context) ([]*Room, error)
	GetByID(ctx context.Context, id string) (*Room, error)
	Create(ctx context.Context, Room *Room) (*Room, error)
	Update(ctx context.Context, Room *Room) (*Room, error)
}
