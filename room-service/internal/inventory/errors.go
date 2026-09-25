package inventory

// Domain errors returned by the inventory service, e.g. ErrRoomUnavailable.
// The postgres adapter maps SQLSTATE 23P01 (overlapping reservation) to
// ErrRoomUnavailable; the gRPC adapter maps it to FAILED_PRECONDITION.
