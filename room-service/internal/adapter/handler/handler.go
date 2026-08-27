package handler

import (
	"booking/room-service/internal/inventory"
	"booking/room-service/internal/room"
	"fmt"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	roomSerivce      *room.RoomService
	inventoryService *inventory.Service
}

func NewRoomHandler(rs *room.RoomService, is *inventory.Service) *RoomHandler {
	return &RoomHandler{roomSerivce: rs, inventoryService: is}
}

func (h *RoomHandler) GetRooms(c *gin.Context) {
	rooms, err := h.roomSerivce.GetRooms(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
	}
	c.JSON(200, rooms)
}

func (h *RoomHandler) GetRoomByID(c *gin.Context) {
	id := c.Param("id")
	room, err := h.roomSerivce.GetRoomByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(404, gin.H{"error": fmt.Errorf("%w", err).Error()})
	}
	c.JSON(200, room)
}

func (h *RoomHandler) CreateRoom(c *gin.Context) {
	var req *CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	room := req.ToRoom()
	newRoom, err := h.roomSerivce.CreateRoom(c.Request.Context(), room)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, newRoom)
}

func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}
	id := c.Param("id")
	room := req.ToRoom(id)

	updatedRoom, err := h.roomSerivce.UpdateRoom(c.Request.Context(), room)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Errorf("%w", err).Error()})
		return
	}
	c.JSON(200, updatedRoom)
}
