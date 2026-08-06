package handler

import (
	"booking/user-service/internal/user"
	"fmt"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	service *user.UserService
}

func NewUserHandler(s *user.UserService) *UserHandler {
	return &UserHandler{service: s}
}

func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.service.GetUsers(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to get users: %w", err).Error()})
	}
	c.JSON(200, users)
}

func (h *UserHandler) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	user, err := h.service.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Errorf("failed to get user: %w", err).Error()})
	}
	c.JSON(200, user)
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var req *CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	user := req.ToUser()
	err := h.service.CreateUser(c.Request.Context(), user)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to create user: %w", err).Error()})
		return
	}
	c.JSON(201, user)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	var req *UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	user := req.ToUser()
	err := h.service.UpdateUser(c.Request.Context(), user)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("failed to update user: %w", err).Error()})
		return
	}
	c.JSON(200, user)
}
