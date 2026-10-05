package room

import (
	"booking/room-service/internal/inventory"
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
	return room, nil
}

// Create room & inventory for next 1 year, every day priced at `rate`
func (s *Service) Create(ctx context.Context, room *Room, rate inventory.Rate) (*Room, error) {
	if !room.Type.IsValid() {
		return nil, fmt.Errorf("%w: room type %q", ErrInvalidInput, room.Type)
	}
	if !room.Status.IsValid() {
		return nil, fmt.Errorf("%w: room status %q", ErrInvalidInput, room.Status)
	}
	if !rate.IsValid() {
		return nil, fmt.Errorf("%w: rate must have price > 0 and a 3-letter uppercase currency", ErrInvalidInput)
	}

	inventories := inventory.NewMonths(time.Now(), inventoryMonths, rate)

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
