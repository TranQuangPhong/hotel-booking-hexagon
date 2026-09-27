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
	TopicCommands = "booking.commands"
	TopicEvents   = "booking.events"
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

// cmd to confirm reservation
type ConfirmReservation struct {
	ReservationID string `json:"reservation_id"`
}

// cmd to release room
type ReleaseRoom struct {
	ReservationID string `json:"reservation_id"`
	Reason        string `json:"reason"` //DEADLINE, PHASE_A_ERROR
}
