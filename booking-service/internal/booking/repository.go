package booking

import "context"

type BookingRespository interface {
	GetBookingDetailByID(ctx context.Context, id string) (*BookingDetail, error)
	GetBookingDetailByUserID(ctx context.Context, userID string) ([]*BookingDetail, error)
	CreateBooking(ctx context.Context, bookingDetail *BookingDetail) (string, error)
	UpdateBookingStatus(ctx context.Context, id string, status BookingStatus) error
	UpdateBookingPaymentStatus(ctx context.Context, id string, paymentStatus PaymentStatus) error
}
