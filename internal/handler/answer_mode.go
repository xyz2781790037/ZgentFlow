package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// AnswerModeHandler exposes the two built-in answer modes.
type AnswerModeHandler struct {
	service interfaces.AnswerModeService
}

func NewAnswerModeHandler(service interfaces.AnswerModeService) *AnswerModeHandler {
	return &AnswerModeHandler{service: service}
}

func (h *AnswerModeHandler) ListAnswerModes(c *gin.Context) {
	modes, err := h.service.ListAnswerModes(c.Request.Context())
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": modes})
}
