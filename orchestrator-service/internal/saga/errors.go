package saga

import "errors"

var (
	ErrInvalidInput    = errors.New("invalid input")                  // 400: a bad X-User-Id, ReserveRoom INVALID_ARGUMENT
	ErrRoomNotFound    = errors.New("ROOM_NOT_FOUND")                 // 404: ReserveRoom NOT_FOUND
	ErrRoomUnavailable = errors.New("ROOM_UNAVAILABLE")               // 409: ReserveRoom FAILED_PRECONDITION
	ErrStale           = errors.New("saga changed since it was read") // Save: the version moved
)

// isRoomRejection: ReserveRoom answered with a definite no, so nothing is held. Any other Reserve error is an unknown
// outcome
func isRoomRejection(err error) bool {
	return errors.Is(err, ErrRoomNotFound) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrRoomUnavailable)
}
