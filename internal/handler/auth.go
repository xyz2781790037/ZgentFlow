package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	appErrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/middleware"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{service: authService}
}

type sendVerificationCodeRequest struct {
	Email   string `json:"email" binding:"required"`
	Purpose string `json:"purpose" binding:"required"`
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

type passwordLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type emailCodeLoginRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func (h *AuthHandler) SendVerificationCode(c *gin.Context) {
	var request sendVerificationCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewValidationError("请输入有效的邮箱和验证码用途"))
		return
	}
	if err := h.service.SendVerificationCode(c.Request.Context(), request.Email, request.Purpose, c.ClientIP()); err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "如果邮箱状态符合要求，验证码已发送，请注意查收",
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewValidationError("请完整填写用户名、邮箱、密码和验证码"))
		return
	}
	result, err := h.service.Register(c.Request.Context(), request.Username, request.Email, request.Password, request.Code)
	if err != nil {
		h.handleError(c, err)
		return
	}
	h.completeLogin(c, result, "注册成功")
}

func (h *AuthHandler) LoginWithPassword(c *gin.Context) {
	var request passwordLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewValidationError("请输入用户名和密码"))
		return
	}
	result, err := h.service.LoginWithPassword(c.Request.Context(), request.Username, request.Password, c.ClientIP())
	if err != nil {
		h.handleError(c, err)
		return
	}
	h.completeLogin(c, result, "登录成功")
}

func (h *AuthHandler) LoginWithEmailCode(c *gin.Context) {
	var request emailCodeLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(appErrors.NewValidationError("请输入邮箱和验证码"))
		return
	}
	result, err := h.service.LoginWithEmailCode(c.Request.Context(), request.Email, request.Code, c.ClientIP())
	if err != nil {
		h.handleError(c, err)
		return
	}
	h.completeLogin(c, result, "登录成功")
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := types.UserFromContext(c.Request.Context())
	if !ok || user == nil {
		c.Error(appErrors.NewUnauthorizedError("登录状态已失效，请重新登录"))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user":       user,
			"csrf_token": middleware.AuthCSRFToken(c),
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.service.RevokeSession(c.Request.Context(), middleware.AuthSessionToken(c)); err != nil {
		h.handleError(c, err)
		return
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "已退出登录"})
}

func (h *AuthHandler) completeLogin(c *gin.Context, result *service.AuthResult, message string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		h.service.CookieName(),
		result.SessionToken,
		int(h.service.SessionTTL().Seconds()),
		"/",
		"",
		h.service.CookieSecure(),
		true,
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data": gin.H{
			"user":       result.User,
			"csrf_token": result.CSRFToken,
		},
	})
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.service.CookieName(), "", -1, "/", "", h.service.CookieSecure(), true)
}

func (h *AuthHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAuthInvalidInput):
		c.Error(appErrors.NewValidationError("用户名需为 3-32 位英文字母或数字，密码不能为空"))
	case errors.Is(err, service.ErrAuthInvalidCredentials):
		c.Error(appErrors.NewUnauthorizedError("用户名或密码错误"))
	case errors.Is(err, service.ErrAuthInvalidCode):
		c.Error(appErrors.NewUnauthorizedError("验证码错误或已过期"))
	case errors.Is(err, service.ErrAuthUserExists):
		c.Error(appErrors.NewConflictError("用户名或邮箱已被注册"))
	case errors.Is(err, service.ErrAuthRateLimited):
		c.Error(appErrors.NewTooManyRequestsError("操作过于频繁，请稍后再试"))
	case errors.Is(err, service.ErrAuthUnavailable):
		logger.Error(c.Request.Context(), "authentication service unavailable", err)
		c.Error(appErrors.NewServiceUnavailableError("认证或邮件服务暂时不可用，请检查 SMTP/Redis 配置"))
	default:
		logger.Error(c.Request.Context(), "authentication request failed", err)
		c.Error(appErrors.NewInternalServerError("认证请求处理失败"))
	}
}
