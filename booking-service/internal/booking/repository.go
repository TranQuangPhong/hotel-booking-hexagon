package booking

import "context"

type Repository interface {
	GetDetailByID(ctx context.Context, id string) (*Detail, error)
	GetDetailsByUserID(ctx context.Context, userID string) ([]*Detail, error)
	Create(ctx context.Context, bookingDetail *Detail) (string, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	UpdatePaymentStatus(ctx context.Context, id string, paymentStatus PaymentStatus) error
}
