package handler

import (
	"booking/room-service/internal/room"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	roomSerivce *room.Service
}

func New(s *room.Service) *Handler {
	return &Handler{roomSerivce: s}
}

func (h *Handler) GetRooms(c *gin.Context) {
	rooms, err := h.roomSerivce.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, rooms)
}

func (h *Handler) GetRoomByID(c *gin.Context) {
	id := c.Param("id")
	room, err := h.roomSerivce.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, room)
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}

	newRoom, err := h.roomSerivce.Create(c.Request.Context(), req.ToRoom(), req.ToRate())
	if err != nil {
		if errors.Is(err, room.ErrInvalidInput) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}

	c.JSON(201, newRoom)
}

func (h *Handler) UpdateRoom(c *gin.Context) {
	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	id := c.Param("id")
	room := req.ToRoom(id)

	updatedRoom, err := h.roomSerivce.Update(c.Request.Context(), room)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, updatedRoom)
}
