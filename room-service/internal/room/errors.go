package room

import "errors"

// Domain errors returned by the room service. Adapters map them to their own protocol
// (REST handler: HTTP status, gRPC server: status code).

// ErrInvalidInput marks errors caused by bad caller input (handler maps it to 400)
var ErrInvalidInput = errors.New("invalid input")
var ErrNotFound = errors.New("room not found")
var ErrReservationReleased = errors.New("reservation already released")
var ErrRoomUnavailable = errors.New("room unavailable")      //overlap, maintain, not active
var ErrReserved = errors.New("conflict, room is being held") // a RESERVED / CONFIRMED hold covers the nights (REST 409)
var ErrNumberTaken = errors.New("number already taken")      // another room has this number (REST 409)
