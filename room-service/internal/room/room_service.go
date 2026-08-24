package room

import (
	"context"
	"fmt"
)

type Service struct {
	repository Repository
}

func NewRoomService(r Repository) *Service {
	return &Service{repository: r}
}

func (s *Service) GetRooms(ctx context.Context) ([]*Room, error) {
	rooms, err := s.repository.GetRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}
	return rooms, nil
}

func (s *Service) GetRoomByID(ctx context.Context, id string) (*Room, error) {
	room, err := s.repository.GetRoomByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}
	return room, nil
}

func (s *Service) CreateRoom(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.CreateRoom(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}
	return newRoom, nil
}

func (s *Service) UpdateRoom(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.UpdateRoom(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("failed to update room: %w", err)
	}
	return newRoom, nil
}
