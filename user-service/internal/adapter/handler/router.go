package handler

import (
	"net/http"

	logger "github.com/TranQuangPhong/hotel-booking-logger"

	"github.com/gin-gonic/gin"
)

func (h *UserHandler) UserRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(logger.LoggingMiddleware())

	r.GET("/users/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/users/api/v1")
	{
		v1.GET("/", h.GetUsers)
		v1.GET("/:id", h.GetUserByID)
		v1.POST("/", h.CreateUser)
		v1.PUT("/:id", h.UpdateUser)
		// v1.DELETE("/:id", h.DeleteUser) //Optional
	}
	return r
}
