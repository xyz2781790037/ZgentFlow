package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

type KnowledgeBaseShareHandler struct {
	service *service.KnowledgeBaseShareService
}

func NewKnowledgeBaseShareHandler(service *service.KnowledgeBaseShareService) *KnowledgeBaseShareHandler {
	return &KnowledgeBaseShareHandler{service: service}
}

func shareError(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		c.Error(appErr)
		return
	}
	c.Error(apperrors.NewInternalServerError(err.Error()))
}

func (h *KnowledgeBaseShareHandler) GetSettings(c *gin.Context) {
	kb, access, err := h.service.ResolveAccess(c.Request.Context(), c.Param("id"), types.KBRoleReader)
	if err != nil {
		shareError(c, err)
		return
	}
	data := gin.H{
		"knowledge_base_id": kb.ID, "sharing_enabled": kb.SharingEnabled,
		"access_role": access.Role, "owner_username": access.OwnerUsername,
	}
	if access.Role == types.KBRoleOwner {
		data["invite_code"] = kb.InviteCode
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *KnowledgeBaseShareHandler) UpdateInvitation(c *gin.Context) {
	var req struct {
		Enabled    bool `json:"enabled"`
		Regenerate bool `json:"regenerate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数无效"))
		return
	}
	kb, err := h.service.UpdateInvitation(c.Request.Context(), c.Param("id"), req.Enabled, req.Regenerate)
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"sharing_enabled": kb.SharingEnabled, "invite_code": kb.InviteCode}})
}

func (h *KnowledgeBaseShareHandler) LookupInvitation(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数无效"))
		return
	}
	data, err := h.service.LookupInvitation(c.Request.Context(), req.Code)
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *KnowledgeBaseShareHandler) SubmitJoinRequest(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数无效"))
		return
	}
	row, err := h.service.SubmitJoinRequest(c.Request.Context(), strings.TrimSpace(req.Code))
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": row})
}

func (h *KnowledgeBaseShareHandler) ListMyJoinRequests(c *gin.Context) {
	rows, err := h.service.ListMyJoinRequests(c.Request.Context())
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *KnowledgeBaseShareHandler) ListJoinRequests(c *gin.Context) {
	rows, err := h.service.ListJoinRequests(c.Request.Context(), c.Param("id"))
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *KnowledgeBaseShareHandler) ReviewJoinRequest(c *gin.Context) {
	var req struct {
		Decision string `json:"decision"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数无效"))
		return
	}
	if err := h.service.ReviewJoinRequest(c.Request.Context(), c.Param("id"), c.Param("request_id"), req.Decision); err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeBaseShareHandler) ListMembers(c *gin.Context) {
	rows, err := h.service.ListMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *KnowledgeBaseShareHandler) UpdateMemberRole(c *gin.Context) {
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("请求参数无效"))
		return
	}
	if err := h.service.UpdateMemberRole(c.Request.Context(), c.Param("id"), c.Param("user_id"), req.Role); err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeBaseShareHandler) RemoveMember(c *gin.Context) {
	if err := h.service.RemoveMember(c.Request.Context(), c.Param("id"), c.Param("user_id")); err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeBaseShareHandler) Leave(c *gin.Context) {
	if err := h.service.Leave(c.Request.Context(), c.Param("id")); err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeBaseShareHandler) ListAuditLogs(c *gin.Context) {
	rows, err := h.service.ListAuditLogs(c.Request.Context(), c.Param("id"))
	if err != nil {
		shareError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}
