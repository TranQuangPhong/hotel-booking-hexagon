package handler

import (
	"booking/booking-service/internal/booking"
	"time"
)

// BookingResponse is GET /bookings/api/v1/:id. Fields keep their column names; stay and rate dates are
// YYYY-MM-DD (contracts.md), timestamps RFC 3339 UTC. No saga_id / reservation_id: internal to the saga
type BookingResponse struct {
	ID string `json:"id"`

	UserID          string `json:"user_id"`
	UserName        string `json:"user_name"`
	UserEmail       string `json:"user_email"`
	UserPhoneNumber string `json:"user_phone_number"`

	RoomID     string `json:"room_id"`
	RoomNumber string `json:"room_number"`
	RoomType   string `json:"room_type"`

	CheckInDate    string `json:"check_in_date"`
	CheckOutDate   string `json:"check_out_date"`
	NumberOfGuests int    `json:"number_of_guests"`

	TotalAmount  int64                 `json:"total_amount"`
	Currency     string                `json:"currency"`
	NightlyRates []NightlyRateResponse `json:"nightly_rates"`

	Status        string    `json:"status"`
	PaymentStatus string    `json:"payment_status"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NightlyRateResponse is the REST form of a rate. Its own type, not booking.NightlyRate: that one's JSON is
// the JSONB column format, and the API shouldn't change when the storage does
type NightlyRateResponse struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Price int64  `json:"price"`
}

func toBookingResponse(b *booking.Booking) BookingResponse {
	rates := make([]NightlyRateResponse, 0, len(b.NightlyRates))
	for _, rate := range b.NightlyRates {
		rates = append(rates, NightlyRateResponse{Date: rate.Date.Format(time.DateOnly), Price: rate.Price})
	}
	return BookingResponse{
		ID:              b.ID,
		UserID:          b.UserID,
		UserName:        b.UserName,
		UserEmail:       b.UserEmail,
		UserPhoneNumber: b.UserPhoneNumber,
		RoomID:          b.RoomID,
		RoomNumber:      b.RoomNumber,
		RoomType:        b.RoomType,
		CheckInDate:     b.CheckInDate.Format(time.DateOnly),
		CheckOutDate:    b.CheckOutDate.Format(time.DateOnly),
		NumberOfGuests:  b.NumberOfGuests,
		TotalAmount:     b.TotalAmount,
		Currency:        b.Currency,
		NightlyRates:    rates,
		Status:          string(b.Status),
		PaymentStatus:   string(b.PaymentStatus),
		ExpiresAt:       b.ExpiresAt,
		CreatedAt:       b.CreatedAt,
		UpdatedAt:       b.UpdatedAt,
	}
}
