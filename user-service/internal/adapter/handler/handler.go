package handler

import (
	"booking/user-service/internal/user"
	"fmt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *user.Service
}

func New(s *user.Service) *Handler {
	return &Handler{service: s}
}

func (h *Handler) GetUsers(c *gin.Context) {
	users, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, users)
}

func (h *Handler) GetUserByID(c *gin.Context) {
	id := c.Param("id")
	user, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, user)
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req *CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	user := req.ToUser()
	newUser, err := h.service.Create(c.Request.Context(), user)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(201, newUser)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var req *UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	id := c.Param("id")
	user := req.ToUser(id)

	updatedUser, err := h.service.Update(c.Request.Context(), user)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, updatedUser)
}
