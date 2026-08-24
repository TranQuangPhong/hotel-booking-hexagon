package handler

import "booking/room-service/internal/room"

type CreateRoomRequest struct{}

func (req *CreateRoomRequest) ToRoom() *room.Room {
	return &room.Room{}
}

type UpdateRoomRequest struct{}

func (req UpdateRoomRequest) ToRoom(id string) *room.Room {
	return &room.Room{}
}
