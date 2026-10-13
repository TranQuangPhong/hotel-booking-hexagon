package roomv1

// Kafka side of the room contract: topic names, type names and data structs.

// topics
const (
	TopicCommands = "room.commands"
	TopicEvents   = "room.events"
)

// message types
const (
	//event
	TypeReservationConfirmed = "ReservationConfirmed"
	TypeRoomReleased         = "RoomReleased"
	//cmd
	TypeConfirmReservation = "ConfirmReservation"
	TypeReleaseRoom        = "ReleaseRoom"
)

type ReservationConfirmed struct {
	ReservationID string `json:"reservation_id"`
}

type RoomReleased struct {
	ReservationID *string `json:"reservation_id"` // nil (JSON null) if there was nothing to release
}

// cmd to confirm reservation.
// No data: the reservation is found by the envelope's saga_id
type ConfirmReservation struct{}

// cmd to release room.
// The reservation is found by the envelope's saga_id
type ReleaseRoom struct {
	Reason string `json:"reason"` //DEADLINE, PHASE_A_ERROR
}
