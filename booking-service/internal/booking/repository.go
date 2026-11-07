package booking

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Booking, error)
	GetByUserID(ctx context.Context, userID string) ([]*Booking, error)
	Create(ctx context.Context, booking *Booking) (string, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	UpdatePaymentStatus(ctx context.Context, id string, paymentStatus PaymentStatus) error
}
