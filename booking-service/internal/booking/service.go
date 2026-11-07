package booking

import (
	"context"
	"errors"
	"fmt"
)

type Service struct {
	bookingRepository Repository
}

func NewService(repo Repository) *Service {
	return &Service{bookingRepository: repo}
}

func (s *Service) GetByID(ctx context.Context, id string) (*Booking, error) {
	booking, err := s.bookingRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by ID: %w", err)
	}
	return booking, nil
}

func (s *Service) GetByUserID(ctx context.Context, userID string) ([]*Booking, error) {
	booking, err := s.bookingRepository.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by user ID: %w", err)
	}
	return booking, nil
}

func (s *Service) Create(ctx context.Context, booking *Booking) (string, error) {
	booking.Status = StatusPending
	booking.PaymentStatus = PaymentPending

	bookingID, err := s.bookingRepository.Create(ctx, booking)
	if err != nil {
		return "", fmt.Errorf("failed to create booking: %w", err)
	}
	return bookingID, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id string, status Status) error {
	if !status.IsValid() {
		return errors.New("booking status invalid")
	}
	return s.bookingRepository.UpdateStatus(ctx, id, status)
}
