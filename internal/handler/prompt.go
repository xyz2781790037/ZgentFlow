package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
)

type PromptHandler struct {
	service *service.PromptService
}

func NewPromptHandler(service *service.PromptService) *PromptHandler {
	return &PromptHandler{service: service}
}

func (h *PromptHandler) List(c *gin.Context) {
	rows, err := h.service.List(c.Request.Context())
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *PromptHandler) History(c *gin.Context) {
	rows, err := h.service.History(c.Request.Context(), c.Param("category"), c.Param("template_id"))
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *PromptHandler) Update(c *gin.Context) {
	var request struct {
		Content    string `json:"content" binding:"required"`
		UserPrompt string `json:"user_prompt"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	row, err := h.service.Update(
		c.Request.Context(), c.Param("category"), c.Param("template_id"),
		request.Content, request.UserPrompt,
	)
	if err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}

func (h *PromptHandler) Rollback(c *gin.Context) {
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version < 1 {
		c.Error(errors.NewBadRequestError("invalid version"))
		return
	}
	row, err := h.service.Rollback(
		c.Request.Context(), c.Param("category"), c.Param("template_id"), version,
	)
	if err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": row})
}
