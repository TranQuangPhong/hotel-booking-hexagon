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
		v1.GET("/", h.GetBookings)
		v1.GET("/:id", h.GetBookingByID)
		// v1.POST("/:id", h.CreateBooking)
		// v1.POST("/:id/modify", h.ModifyBooking)
		// v1.POST("/:id/cancel", h.CancelBooking)
	}

	return r
}
