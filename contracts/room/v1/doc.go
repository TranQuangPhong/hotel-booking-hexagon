// Package roomv1 holds room-service's public contracts: the RoomService gRPC API
// (ReserveRoom, generated from room.proto) and the Kafka messages on its topics.
//
//   - room.commands (room accepts): ConfirmReservation, ReleaseRoom
//   - room.events   (room emits):   ReservationConfirmed, RoomReleased
package roomv1
