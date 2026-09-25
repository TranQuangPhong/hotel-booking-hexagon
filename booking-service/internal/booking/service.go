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

func (s *Service) GetDetailByID(ctx context.Context, id string) (*Detail, error) {
	booking, err := s.bookingRepository.GetDetailByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by ID: %w", err)
	}
	return booking, nil
}

func (s *Service) GetDetailsByUserID(ctx context.Context, userID string) ([]*Detail, error) {
	booking, err := s.bookingRepository.GetDetailsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by user ID: %w", err)
	}
	return booking, nil
}

func (s *Service) Create(ctx context.Context, bookingDetail *Detail) (string, error) {
	bookingDetail.Status = StatusPending
	bookingDetail.PaymentStatus = PaymentPending

	bookingID, err := s.bookingRepository.Create(ctx, bookingDetail)
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
