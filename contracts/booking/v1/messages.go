// Package bookingv1 holds booking-service's public contracts: the BookingService
// gRPC API (CreateBooking, generated from booking.proto) and the Kafka messages
// on its topics.
//
//   - booking.commands (booking accepts): ConfirmBooking, ExpireBooking
//   - booking.events   (booking emits):   BookingConfirmed, BookingExpired
//
// This file holds the Kafka side: type names, topic names and data structs.
package bookingv1

// topics
const (
	TopicCommands = "booking.commands"
	TopicEvents   = "booking.events"
)

// message types
const (
	//event
	TypeBookingConfirmed = "BookingConfirmed"
	TypeBookingExpired   = "BookingExpired"
	//cmd
	TypeConfirmBooking = "ConfirmBooking"
	TypeExpireBooking  = "ExpireBooking"
)

// message structures
type BookingConfirmed struct {
	BookingID       string `json:"booking_id"`
	UserID          string `json:"user_id"`
	UserName        string `json:"user_name"`
	UserEmail       string `json:"user_email"`
	UserPhoneNumber string `json:"user_phone_number"`
	RoomNumber      string `json:"room_number"`
	RoomType        string `json:"room_type"`
	CheckInDate     string `json:"check_in_date"`  //"YYYY-MM-DD"
	CheckOutDate    string `json:"check_out_date"` //"YYYY-MM-DD"
	NumberOfGuests  int    `json:"number_of_guests"`
	TotalAmount     int64  `json:"total_amount"`
	Currency        string `json:"currency"`
}

// message structures
type BookingExpired struct {
	BookingID string `json:"booking_id"` //null if booking not exists
}

// cmd to confirm booking
type ConfirmBooking struct {
	BookingID string `json:"booking_id"`
}

// cmd to expire booking
type ExpireBooking struct {
	BookingID string `json:"booking_id"`
	Reason    string `json:"reason"`
}
