package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

const (
	AuthPurposeRegister   = "register"
	AuthPurposeEmailLogin = "email_login"
	verificationCodeTTL   = 5 * time.Minute
	sessionTTL            = 7 * 24 * time.Hour
	defaultStorageQuota   = 10 * 1024 * 1024 * 1024
)

var (
	ErrAuthInvalidCredentials = errors.New("invalid username or password")
	ErrAuthInvalidCode        = errors.New("invalid or expired verification code")
	ErrAuthUserExists         = errors.New("username or email already exists")
	ErrAuthRateLimited        = errors.New("too many authentication attempts")
	ErrAuthInvalidSession     = errors.New("invalid or expired session")
	ErrAuthInvalidInput       = errors.New("invalid authentication input")
	ErrAuthUnavailable        = errors.New("authentication service unavailable")
)

type AuthResult struct {
	User         *types.User
	SessionToken string
	CSRFToken    string
}

type authSession struct {
	UserID    string `json:"user_id"`
	CSRFToken string `json:"csrf_token"`
}

// AuthService owns account verification, password checking and opaque Redis
// sessions. The browser only receives the opaque session cookie and CSRF token.
type AuthService struct {
	users        interfaces.UserRepository
	tenants      interfaces.TenantRepository
	redis        *redis.Client
	email        interfaces.VerificationEmailSender
	codeSecret   []byte
	dummyHash    string
	cookieName   string
	cookieSecure bool
}

func NewAuthService(
	users interfaces.UserRepository,
	tenants interfaces.TenantRepository,
	redisClient *redis.Client,
	emailSender interfaces.VerificationEmailSender,
) (*AuthService, error) {
	secret := strings.TrimSpace(os.Getenv("AUTH_CODE_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("SYSTEM_AES_KEY"))
	}
	dummyHash, err := hashPassword("zgentflow-invalid-password")
	if err != nil {
		return nil, err
	}
	cookieName := strings.TrimSpace(os.Getenv("AUTH_COOKIE_NAME"))
	if cookieName == "" {
		cookieName = "zgentflow_session"
	}
	cookieSecure, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("AUTH_COOKIE_SECURE")))
	return &AuthService{
		users:        users,
		tenants:      tenants,
		redis:        redisClient,
		email:        emailSender,
		codeSecret:   []byte(secret),
		dummyHash:    dummyHash,
		cookieName:   cookieName,
		cookieSecure: cookieSecure,
	}, nil
}

func (s *AuthService) CookieName() string        { return s.cookieName }
func (s *AuthService) CookieSecure() bool        { return s.cookieSecure }
func (s *AuthService) SessionTTL() time.Duration { return sessionTTL }

func (s *AuthService) SendVerificationCode(ctx context.Context, rawEmail, purpose, clientID string) error {
	emailAddress, err := normalizeEmail(rawEmail)
	if err != nil || (purpose != AuthPurposeRegister && purpose != AuthPurposeEmailLogin) {
		return ErrAuthInvalidInput
	}
	if len(s.codeSecret) < 16 {
		return ErrAuthUnavailable
	}
	if err := s.checkRateLimit(ctx, "code:ip:"+stableKey(clientID), 20, time.Hour); err != nil {
		return err
	}
	if err := s.checkRateLimit(ctx, "code:email:"+purpose+":"+stableKey(emailAddress), 1, time.Minute); err != nil {
		return err
	}

	user, lookupErr := s.users.GetUserByEmail(ctx, emailAddress)
	if lookupErr != nil && !errors.Is(lookupErr, interfaces.ErrUserNotFound) {
		return fmt.Errorf("%w: lookup email", ErrAuthUnavailable)
	}
	shouldSend := purpose == AuthPurposeRegister && errors.Is(lookupErr, interfaces.ErrUserNotFound)
	if purpose == AuthPurposeEmailLogin {
		shouldSend = lookupErr == nil && user != nil && user.IsActive
	}
	// Keep this endpoint enumeration-resistant: callers receive the same success
	// response when an account already exists or does not exist.
	if !shouldSend {
		return nil
	}

	code, err := generateNumericCode()
	if err != nil {
		return fmt.Errorf("%w: generate code", ErrAuthUnavailable)
	}
	key := s.verificationKey(purpose, emailAddress)
	if err := s.redis.HSet(ctx, key, "digest", s.codeDigest(purpose, emailAddress, code), "attempts", 0).Err(); err != nil {
		return fmt.Errorf("%w: store code", ErrAuthUnavailable)
	}
	if err := s.redis.Expire(ctx, key, verificationCodeTTL).Err(); err != nil {
		_ = s.redis.Del(ctx, key).Err()
		return fmt.Errorf("%w: expire code", ErrAuthUnavailable)
	}
	if err := s.email.SendVerificationCode(ctx, emailAddress, code, purpose); err != nil {
		_ = s.redis.Del(ctx, key).Err()
		return fmt.Errorf("%w: send email: %v", ErrAuthUnavailable, err)
	}
	return nil
}

func (s *AuthService) Register(ctx context.Context, rawUsername, rawEmail, password, code string) (*AuthResult, error) {
	username, usernameKey, err := normalizeUsername(rawUsername)
	if err != nil || !validPassword(password) {
		return nil, ErrAuthInvalidInput
	}
	emailAddress, err := normalizeEmail(rawEmail)
	if err != nil {
		return nil, ErrAuthInvalidInput
	}
	if user, lookupErr := s.users.GetUserByUsername(ctx, usernameKey); lookupErr == nil && user != nil {
		return nil, ErrAuthUserExists
	} else if lookupErr != nil && !errors.Is(lookupErr, interfaces.ErrUserNotFound) {
		return nil, ErrAuthUnavailable
	}
	if user, lookupErr := s.users.GetUserByEmail(ctx, emailAddress); lookupErr == nil && user != nil {
		return nil, ErrAuthUserExists
	} else if lookupErr != nil && !errors.Is(lookupErr, interfaces.ErrUserNotFound) {
		return nil, ErrAuthUnavailable
	}
	if err := s.verifyCode(ctx, AuthPurposeRegister, emailAddress, code); err != nil {
		return nil, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	now := time.Now()
	tenant := &types.Tenant{
		Name:         username + "'s Workspace",
		Description:  "Personal workspace for " + username,
		Status:       "active",
		StorageQuota: defaultStorageQuota,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	user := &types.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        emailAddress,
		PasswordHash: passwordHash,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.CreateUserWithTenant(ctx, user, tenant); err != nil {
		return nil, fmt.Errorf("%w: create personal workspace", ErrAuthUnavailable)
	}
	return s.createSession(ctx, user)
}

func (s *AuthService) LoginWithPassword(ctx context.Context, rawUsername, password, clientID string) (*AuthResult, error) {
	_, usernameKey, err := normalizeUsername(rawUsername)
	if err != nil || password == "" {
		return nil, ErrAuthInvalidCredentials
	}
	if err := s.checkRateLimit(ctx, "login:ip:"+stableKey(clientID), 30, 10*time.Minute); err != nil {
		return nil, err
	}
	if err := s.checkRateLimit(ctx, "login:user:"+stableKey(usernameKey), 10, 10*time.Minute); err != nil {
		return nil, err
	}
	user, lookupErr := s.users.GetUserByUsername(ctx, usernameKey)
	if lookupErr != nil && !errors.Is(lookupErr, interfaces.ErrUserNotFound) {
		return nil, ErrAuthUnavailable
	}
	if errors.Is(lookupErr, interfaces.ErrUserNotFound) || user == nil {
		_ = verifyPassword(password, s.dummyHash)
		return nil, ErrAuthInvalidCredentials
	}
	if !user.IsActive || !verifyPassword(password, user.PasswordHash) {
		return nil, ErrAuthInvalidCredentials
	}
	_ = s.redis.Del(ctx, s.rateLimitKey("login:user:"+stableKey(usernameKey))).Err()
	return s.createSession(ctx, user)
}

func (s *AuthService) LoginWithEmailCode(ctx context.Context, rawEmail, code, clientID string) (*AuthResult, error) {
	emailAddress, err := normalizeEmail(rawEmail)
	if err != nil {
		return nil, ErrAuthInvalidCode
	}
	if err := s.checkRateLimit(ctx, "login:ip:"+stableKey(clientID), 30, 10*time.Minute); err != nil {
		return nil, err
	}
	user, lookupErr := s.users.GetUserByEmail(ctx, emailAddress)
	if lookupErr != nil && !errors.Is(lookupErr, interfaces.ErrUserNotFound) {
		return nil, ErrAuthUnavailable
	}
	if errors.Is(lookupErr, interfaces.ErrUserNotFound) || user == nil || !user.IsActive {
		return nil, ErrAuthInvalidCode
	}
	if err := s.verifyCode(ctx, AuthPurposeEmailLogin, emailAddress, code); err != nil {
		return nil, err
	}
	return s.createSession(ctx, user)
}

func (s *AuthService) AuthenticateSession(ctx context.Context, token string) (*types.User, *types.Tenant, string, error) {
	if token == "" {
		return nil, nil, "", ErrAuthInvalidSession
	}
	raw, err := s.redis.Get(ctx, s.sessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil, "", ErrAuthInvalidSession
	}
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w: load session", ErrAuthUnavailable)
	}
	var session authSession
	if json.Unmarshal(raw, &session) != nil || session.UserID == "" || session.CSRFToken == "" {
		return nil, nil, "", ErrAuthInvalidSession
	}
	user, err := s.users.GetUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, interfaces.ErrUserNotFound) {
			return nil, nil, "", ErrAuthInvalidSession
		}
		return nil, nil, "", fmt.Errorf("%w: load session user", ErrAuthUnavailable)
	}
	if user == nil || !user.IsActive {
		return nil, nil, "", ErrAuthInvalidSession
	}
	tenant, err := s.tenants.GetTenantByID(ctx, user.TenantID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w: load session workspace", ErrAuthUnavailable)
	}
	if tenant == nil || tenant.Status != "active" {
		return nil, nil, "", ErrAuthInvalidSession
	}
	return user, tenant, session.CSRFToken, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.redis.Del(ctx, s.sessionKey(token)).Err(); err != nil {
		return fmt.Errorf("%w: revoke session", ErrAuthUnavailable)
	}
	return nil
}

func (s *AuthService) createSession(ctx context.Context, user *types.User) (*AuthResult, error) {
	token, err := randomToken(32)
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	payload, err := json.Marshal(authSession{UserID: user.ID, CSRFToken: csrfToken})
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	if err := s.redis.Set(ctx, s.sessionKey(token), payload, sessionTTL).Err(); err != nil {
		return nil, fmt.Errorf("%w: create session", ErrAuthUnavailable)
	}
	return &AuthResult{User: user, SessionToken: token, CSRFToken: csrfToken}, nil
}

func (s *AuthService) verifyCode(ctx context.Context, purpose, emailAddress, code string) error {
	if len(code) != 6 {
		return ErrAuthInvalidCode
	}
	key := s.verificationKey(purpose, emailAddress)
	digest, err := s.redis.HGet(ctx, key, "digest").Result()
	if errors.Is(err, redis.Nil) {
		return ErrAuthInvalidCode
	}
	if err != nil {
		return fmt.Errorf("%w: read code", ErrAuthUnavailable)
	}
	attempts, err := s.redis.HIncrBy(ctx, key, "attempts", 1).Result()
	if err != nil {
		return fmt.Errorf("%w: update code attempts", ErrAuthUnavailable)
	}
	if attempts > 5 {
		_ = s.redis.Del(ctx, key).Err()
		return ErrAuthInvalidCode
	}
	expected := s.codeDigest(purpose, emailAddress, code)
	if !hmac.Equal([]byte(digest), []byte(expected)) {
		return ErrAuthInvalidCode
	}
	_ = s.redis.Del(ctx, key).Err()
	return nil
}

func (s *AuthService) checkRateLimit(ctx context.Context, scope string, limit int64, window time.Duration) error {
	key := s.rateLimitKey(scope)
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("%w: rate limit", ErrAuthUnavailable)
	}
	if count == 1 {
		if err := s.redis.Expire(ctx, key, window).Err(); err != nil {
			return fmt.Errorf("%w: rate limit expiry", ErrAuthUnavailable)
		}
	}
	if count > limit {
		return ErrAuthRateLimited
	}
	return nil
}

func (s *AuthService) verificationKey(purpose, emailAddress string) string {
	return "zgentflow:auth:code:" + purpose + ":" + stableKey(emailAddress)
}

func (s *AuthService) rateLimitKey(scope string) string {
	return "zgentflow:auth:rate:" + scope
}

func (s *AuthService) sessionKey(token string) string {
	return "zgentflow:auth:session:" + stableKey(token)
}

func (s *AuthService) codeDigest(purpose, emailAddress, code string) string {
	mac := hmac.New(sha256.New, s.codeSecret)
	_, _ = mac.Write([]byte(purpose + "\x00" + emailAddress + "\x00" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 255 {
		return "", ErrAuthInvalidInput
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", ErrAuthInvalidInput
	}
	return value, nil
}

func normalizeUsername(value string) (display, key string, err error) {
	display = strings.TrimSpace(value)
	count := utf8.RuneCountInString(display)
	if count < 3 || count > 32 {
		return "", "", ErrAuthInvalidInput
	}
	for _, char := range display {
		if char >= '0' && char <= '9' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' {
			continue
		}
		return "", "", ErrAuthInvalidInput
	}
	return display, strings.ToLower(display), nil
}

func validPassword(password string) bool {
	length := utf8.RuneCountInString(password)
	return length >= 1 && length <= 128 && len(password) <= 512
}

func generateNumericCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func stableKey(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
