package user

import (
	"context"
	"fmt"
)

type UserService struct {
	userRepository UserRepository
}

func NewUserService(r UserRepository) *UserService {
	return &UserService{userRepository: r}
}

func (s *UserService) GetUsers(ctx context.Context) ([]*User, error) {
	users, err := s.userRepository.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

func (s *UserService) GetUserByID(ctx context.Context, id string) (*User, error) {
	user, err := s.userRepository.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	err := s.userRepository.CreateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	err := s.userRepository.UpdateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}
