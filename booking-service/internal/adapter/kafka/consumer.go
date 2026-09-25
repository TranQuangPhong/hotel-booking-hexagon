// Package kafka is the driving adapter for booking-service's Kafka commands.
//
// The consumer reads booking.commands (ConfirmBooking, ExpireBooking), finds the
// booking by the envelope's sagaId and calls the booking service.
package kafka
