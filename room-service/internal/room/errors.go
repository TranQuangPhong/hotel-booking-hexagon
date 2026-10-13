package room

import "errors"

// Domain errors returned by the room service. Adapters map them to their own protocol
// (REST handler: HTTP status, gRPC server: status code).

// ErrInvalidInput marks errors caused by bad caller input (handler maps it to 400)
var ErrInvalidInput = errors.New("invalid input")
