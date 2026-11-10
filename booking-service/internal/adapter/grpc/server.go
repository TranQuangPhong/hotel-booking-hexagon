package grpc

import (
	"booking/booking-service/internal/booking"
	bookingv1 "booking/contracts/booking/v1"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	bookingv1.UnimplementedBookingServiceServer
	svc *booking.Service
}

func New(s *booking.Service) *Server {
	return &Server{svc: s}
}

func (s *Server) CreateBooking(ctx context.Context, req *bookingv1.CreateBookingRequest) (*bookingv1.CreateBookingResponse, error) {
	b, err := toBooking(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	stored, err := s.svc.Create(ctx, b)
	if err != nil {
		switch {
		case errors.Is(err, booking.ErrInvalidInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			// Includes ErrNotFound from the saga_id lookup after a skipped insert: the conflicting row is committed,
			// so a miss there is a bug, not the caller's fault (booking.proto lists only INVALID_ARGUMENT)
			slog.ErrorContext(ctx, "Create booking failed",
				slog.String("saga_id", req.GetSagaId()),
				slog.String("reservation_id", req.GetReservationId()),
				slog.String("user_id", req.GetUser().GetId()),
				slog.String("room_id", req.GetRoom().GetId()),
				slog.Any("error", err))
			return nil, status.Errorf(codes.Internal, "internal server error")
		}
	}
	return toResponse(stored), nil
}

func toBooking(req *bookingv1.CreateBookingRequest) (*booking.Booking, error) {
	sagaID, err := uuid.Parse(req.GetSagaId())
	if err != nil {
		return nil, fmt.Errorf("saga_id %q: %w", req.GetSagaId(), booking.ErrInvalidInput)
	}
	reservationID, err := uuid.Parse(req.GetReservationId())
	if err != nil {
		return nil, fmt.Errorf("reservation_id %q: %w", req.GetReservationId(), booking.ErrInvalidInput)
	}
	userID, err := uuid.Parse(req.GetUser().GetId())
	if err != nil {
		return nil, fmt.Errorf("user_id %q: %w", req.GetUser().GetId(), booking.ErrInvalidInput)
	}
	roomID, err := uuid.Parse(req.GetRoom().GetId())
	if err != nil {
		return nil, fmt.Errorf("room_id %q: %w", req.GetRoom().GetId(), booking.ErrInvalidInput)
	}

	checkIn, err := time.Parse(time.DateOnly, req.GetCheckInDate())
	if err != nil {
		return nil, fmt.Errorf("check_in_date %q: %w", req.GetCheckInDate(), booking.ErrInvalidInput)
	}
	checkOut, err := time.Parse(time.DateOnly, req.GetCheckOutDate())
	if err != nil {
		return nil, fmt.Errorf("check_out_date %q: %w", req.GetCheckOutDate(), booking.ErrInvalidInput)
	}

	rates := make([]booking.NightlyRate, 0, len(req.GetNightlyRates()))
	for _, rate := range req.GetNightlyRates() {
		d, err := time.Parse(time.DateOnly, rate.GetDate())
		if err != nil {
			return nil, fmt.Errorf("nightly_rate date %q: %w", rate.GetDate(), booking.ErrInvalidInput)
		}
		rates = append(rates, booking.NightlyRate{Date: d, Price: rate.GetPrice()})
	}
	if err := req.GetExpiresAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("expires_at: %w: %w", err, booking.ErrInvalidInput)
	}

	// Name, email, phone, room number / type, guests, total and currency are copied as sent: validate checks
	// them and names the field, never the value (personal data)
	return &booking.Booking{
		SagaID:          sagaID.String(),
		ReservationID:   reservationID.String(),
		UserID:          userID.String(),
		UserName:        req.GetUser().GetName(),
		UserEmail:       req.GetUser().GetEmail(),
		UserPhoneNumber: req.GetUser().GetPhoneNumber(),
		RoomID:          roomID.String(),
		RoomNumber:      req.GetRoom().GetNumber(),
		RoomType:        req.GetRoom().GetType(),
		CheckInDate:     checkIn,
		CheckOutDate:    checkOut,
		NumberOfGuests:  int(req.GetNumberOfGuests()),
		TotalAmount:     req.GetTotalAmount(),
		Currency:        req.GetCurrency(),
		NightlyRates:    rates,
		ExpiresAt:       req.GetExpiresAt().AsTime(),
	}, nil
}

// toResponse maps the stored booking: on a repeated call its status is the current one, not PENDING
func toResponse(b *booking.Booking) *bookingv1.CreateBookingResponse {
	return &bookingv1.CreateBookingResponse{
		BookingId: b.ID,
		Status:    string(b.Status),
	}
}
