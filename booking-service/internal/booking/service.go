package booking

import (
	"context"
	"errors"
	"fmt"
)

type BookingService struct {
	bookingRepository BookingRespository
}

func NewBookingService(repo BookingRespository) *BookingService {
	return &BookingService{bookingRepository: repo}
}

func (s *BookingService) GetBookingDetailByID(ctx context.Context, id string) (*BookingDetail, error) {
	booking, err := s.bookingRepository.GetBookingDetailByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by ID: %w", err)
	}
	return booking, nil
}

func (s *BookingService) GetBookingDetailByUserID(ctx context.Context, userID string) ([]*BookingDetail, error) {
	booking, err := s.bookingRepository.GetBookingDetailByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get booking by user ID: %w", err)
	}
	return booking, nil
}

func (s *BookingService) CreateBooking(ctx context.Context, bookingDetail *BookingDetail) (string, error) {
	bookingDetail.Status = StatusPending
	bookingDetail.PaymentStatus = PaymentPending

	bookingID, err := s.bookingRepository.CreateBooking(ctx, bookingDetail)
	if err != nil {
		return "", fmt.Errorf("failed to create booking: %w", err)
	}
	return bookingID, nil
}

func (s *BookingService) UpdateBookingStatus(ctx context.Context, id string, status BookingStatus) error {
	if !status.IsValid() {
		return errors.New("booking status invalid")
	}
	return s.bookingRepository.UpdateBookingStatus(ctx, id, status)
}
