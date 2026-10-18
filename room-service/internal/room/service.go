package room

import (
	"context"
	"fmt"
	"time"
)

// inventoryMonths is how far ahead inventory is opened when a room is created
const inventoryMonths = 12

type Service struct {
	repository Repository
}

func NewService(r Repository) *Service {
	return &Service{repository: r}
}

func (s *Service) GetAll(ctx context.Context) ([]*Room, error) {
	rooms, err := s.repository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}
	return rooms, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*Room, error) {
	room, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}
	if room == nil {
		return nil, ErrNotFound
	}
	return room, nil
}

// Create room & inventory for next 1 year, every day priced at `rate`
func (s *Service) Create(ctx context.Context, room *Room, rate Rate) (*Room, error) {
	if !room.Type.IsValid() {
		return nil, fmt.Errorf("%w: room type %q", ErrInvalidInput, room.Type)
	}
	if !room.Status.IsValid() {
		return nil, fmt.Errorf("%w: room status %q", ErrInvalidInput, room.Status)
	}
	if !rate.IsValid() {
		return nil, fmt.Errorf("%w: rate must have price > 0 and a 3-letter uppercase currency", ErrInvalidInput)
	}

	inventories := NewInventories(time.Now(), inventoryMonths, rate)

	newRoom, err := s.repository.Create(ctx, room, inventories)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}
	return newRoom, nil
}

func (s *Service) Update(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.Update(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("failed to update room: %w", err)
	}
	return newRoom, nil
}

// TODO: txn
// in: SagaID, RoomID, CheckInDate, CheckOutDate, ExpiresAt (set by the gRPC server); Reserve fills the rest
func (s *Service) Reserve(ctx context.Context, r *Reservation) (*Reservation, *Room, error) {
	// Check room available
	room, err := s.repository.GetByID(ctx, r.RoomID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get room: %w", err)
	}
	if room == nil {
		return nil, nil, ErrNotFound
	}

	//Check existing reservation by saga ID
	reservation, err := s.repository.GetReservationBySagaID(ctx, r.SagaID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get reservation: %w", err)
	}
	if reservation != nil {
		if reservation.Status == Released {
			return nil, nil, ErrReservationReleased
		}
		return reservation, room, nil
	}

	//New reservation
	if room.Status != Active {
		return nil, nil, fmt.Errorf("room status %s: %w", room.Status, ErrRoomUnavailable)
	}
	now := time.Now().UTC()
	if !r.CheckInDate.Before(r.CheckOutDate) ||
		r.CheckInDate.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) {
		return nil, nil, fmt.Errorf("bad input date range: %w", ErrInvalidInput)
	}

	// Get inventories
	inventories, err := s.repository.GetInventories(ctx, r.RoomID, r.CheckInDate, r.CheckOutDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get inventories: %w", err)
	}

	// Reserve
	if err := r.setRates(inventories); err != nil {
		return nil, nil, err
	}
	r.Status = Reserved
	reservation, err = s.repository.CreateReservation(ctx, r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create reservation: %w", err)
	}

	return reservation, room, nil
}
