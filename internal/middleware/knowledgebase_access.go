package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func abortKBAccess(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		c.Error(appErr)
	} else {
		c.Error(apperrors.NewInternalServerError(err.Error()))
	}
	c.Abort()
}

// KnowledgeBaseJSONBodyAccess resolves one or more KB IDs from a JSON body,
// restores the body for the handler, and requires every KB to share the same
// storage tenant. It is used by batch delete/move endpoints.
func KnowledgeBaseJSONBodyAccess(share *service.KnowledgeBaseShareService, requiredRole string, fields ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			abortKBAccess(c, apperrors.NewBadRequestError("请求参数无效"))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			abortKBAccess(c, apperrors.NewBadRequestError("请求参数无效"))
			return
		}
		var firstKB *types.KnowledgeBase
		var firstAccess *types.KnowledgeBaseAccess
		for _, field := range fields {
			kbID, _ := payload[field].(string)
			if kbID == "" {
				abortKBAccess(c, apperrors.NewBadRequestError("缺少知识库参数"))
				return
			}
			kb, access, err := share.ResolveAccess(c.Request.Context(), kbID, requiredRole)
			if err != nil {
				abortKBAccess(c, err)
				return
			}
			if firstKB == nil {
				firstKB, firstAccess = kb, access
			} else if firstKB.TenantID != kb.TenantID {
				abortKBAccess(c, apperrors.NewBadRequestError("不能跨所有者移动文档"))
				return
			}
		}
		attachKBStorageContext(c, share, firstKB, firstAccess)
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Next()
	}
}

func attachKBStorageContext(c *gin.Context, share *service.KnowledgeBaseShareService, kb *types.KnowledgeBase, access *types.KnowledgeBaseAccess) {
	ctx := share.WithStorageTenant(c.Request.Context(), kb, access.Role)
	c.Request = c.Request.WithContext(ctx)
	c.Set(types.TenantIDContextKey.String(), kb.TenantID)
	c.Set(types.KnowledgeBaseAccessRoleContextKey.String(), access.Role)
	c.Set("ResolvedKnowledgeBaseID", kb.ID)
	if tenant, ok := types.TenantInfoFromContext(ctx); ok {
		c.Set(types.TenantInfoContextKey.String(), tenant)
	}
}

// KnowledgeBaseAccess resolves a :param knowledge-base ID and switches the
// downstream repository context to that KB's immutable storage tenant.
func KnowledgeBaseAccess(share *service.KnowledgeBaseShareService, requiredRole, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		kb, access, err := share.ResolveAccess(c.Request.Context(), c.Param(param), requiredRole)
		if err != nil {
			abortKBAccess(c, err)
			return
		}
		attachKBStorageContext(c, share, kb, access)
		c.Next()
	}
}

func KnowledgeAccess(share *service.KnowledgeBaseShareService, requiredRole, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		kb, access, _, err := share.ResolveKnowledgeAccess(c.Request.Context(), c.Param(param), requiredRole)
		if err != nil {
			abortKBAccess(c, err)
			return
		}
		attachKBStorageContext(c, share, kb, access)
		c.Next()
	}
}

// KnowledgeReparseAccess lets Admin+ retry any document and lets a Writer retry
// only a document they uploaded themselves after a failed/cancelled parse.
func KnowledgeReparseAccess(share *service.KnowledgeBaseShareService, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		kb, access, knowledge, err := share.ResolveKnowledgeAccess(c.Request.Context(), c.Param(param), types.KBRoleWriter)
		if err != nil {
			abortKBAccess(c, err)
			return
		}
		if types.KBRoleRank(access.Role) < types.KBRoleRank(types.KBRoleAdmin) {
			userID, _ := types.UserIDFromContext(c.Request.Context())
			if knowledge.UploadedByUserID != userID || (knowledge.ParseStatus != types.ParseStatusFailed && knowledge.ParseStatus != types.ParseStatusCancelled) {
				abortKBAccess(c, apperrors.NewForbiddenError("写入用户只能重试自己上传的失败文档"))
				return
			}
		}
		attachKBStorageContext(c, share, kb, access)
		c.Next()
	}
}

func ChunkAccess(share *service.KnowledgeBaseShareService, requiredRole, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		kb, access, err := share.ResolveChunkAccess(c.Request.Context(), c.Param(param), requiredRole)
		if err != nil {
			abortKBAccess(c, err)
			return
		}
		attachKBStorageContext(c, share, kb, access)
		c.Next()
	}
}

// AuditKBMutation appends a log only after a successful handler response.
func AuditKBMutation(share *service.KnowledgeBaseShareService, action, kbParam, resourceParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.IsAborted() || len(c.Errors) > 0 || c.Writer.Status() >= http.StatusBadRequest {
			return
		}
		kbID := c.Param(kbParam)
		if kbID == "" {
			kbID = c.GetString("ResolvedKnowledgeBaseID")
		}
		resourceID := ""
		if resourceParam != "" {
			resourceID = c.Param(resourceParam)
		}
		_ = share.Audit(c.Request.Context(), kbID, action, "", resourceID, nil)
	}
}
