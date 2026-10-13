package bookingv1

// Kafka side of the booking contract: topic names, type names and data structs.

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

type BookingExpired struct {
	BookingID *string `json:"booking_id"` // nil (JSON null) if there was no booking
}

// cmd to confirm booking.
// No data: the booking is found by the envelope's saga_id
type ConfirmBooking struct{}

// cmd to expire booking.
// The booking is found by the envelope's saga_id
type ExpireBooking struct {
	Reason string `json:"reason"`
}
