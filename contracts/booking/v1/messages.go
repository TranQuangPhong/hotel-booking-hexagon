// Package bookingv1 holds booking-service's public contracts: the BookingService
// gRPC API (CreateBooking, generated from booking.proto) and the Kafka messages
// on its topics.
//
//   - booking.commands (booking accepts): ConfirmBooking, ExpireBooking
//   - booking.events   (booking emits):   BookingConfirmed, BookingExpired
//
// This file holds the Kafka side: type names, topic names and data structs.
package bookingv1
