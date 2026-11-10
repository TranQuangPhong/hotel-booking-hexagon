package booking

import (
	"context"
)

type Service struct {
	repository Repository
}

func NewService(repo Repository) *Service {
	return &Service{repository: repo}
}

// GetByID returns the repository's error as is: it already says what failed (ErrNotFound or a wrapped DB error)
func (s *Service) GetByID(ctx context.Context, id string) (*Booking, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) GetByUserID(ctx context.Context, userID string) ([]*Booking, error) {
	return s.repository.GetByUserID(ctx, userID)
}

// Create validates b, sets both statuses to PENDING and stores it. b: filled by the caller, except ID,
// Status, PaymentStatus, CreatedAt, UpdatedAt. Returns the stored booking: on a repeated saga_id that is the
// existing row (its current status), so callers use the return value, not b.
func (s *Service) Create(ctx context.Context, b *Booking) (*Booking, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	// Copy, so the caller's b keeps what it sent (same rule as the repository: no layer modifies its argument)
	nb := *b
	nb.Status = StatusPending
	nb.PaymentStatus = PaymentPending

	// The repository's error already says which step failed (insert, or the read after a saga_id conflict)
	return s.repository.Create(ctx, &nb)
}
