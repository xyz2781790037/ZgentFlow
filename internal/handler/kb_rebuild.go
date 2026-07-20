package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
)

// StartKBFullRebuild starts one indivisible vectors + optional Wiki rebuild.
func (h *KnowledgeBaseHandler) StartKBFullRebuild(c *gin.Context) {
	if h.rebuildService == nil {
		c.Error(apperrors.NewInternalServerError("full rebuild service is unavailable"))
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	state, err := h.rebuildService.Start(c.Request.Context(), id)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": state})
}

// GetKBFullRebuildState returns the last rebuild status and failure reason.
func (h *KnowledgeBaseHandler) GetKBFullRebuildState(c *gin.Context) {
	if h.rebuildService == nil {
		c.Error(apperrors.NewInternalServerError("full rebuild service is unavailable"))
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	state, err := h.rebuildService.GetState(c.Request.Context(), id)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": state})
}
