package grpc

import (
	roomv1 "booking/contracts/room/v1"
	"booking/room-service/internal/room"
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
	roomv1.UnimplementedRoomServiceServer
	svc *room.Service
}

func New(s *room.Service) *Server {
	return &Server{svc: s}
}

func (s *Server) ReserveRoom(ctx context.Context, req *roomv1.ReserveRoomRequest) (*roomv1.ReserveRoomResponse, error) {
	reservation, err := toReservation(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	reservation, rm, err := s.svc.Reserve(ctx, reservation)
	if err != nil {
		switch {
		case errors.Is(err, room.ErrInvalidInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, room.ErrNotFound):
			return nil, status.Errorf(codes.NotFound, "room not found")
		case errors.Is(err, room.ErrRoomUnavailable):
			return nil, status.Errorf(codes.FailedPrecondition, "ROOM_UNAVAILABLE")
		case errors.Is(err, room.ErrReservationReleased):
			return nil, status.Errorf(codes.FailedPrecondition, "RESERVATION_RELEASED")
		default:
			slog.ErrorContext(ctx, "Reserve failed",
				slog.String("saga_id", req.GetSagaId()),
				slog.String("room_id", req.GetRoomId()),
				slog.String("check_in_date", req.GetCheckInDate()),
				slog.String("check_out_date", req.GetCheckOutDate()),
				slog.Time("expires_at", req.GetExpiresAt().AsTime()),
				slog.Any("error", err))
			return nil, status.Errorf(codes.Internal, "internal server error")
		}
	}

	res := toResponse(reservation, rm)
	return res, nil
}

func toReservation(req *roomv1.ReserveRoomRequest) (*room.Reservation, error) {
	sagaID, err := uuid.Parse(req.GetSagaId())
	if err != nil {
		return nil, fmt.Errorf("saga_id %q: %w", req.GetSagaId(), room.ErrInvalidInput)
	}
	roomID, err := uuid.Parse(req.GetRoomId())
	if err != nil {
		return nil, fmt.Errorf("room_id %q: %w", req.GetRoomId(), room.ErrInvalidInput)
	}
	checkIn, err := time.Parse(time.DateOnly, req.GetCheckInDate())
	if err != nil {
		return nil, fmt.Errorf("check_in_date %q: %w", req.GetCheckInDate(), room.ErrInvalidInput)
	}
	checkOut, err := time.Parse(time.DateOnly, req.GetCheckOutDate())
	if err != nil {
		return nil, fmt.Errorf("check_out_date %q: %w", req.GetCheckOutDate(), room.ErrInvalidInput)
	}
	if err := req.GetExpiresAt().CheckValid(); err != nil {
		return nil, fmt.Errorf("expires_at: %w: %w", err, room.ErrInvalidInput)
	}

	return &room.Reservation{
		SagaID:       sagaID.String(),
		RoomID:       roomID.String(),
		CheckInDate:  checkIn,
		CheckOutDate: checkOut,
		ExpiresAt:    req.GetExpiresAt().AsTime(),
	}, nil
}

func toResponse(reservation *room.Reservation, rm *room.Room) *roomv1.ReserveRoomResponse {
	rates := make([]*roomv1.NightlyRate, 0, len(reservation.NightlyRates))
	for _, rate := range reservation.NightlyRates {
		rates = append(rates, &roomv1.NightlyRate{Date: rate.Date.Format(time.DateOnly), Price: rate.Price})
	}
	return &roomv1.ReserveRoomResponse{
		ReservationId: reservation.ID,
		RoomNumber:    rm.Number,
		RoomType:      string(rm.Type),
		Currency:      reservation.Currency,
		TotalAmount:   reservation.TotalAmount,
		NightlyRates:  rates,
	}
}
