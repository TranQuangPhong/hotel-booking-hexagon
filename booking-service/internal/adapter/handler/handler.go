package handler

import (
	"booking/booking-service/internal/booking"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	bookingService *booking.Service
}

func New(s *booking.Service) *Handler {
	return &Handler{bookingService: s}
}

func (h *Handler) GetBookingByID(c *gin.Context) {
	id, err := parseBookingID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// c.Request.Context(), not c: gin.Context doesn't carry the request's trace_id or its cancel
	b, err := h.bookingService.GetByID(c.Request.Context(), id)
	if err != nil {
		writeError(c, "GetBookingByID failed", err)
		return
	}
	c.JSON(200, toBookingResponse(b))
}

// parseBookingID checks the id is a UUID, so a malformed one is a 400, not a Postgres cast error (500)
func parseBookingID(c *gin.Context) (string, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return "", fmt.Errorf("booking id %q: %w", c.Param("id"), booking.ErrInvalidInput)
	}
	return id.String(), nil
}

// writeError answers a service error: 400 ErrInvalidInput, 404 ErrNotFound. Anything else is a 500, like
// INTERNAL on gRPC: the cause is logged here (the logger adds the request's trace_id) and the client gets a
// generic message, since the error text can carry SQL details
func writeError(c *gin.Context, msg string, err error) {
	switch {
	case errors.Is(err, booking.ErrInvalidInput):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, booking.ErrNotFound):
		c.JSON(404, gin.H{"error": err.Error()})
	default:
		slog.ErrorContext(c.Request.Context(), msg, slog.Any("error", err))
		c.JSON(500, gin.H{"error": "internal server error"})
	}
}
