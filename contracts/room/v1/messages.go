// Package roomv1 holds room-service's public contracts: the RoomService gRPC API
// (generated from room.proto) and the Kafka messages on its topics.
//
//   - room.commands (room accepts): ConfirmReservation, ReleaseRoom
//   - room.events   (room emits):   ReservationConfirmed, RoomReleased
//
// This file holds the Kafka side: type names, topic names and data structs.
package roomv1

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
	ReservationID string `json:"reservation_id"` // null if reservation not exists
}

// cmd to confirm reservation.
// No data: the reservation is found by the envelope's saga_id
type ConfirmReservation struct{}

// cmd to release room.
// The reservation is found by the envelope's saga_id
type ReleaseRoom struct {
	Reason string `json:"reason"` //DEADLINE, PHASE_A_ERROR
}
