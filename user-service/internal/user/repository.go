package user

import "context"

//All repository port interfaces in hexagonal architecture

type UserRepository interface {
	GetUsers(ctx context.Context) ([]*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	CreateUser(ctx context.Context, user *User) (*User, error)
	UpdateUser(ctx context.Context, user *User) (*User, error)
}
