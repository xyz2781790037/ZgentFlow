package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	appErrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

const (
	authSessionTokenKey = "auth.session_token"
	authCSRFTokenKey    = "auth.csrf_token"
)

var localWorkspaceInitMu sync.Mutex

// LocalWorkspace bootstraps the shared workspace actor. Interactive requests
// use RequireSession; registration reuses this initializer for the workspace.
func LocalWorkspace(
	userService interfaces.UserService,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, tenant, err := resolveLocalWorkspace(c.Request.Context(), userService)
		if err != nil {
			logger.Errorf(c.Request.Context(), "resolve local workspace failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   gin.H{"message": "本地工作区初始化失败"},
			})
			return
		}

		c.Set(types.TenantIDContextKey.String(), tenant.ID)
		c.Set(types.TenantInfoContextKey.String(), tenant)
		c.Set(types.UserContextKey.String(), user)
		c.Set(types.UserIDContextKey.String(), user.ID)

		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant.ID)
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
		ctx = context.WithValue(ctx, types.UserContextKey, user)
		ctx = context.WithValue(ctx, types.UserIDContextKey, user.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequireSession resolves the opaque session cookie and injects the same
// user/workspace context expected by the existing business handlers. Unsafe
// requests additionally require the per-session CSRF token.
func RequireSession(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(authService.CookieName())
		if err != nil || token == "" {
			c.Error(appErrors.NewUnauthorizedError("请先登录"))
			c.Abort()
			return
		}
		user, tenant, csrfToken, err := authService.AuthenticateSession(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, service.ErrAuthUnavailable) {
				logger.Error(c.Request.Context(), "load authentication session failed", err)
				c.Error(appErrors.NewServiceUnavailableError("认证服务暂时不可用"))
			} else {
				c.Error(appErrors.NewUnauthorizedError("登录状态已失效，请重新登录"))
			}
			c.Abort()
			return
		}
		if requiresCSRF(c.Request.Method) {
			supplied := c.GetHeader("X-CSRF-Token")
			if len(supplied) != len(csrfToken) || subtle.ConstantTimeCompare([]byte(supplied), []byte(csrfToken)) != 1 {
				c.Error(appErrors.NewForbiddenError("请求安全校验失败，请刷新页面后重试"))
				c.Abort()
				return
			}
		}

		c.Set(authSessionTokenKey, token)
		c.Set(authCSRFTokenKey, csrfToken)
		attachWorkspace(c, user, tenant)
		c.Next()
	}
}

func attachWorkspace(c *gin.Context, user *types.User, tenant *types.Tenant) {
	c.Set(types.TenantIDContextKey.String(), tenant.ID)
	c.Set(types.TenantInfoContextKey.String(), tenant)
	c.Set(types.UserContextKey.String(), user)
	c.Set(types.UserIDContextKey.String(), user.ID)

	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenant.ID)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserContextKey, user)
	ctx = context.WithValue(ctx, types.UserIDContextKey, user.ID)
	c.Request = c.Request.WithContext(ctx)
}

func AuthSessionToken(c *gin.Context) string {
	value, _ := c.Get(authSessionTokenKey)
	token, _ := value.(string)
	return token
}

func AuthCSRFToken(c *gin.Context) string {
	value, _ := c.Get(authCSRFTokenKey)
	token, _ := value.(string)
	return token
}

func requiresCSRF(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func resolveLocalWorkspace(
	ctx context.Context,
	userService interfaces.UserService,
) (*types.User, *types.Tenant, error) {
	localWorkspaceInitMu.Lock()
	defer localWorkspaceInitMu.Unlock()

	return userService.ResolveLocalWorkspace(ctx)
}

// GetTenantIDFromContext returns the internal workspace partition ID.
func GetTenantIDFromContext(ctx context.Context) (uint64, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return 0, fmt.Errorf("workspace ID not found in context")
	}
	return tenantID, nil
}
