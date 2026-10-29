package handler

import (
	"booking/room-service/internal/room"
	"fmt"
	"strings"
	"time"
)

type CreateRoomRequest struct {
	Number   string `json:"number" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Status   string `json:"status"`
	Price    int64  `json:"price" binding:"required,gt=0"` // minor units, eg: 12000 = 120.00 USD
	Currency string `json:"currency" binding:"required,len=3"`
}

// ToRate is the initial nightly rate for the room's inventory
func (req *CreateRoomRequest) ToRate() room.Rate {
	return room.Rate{
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

// UpdateInventoriesRequest edits the existing days from first_night to last_night, both inclusive.
// A nil status / price keeps the stored value. The price is in the room's currency: there is no currency field
// (sending one is a 400: unknown JSON fields are rejected, see main.go)
type UpdateInventoriesRequest struct {
	Status     *string `json:"status"`
	Price      *int64  `json:"price" binding:"omitempty,gt=0"`
	FirstNight string  `json:"first_night" binding:"required,datetime=2006-01-02"`
	LastNight  string  `json:"last_night" binding:"required,datetime=2006-01-02"`
}

// ToNights parses both nights as dates (time.DateOnly), so each is midnight UTC
func (req UpdateInventoriesRequest) ToNights() (firstNight, lastNight time.Time, err error) {
	firstNight, err = time.Parse(time.DateOnly, req.FirstNight)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("first_night %q: %w", req.FirstNight, room.ErrInvalidInput)
	}
	lastNight, err = time.Parse(time.DateOnly, req.LastNight)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("last_night %q: %w", req.LastNight, room.ErrInvalidInput)
	}
	return firstNight, lastNight, nil
}

// ToDayPatch keeps a missing field nil; the service checks the status value
func (req UpdateInventoriesRequest) ToDayPatch() room.DayPatch {
	patch := room.DayPatch{Price: req.Price}
	if req.Status != nil {
		status := room.DayStatus(*req.Status)
		patch.Status = &status
	}
	return patch
}
