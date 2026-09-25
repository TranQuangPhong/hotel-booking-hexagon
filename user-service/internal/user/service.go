package user

import (
	"context"
	"fmt"
)

type Service struct {
	userRepository Repository
}

func NewService(r Repository) *Service {
	return &Service{userRepository: r}
}

func (s *Service) GetAll(ctx context.Context) ([]*User, error) {
	users, err := s.userRepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*User, error) {
	user, err := s.userRepository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *Service) Create(ctx context.Context, user *User) (*User, error) {
	newUser, err := s.userRepository.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return newUser, nil
}

func (s *Service) Update(ctx context.Context, user *User) (*User, error) {
	newUser, err := s.userRepository.Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}
	return newUser, nil
}
