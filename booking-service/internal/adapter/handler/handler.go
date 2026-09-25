package handler

import (
	"booking/booking-service/internal/booking"
	"fmt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	bookingService *booking.Service
}

func New(s *booking.Service) *Handler {
	return &Handler{bookingService: s}
}

func (h *Handler) GetBookings(c *gin.Context) {
	userID := c.Param("user_id") //TODO: Extract from JWT
	bookings, err := h.bookingService.GetDetailsByUserID(c, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, bookings)
}

func (h *Handler) GetBookingByID(c *gin.Context) {
	bookingID := c.Param("id")
	booking, err := h.bookingService.GetDetailByID(c, bookingID)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, booking)
}
