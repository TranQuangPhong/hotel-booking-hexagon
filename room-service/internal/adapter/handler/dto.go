package handler

import "booking/room-service/internal/room"

type CreateRoomRequest struct {
	Number string `json:"number" binding:"required"`
	Type   string `json:"type" binding:"required"`
	Status string `json:"status"`
}

func (req *CreateRoomRequest) ToRoom() *room.Room {
	if req.Status == "" {
		req.Status = string(room.Active)
	}
	return &room.Room{
		Number: req.Number,
		Type:   room.RoomType(req.Type),
		Status: room.RoomStatus(req.Status),
	}
}

type UpdateRoomRequest struct {
	Number string `json:"number" binding:"required"`
	Type   string `json:"type" binding:"required"`
	Status string `json:"status" binding:"required"`
}

func (req UpdateRoomRequest) ToRoom(id string) *room.Room {
	return &room.Room{
		ID:     id,
		Number: req.Number,
		Type:   room.RoomType(req.Type),
		Status: room.RoomStatus(req.Status)}
}
