package handler

import "github.com/gin-gonic/gin"

type Handler struct{}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) CreateBooking(c *gin.Context) {}

func (h *Handler) StartPaymentTxn(c *gin.Context) {}
