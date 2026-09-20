package handler

import (
	logger "github.com/TranQuangPhong/hotel-booking-logger"
	"github.com/gin-gonic/gin"
)

func (h *OrchestratorHandler) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.LoggingMiddleware())

	r.GET("/booking/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/booking")
	{
		v1.POST("/", h.CreateBooking)
		// v1.POST("/:id/modify", h.ModifyBooking)
		// v1.POST("/:id/cancel", h.CancelBooking)
		v1.POST("/:id/payment", h.StartPaymentTxn)
		// v1.POST("/:id/refund", h.Refund)
	}

	return r
}
