package handler

import (
	"booking/room-service/internal/room"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		writeError(c, "GetRooms failed", err)
		return
	}
	c.JSON(200, rooms)
}

func (h *Handler) GetRoomByID(c *gin.Context) {
	id, err := parseRoomID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	rm, err := h.roomSerivce.GetByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, "GetRoomByID failed", err)
		return
	}
	c.JSON(200, rm)
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}

	newRoom, err := h.roomSerivce.Create(c.Request.Context(), req.ToRoom(), req.ToRate())
	if err != nil {
		writeError(c, "CreateRoom failed", err)
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
	id, err := parseRoomID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	rm := req.ToRoom(id)

	updatedRoom, err := h.roomSerivce.Update(c.Request.Context(), rm)
	if err != nil {
		writeError(c, "UpdateRoom failed", err)
		return
	}
	c.JSON(200, updatedRoom)
}

// UpdateInventories edits the status / price of the existing days from first_night to last_night (both inclusive)
func (h *Handler) UpdateInventories(c *gin.Context) {
	var req UpdateInventoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	id, err := parseRoomID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	firstNight, lastNight, err := req.ToNights()
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	inventories, err := h.roomSerivce.UpdateInventory(c.Request.Context(), id, req.ToDayPatch(), firstNight, lastNight)
	if err != nil {
		writeError(c, "UpdateInventories failed", err)
		return
	}
	c.JSON(200, inventories)
}

// parseRoomID returns the :id path param in canonical UUID form. A malformed id is bad input (400, like INVALID_ARGUMENT
// on gRPC): sent as is, Postgres rejects it with an error that isn't ErrNotFound
func parseRoomID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", fmt.Errorf("room id %q: %w", c.Param("id"), room.ErrInvalidInput)
	}
	return id.String(), nil
}

// writeError answers a service error: 400 ErrInvalidInput, 404 ErrNotFound, 409 ErrReserved / ErrNumberTaken.
// Anything else is a 500, like INTERNAL on gRPC: the cause is logged here (the logger adds the request's trace_id)
// and the client gets a generic message, since the error text can carry SQL details
func writeError(c *gin.Context, msg string, err error) {
	switch {
	case errors.Is(err, room.ErrInvalidInput):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, room.ErrNotFound):
		c.JSON(404, gin.H{"error": err.Error()})
	case errors.Is(err, room.ErrReserved), errors.Is(err, room.ErrNumberTaken):
		c.JSON(409, gin.H{"error": err.Error()})
	default:
		slog.ErrorContext(c.Request.Context(), msg, slog.Any("error", err))
		c.JSON(500, gin.H{"error": "internal server error"})
	}
}
