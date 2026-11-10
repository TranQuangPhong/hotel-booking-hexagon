package handler

import (
	"net/http"

	logger "github.com/TranQuangPhong/hotel-booking-logger"
	"github.com/gin-gonic/gin"
)

func (h *Handler) BookingRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.LoggingMiddleware())

	r.GET("/bookings/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	v1 := r.Group("/bookings/api/v1")
	{
		// CreateBooking is gRPC (orchestrator). The list route comes back with UC2 / the FE
		v1.GET("/:id", h.GetBookingByID)
	}

	return r
}
