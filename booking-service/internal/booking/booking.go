package booking

import "time"

// Map to booking table in DB
type Booking struct {
	ID string `db:"id" json:"id"`

	// User snapshot
	UserID          string `db:"user_id" json:"user_id"`
	UserName        string `db:"user_name" json:"user_name"`
	UserEmail       string `db:"user_email" json:"user_email"`
	UserPhoneNumber string `db:"user_phone_number" json:"user_phone_number"`

	// Room snapshot
	RoomID     string `db:"room_id" json:"room_id"`
	RoomNumber string `db:"room_number" json:"room_number"`
	RoomType   string `db:"room_type" json:"room_type"`

	// Booking information
	CheckInDate    time.Time `db:"check_in_date" json:"check_in_date"`
	CheckOutDate   time.Time `db:"check_out_date" json:"check_out_date"`
	NumberOfGuests int       `db:"number_of_guests" json:"number_of_guests"`

	// Money is stored in minor units.
	// Example: USD 100.25 -> 10025
	TotalAmount int64  `db:"total_amount" json:"total_amount"`
	Currency    string `db:"currency" json:"currency"`

	Status        BookingStatus `db:"status" json:"status"`
	PaymentStatus PaymentStatus `db:"payment_status" json:"payment_status"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Booking detail includes nightly rates
type BookingDetail struct {
	Booking
	NightlyRates []NightlyRate `json:"nightly_rates"`
}

// Booking status
type BookingStatus string

const (
	StatusPending           BookingStatus = "PENDING"
	StatusReserved          BookingStatus = "RESERVED"
	StatusReservationFailed BookingStatus = "RESERVATION_FAILED"
	StatusPaymentFailed     BookingStatus = "PAYMENT_FAILED"
	StatusBooked            BookingStatus = "BOOKED"
	StatusCancelled         BookingStatus = "CANCELLED"
	StatusCheckedIn         BookingStatus = "CHECKED_IN"
	StatusCheckedOut        BookingStatus = "CHECKED_OUT"
	StatusNoShow            BookingStatus = "NO_SHOW"
)

func (s BookingStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusReserved, StatusReservationFailed, StatusPaymentFailed, StatusBooked, StatusCancelled, StatusCheckedIn, StatusCheckedOut, StatusNoShow:
		return true
	}
	return false
}

// Payment status
type PaymentStatus string

const (
	PaymentPending           PaymentStatus = "PENDING"
	PaymentCompleted         PaymentStatus = "COMPLETED"
	PaymentFailed            PaymentStatus = "FAILED"
	PaymentRefunded          PaymentStatus = "REFUNDED"           // Full refund
	PaymentPartiallyRefunded PaymentStatus = "PARTIALLY_REFUNDED" // Partial refund. Eg: user cancels after check-in, so only refund for unused nights
)

func (s PaymentStatus) IsValid() bool {
	switch s {
	case PaymentPending, PaymentCompleted, PaymentFailed, PaymentRefunded, PaymentPartiallyRefunded:
		return true
	}
	return false
}

func (s PaymentStatus) IsTerminal() bool {
	return s == PaymentRefunded || s == PaymentFailed
}
