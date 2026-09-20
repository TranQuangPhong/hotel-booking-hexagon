package handler

import "github.com/gin-gonic/gin"

type OrchestratorHandler struct{}

func NewOrchestratorHandler() *OrchestratorHandler {
	return &OrchestratorHandler{}
}

func (h *OrchestratorHandler) CreateBooking(c *gin.Context) {}

func (h *OrchestratorHandler) StartPaymentTxn(c *gin.Context) {}
