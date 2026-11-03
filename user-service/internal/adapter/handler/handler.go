package handler

import (
	"booking/user-service/internal/user"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		writeError(c, "GetUsers failed", err)
		return
	}
	c.JSON(200, users)
}

func (h *Handler) GetUserByID(c *gin.Context) {
	id, err := parseUserID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	u, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, "GetUserByID failed", err)
		return
	}
	c.JSON(200, u)
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	newUser, err := h.service.Create(c.Request.Context(), req.ToUser())
	if err != nil {
		writeError(c, "CreateUser failed", err)
		return
	}
	c.JSON(201, newUser)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	id, err := parseUserID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	updatedUser, err := h.service.Update(c.Request.Context(), req.ToUser(id))
	if err != nil {
		writeError(c, "UpdateUser failed", err)
		return
	}
	c.JSON(200, updatedUser)
}

// parseUserID returns the :id path param in canonical UUID form. A malformed id is bad input (400):
// sent as is, Postgres rejects it with an error that isn't ErrNotFound
func parseUserID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", fmt.Errorf("user id %q: %w", c.Param("id"), user.ErrInvalidInput)
	}
	return id.String(), nil
}

// writeError answers a service error: 400 ErrInvalidInput, 404 ErrNotFound, 409 ErrEmailTaken.
// Anything else is a 500: the cause is logged here (the logger adds the request's trace_id)
// and the client gets a generic message, since the error text can carry SQL details
func writeError(c *gin.Context, msg string, err error) {
	switch {
	case errors.Is(err, user.ErrInvalidInput):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, user.ErrNotFound):
		c.JSON(404, gin.H{"error": err.Error()})
	case errors.Is(err, user.ErrEmailTaken):
		c.JSON(409, gin.H{"error": err.Error()})
	default:
		slog.ErrorContext(c.Request.Context(), msg, slog.Any("error", err))
		c.JSON(500, gin.H{"error": "internal server error"})
	}
}
