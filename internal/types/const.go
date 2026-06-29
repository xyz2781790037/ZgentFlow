package types

// ContextKey defines a type for context keys to avoid string collision
type ContextKey string

const (
	// TenantIDContextKey is the context key for tenant ID
	TenantIDContextKey ContextKey = "TenantID"
	// CallerTenantIDContextKey keeps the signed-in user's personal tenant when
	// TenantIDContextKey is temporarily switched to a shared KB's storage tenant.
	CallerTenantIDContextKey ContextKey = "CallerTenantID"
	// KnowledgeBaseAccessRoleContextKey carries the resolved per-KB ACL role.
	KnowledgeBaseAccessRoleContextKey ContextKey = "KnowledgeBaseAccessRole"
	// TenantInfoContextKey is the context key for tenant information
	TenantInfoContextKey ContextKey = "TenantInfo"
	// RequestIDContextKey is the context key for request ID
	RequestIDContextKey ContextKey = "RequestID"
	// LoggerContextKey is the context key for logger
	LoggerContextKey ContextKey = "Logger"
	// UserContextKey is the context key for user information
	UserContextKey ContextKey = "User"
	// UserIDContextKey is the context key for user ID
	UserIDContextKey ContextKey = "UserID"
	// EmbedQueryContextKey is the context key for embedding query text
	EmbedQueryContextKey ContextKey = "EmbedQuery"
	// LanguageContextKey is the context key for user language preference (e.g. "zh-CN", "en-US")
	LanguageContextKey ContextKey = "Language"
	// LangfuseTraceContextKey carries the active Langfuse *Trace across the
	// request lifecycle. Defined here (not inside the langfuse package) so
	// that logger.CloneContext can preserve it without importing langfuse.
	LangfuseTraceContextKey ContextKey = "LangfuseTrace"
	// SystemAdminContextKey is the context key indicating whether the user is a system administrator
	SystemAdminContextKey ContextKey = "SystemAdmin"
)

// String returns the string representation of the context key
func (c ContextKey) String() string {
	return string(c)
}
