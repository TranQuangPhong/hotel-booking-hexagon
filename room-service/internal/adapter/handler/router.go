package handler

import (
	"net/http"

	logger "github.com/TranQuangPhong/hotel-booking-logger"
	"github.com/gin-gonic/gin"
)

func (h *RoomHandler) RoomRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.LoggingMiddleware())

	r.GET("/rooms/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/rooms/api/v1")
	{
		v1.GET("/", h.GetRooms)
		v1.GET("/:id", h.GetRoomByID)
		v1.POST("/", h.CreateRoom)
		v1.PUT("/:id", h.UpdateRoom)
		// v1.DELETE("/:id", h.DeleteRoom) //Optional
	}
	return r
}
