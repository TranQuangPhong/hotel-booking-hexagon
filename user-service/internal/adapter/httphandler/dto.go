package handler

import "booking/user-service/internal/user"

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (req *CreateUserRequest) ToUser() *user.User {
	return &user.User{
		Name:  req.Name,
		Email: req.Email,
		Role:  user.Role(req.Role),
	}
}

type UpdateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	Role  string `json:"role" binding:"required"`
}

func (req *UpdateUserRequest) ToUser() *user.User {
	return &user.User{
		Name:  req.Name,
		Email: req.Email,
		Role:  user.Role(req.Role),
	}
}
