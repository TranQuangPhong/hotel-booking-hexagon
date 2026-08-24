package room

import "context"

type Repository interface {
	GetRooms(ctx context.Context) ([]*Room, error)
	GetRoomByID(ctx context.Context, id string) (*Room, error)
	CreateRoom(ctx context.Context, Room *Room) (*Room, error)
	UpdateRoom(ctx context.Context, Room *Room) (*Room, error)
}
