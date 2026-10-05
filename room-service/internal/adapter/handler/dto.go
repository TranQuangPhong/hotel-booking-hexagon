package handler

import (
	"booking/room-service/internal/inventory"
	"booking/room-service/internal/room"
	"strings"
)

type CreateRoomRequest struct {
	Number   string `json:"number" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Status   string `json:"status"`
	Price    int64  `json:"price" binding:"required,gt=0"` // minor units, eg: 12000 = 120.00 USD
	Currency string `json:"currency" binding:"required,len=3"`
}

// ToRate is the initial nightly rate for the room's inventory
func (req *CreateRoomRequest) ToRate() inventory.Rate {
	return inventory.Rate{
		Price:    req.Price,
		Currency: strings.ToUpper(req.Currency),
	}
}

func (req *CreateRoomRequest) ToRoom() *room.Room {
	if req.Status == "" {
		req.Status = string(room.Active)
	}
	return &room.Room{
		Number: req.Number,
		Type:   room.Type(req.Type),
		Status: room.Status(req.Status),
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
		Type:   room.Type(req.Type),
		Status: room.Status(req.Status)}
}
