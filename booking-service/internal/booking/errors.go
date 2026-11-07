package booking

import "errors"

// Domain errors returned by the booking service. Adapters map them to their own protocol
// (REST handler: HTTP status, gRPC server: status code).

var (
	ErrInvalidInput = errors.New("invalid input") // handler maps it to 400 and gRPC maps it to INVALID_ARGUMENT
	ErrNotFound     = errors.New("booking not found")
)
