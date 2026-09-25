package room

import (
	"context"
	"fmt"
)

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

func (s *Service) Create(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.Create(ctx, room)
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
