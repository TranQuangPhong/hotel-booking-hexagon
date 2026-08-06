package handler

import (
	"github.com/gin-gonic/gin"
)

func (h *UserHandler) UserRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	// r.User(LoggingMidleware()) //TODO
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
