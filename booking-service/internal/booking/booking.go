package booking

import (
	"fmt"
	"time"
)

// Map to bookings table in DB
type Booking struct {
	ID            string `db:"id" json:"id"`
	SagaID        string `db:"saga_id" json:"saga_id"`
	ReservationID string `db:"reservation_id" json:"reservation_id"`

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
	CheckInDate    time.Time `db:"check_in_date" json:"check_in_date"`   // midnight UTC
	CheckOutDate   time.Time `db:"check_out_date" json:"check_out_date"` // midnight UTC
	NumberOfGuests int       `db:"number_of_guests" json:"number_of_guests"`

	// Money is stored in minor units.
	// Example: USD 100.25 -> 10025
	TotalAmount  int64         `db:"total_amount" json:"total_amount"`
	Currency     string        `db:"currency" json:"currency"`
	NightlyRates []NightlyRate `db:"nightly_rates" json:"nightly_rates"`

	Status        Status        `db:"status" json:"status"`
	PaymentStatus PaymentStatus `db:"payment_status" json:"payment_status"`
	ExpiresAt     time.Time     `db:"expires_at" json:"expires_at"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Booking status
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusBooked     Status = "BOOKED"
	StatusExpired    Status = "EXPIRED"
	StatusCancelled  Status = "CANCELLED"
	StatusCheckedIn  Status = "CHECKED_IN"
	StatusCheckedOut Status = "CHECKED_OUT"
	StatusNoShow     Status = "NO_SHOW"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusBooked, StatusExpired, StatusCancelled, StatusCheckedIn, StatusCheckedOut, StatusNoShow:
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

// validate rejects what proto3 lets through: a forgotten field arrives as 0 or "", not as an error. No I/O.
// The error names the field, never its value: the gRPC message lands in the access log, and name, email
// and phone are personal data.
func (b *Booking) validate() error {
	if !b.CheckInDate.Before(b.CheckOutDate) {
		return fmt.Errorf("bad input date range: %w", ErrInvalidInput)
	}

	if b.NumberOfGuests <= 0 {
		return fmt.Errorf("bad input number of guests: %w", ErrInvalidInput)
	}

	// Phone may be empty: users has no phone column
	if b.UserName == "" {
		return fmt.Errorf("bad input user name: %w", ErrInvalidInput)
	}
	if b.UserEmail == "" {
		return fmt.Errorf("bad input user email: %w", ErrInvalidInput)
	}
	if b.RoomNumber == "" {
		return fmt.Errorf("bad input room number: %w", ErrInvalidInput)
	}
	if b.RoomType == "" {
		return fmt.Errorf("bad input room type: %w", ErrInvalidInput)
	}

	if len(b.Currency) != 3 {
		return fmt.Errorf("bad input currency: %w", ErrInvalidInput)
	}
	for _, c := range b.Currency {
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("bad input currency: %w", ErrInvalidInput)
		}
	}

	// One rate per night: rates[i] is check-in + i days, and the last one is the night before check-out.
	// Equal, not ==: == also compares the Location.
	var sum int64
	for i, rate := range b.NightlyRates {
		if !rate.Date.Equal(b.CheckInDate.AddDate(0, 0, i)) {
			return fmt.Errorf("bad input nightly rate date: %w", ErrInvalidInput)
		}
		if rate.Price <= 0 {
			return fmt.Errorf("bad input nightly rate price: %w", ErrInvalidInput)
		}
		sum += rate.Price
	}
	if !b.CheckInDate.AddDate(0, 0, len(b.NightlyRates)).Equal(b.CheckOutDate) {
		return fmt.Errorf("bad input nightly rates do not cover the stay: %w", ErrInvalidInput)
	}

	// Also catches a forgotten total: at least one night and every price > 0, so the sum is never 0
	if sum != b.TotalAmount {
		return fmt.Errorf("bad input total amount: %w", ErrInvalidInput)
	}

	return nil
}
