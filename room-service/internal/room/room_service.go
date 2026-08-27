package room

import (
	"context"
	"fmt"
)

type RoomService struct {
	repository Repository
}

func NewRoomService(r Repository) *RoomService {
	return &RoomService{repository: r}
}

func (s *RoomService) GetRooms(ctx context.Context) ([]*Room, error) {
	rooms, err := s.repository.GetRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}
	return rooms, nil
}

func (s *RoomService) GetRoomByID(ctx context.Context, id string) (*Room, error) {
	room, err := s.repository.GetRoomByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get room: %w", err)
	}
	return room, nil
}

func (s *RoomService) CreateRoom(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.CreateRoom(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("failed to create room: %w", err)
	}
	return newRoom, nil
}

func (s *RoomService) UpdateRoom(ctx context.Context, room *Room) (*Room, error) {
	newRoom, err := s.repository.UpdateRoom(ctx, room)
	if err != nil {
		return nil, fmt.Errorf("failed to update room: %w", err)
	}
	return newRoom, nil
}
