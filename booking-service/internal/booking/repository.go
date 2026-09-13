package booking

import "context"

type BookingRespository interface {
	GetBookingByID(ctx context.Context, id string) (*Booking, error)
	GetBookingByUserID(ctx context.Context, userID string) ([]*Booking, error)
	CreateBooking(ctx context.Context, bookingDetail *BookingDetail) (string, error)
	UpdateBookingStatus(ctx context.Context, id string, status BookingStatus) error
	UpdateBookingPaymentStatus(ctx context.Context, id string, paymentStatus PaymentStatus) error
}
