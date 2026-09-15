package handler

import (
	"booking/booking-service/internal/booking"
	"fmt"

	"github.com/gin-gonic/gin"
)

type BookingHandler struct {
	bookingService *booking.BookingService
}

func NewBookingHandler(s *booking.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: s}
}

func (h *BookingHandler) GetBookings(c *gin.Context) {
	userID := c.Param("user_id") //TODO: Extract from JWT
	bookings, err := h.bookingService.GetBookingDetailByUserID(c, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, bookings)
}

func (h *BookingHandler) GetBookingByID(c *gin.Context) {
	bookingID := c.Param("id")
	booking, err := h.bookingService.GetBookingDetailByID(c, bookingID)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, booking)
}
