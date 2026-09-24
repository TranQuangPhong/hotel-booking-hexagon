package handler

import (
	logger "github.com/TranQuangPhong/hotel-booking-logger"
	"github.com/gin-gonic/gin"
)

func (h *OrchestratorHandler) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.LoggingMiddleware())

	r.GET("/orchestrator/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/orchestrator/api/v1")
	{
		v1.POST("/bookings", h.CreateBooking)
		// v1.POST("/bookings/:id/modify", h.ModifyBooking)
		// v1.POST("/bookings/:id/cancel", h.CancelBooking)
		v1.POST("/bookings/:id/payment", h.StartPaymentTxn)
		// v1.POST("/bookings/:id/refund", h.Refund)
	}

	return r
}
