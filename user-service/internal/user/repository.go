package user

import "context"

//All repository port interfaces in hexagonal architecture

type Repository interface {
	GetAll(ctx context.Context) ([]*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, user *User) (*User, error)
	Update(ctx context.Context, user *User) (*User, error)
}
