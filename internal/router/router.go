package router

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"

	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	"github.com/xyz2781790037/ZealRAG/internal/handler"
	"github.com/xyz2781790037/ZealRAG/internal/handler/session"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/middleware"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// RouterParams 路由参数
type RouterParams struct {
	dig.In

	FileService                 interfaces.FileService
	UserService                 interfaces.UserService
	AuthService                 *service.AuthService
	SessionService              interfaces.SessionService
	MessageService              interfaces.MessageService
	ModelService                interfaces.ModelService
	KBHandler                   *handler.KnowledgeBaseHandler
	KBShareHandler              *handler.KnowledgeBaseShareHandler
	KBShareService              *service.KnowledgeBaseShareService
	KnowledgeHandler            *handler.KnowledgeHandler
	WorkspaceHandler            *handler.WorkspaceHandler
	ChunkHandler                *handler.ChunkHandler
	SessionHandler              *session.Handler
	MessageHandler              *handler.MessageHandler
	ModelHandler                *handler.ModelHandler
	ModelCredentialsHandler     *handler.ModelCredentialsHandler
	ModelAPIConfigHandler       *handler.ModelAPIConfigHandler
	InitializationHandler       *handler.InitializationHandler
	SystemHandler               *handler.SystemHandler
	WebSearchProviderHandler    *handler.WebSearchProviderHandler
	WebSearchCredentialsHandler *handler.WebSearchProviderCredentialsHandler
	FAQHandler                  *handler.FAQHandler
	AnswerModeHandler           *handler.AnswerModeHandler
	PromptHandler               *handler.PromptHandler
	AuthHandler                 *handler.AuthHandler
}

// NewRouter 创建新的路由
func NewRouter(params RouterParams) *gin.Engine {
	r := gin.New()
	r.ContextWithFallback = true

	// Gin trusts all proxies by default. Restrict proxy headers to configured
	// fronting networks so ClientIP cannot be spoofed by direct clients.
	if err := r.SetTrustedProxies(trustedProxies()); err != nil {
		logger.Errorf(context.Background(), "[Router] failed to set trusted proxies: %v", err)
	}

	// CORS 中间件应放在最前面
	r.Use(cors.New(cors.Config{
		AllowOriginWithContextFunc: corsOriginAllowed,
		AllowMethods:               []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:               []string{"Origin", "Content-Type", "Accept", "Accept-Language", "X-Request-ID", "X-CSRF-Token"},
		ExposeHeaders:              []string{"Content-Length", "Access-Control-Allow-Origin"},
		AllowCredentials:           true,
		MaxAge:                     12 * time.Hour,
	}))

	// 基础中间件（不需要认证）
	r.Use(middleware.RequestID())
	r.Use(middleware.Language())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.ErrorHandler())

	// 健康检查（不需要认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	serveFrontendStatic(r)

	// Presigned file access: no auth required, signature-verified.
	servePresignedFiles(r, params.FileService)

	// Langfuse observability — only active when LANGFUSE_* env vars are set.
	// The middleware is registered unconditionally; when disabled it's a no-op.
	r.Use(langfuse.GinMiddleware())

	// Authentication entry points remain public. /me and /logout resolve the
	// session explicitly; all application routes registered afterwards inherit
	// the same session middleware.
	RegisterAuthRoutes(r.Group("/api/v1/auth"), params.AuthHandler, params.AuthService)

	authenticated := r.Group("")
	authenticated.Use(middleware.RequireSession(params.AuthService))
	serveFiles(authenticated, params.FileService)

	v1 := authenticated.Group("/api/v1")
	{
		RegisterWorkspaceRoutes(v1, params.WorkspaceHandler)
		RegisterKnowledgeBaseRoutes(v1, params.KBHandler, params.KBShareHandler, params.KBShareService)
		RegisterKnowledgeRoutes(v1, params.KnowledgeHandler, params.KBShareService)
		RegisterFAQRoutes(v1, params.FAQHandler, params.KBShareService)
		RegisterChunkRoutes(v1, params.ChunkHandler, params.KBShareService)
		RegisterSessionRoutes(v1, params.SessionHandler)
		RegisterChatRoutes(v1, params.SessionHandler)
		RegisterMessageRoutes(v1, params.MessageHandler)
		RegisterModelRoutes(v1, params.ModelHandler, params.ModelCredentialsHandler)
		RegisterModelAPIConfigRoutes(v1, params.ModelAPIConfigHandler)
		RegisterInitializationRoutes(v1, params.InitializationHandler, params.KBShareService)
		RegisterSystemRoutes(v1, params.SystemHandler)
		RegisterWebSearchProviderRoutes(v1, params.WebSearchProviderHandler, params.WebSearchCredentialsHandler)
		RegisterAnswerModeRoutes(v1, params.AnswerModeHandler)
		RegisterPromptRoutes(v1, params.PromptHandler)
		RegisterChunkerDebugRoutes(v1)
	}

	return r
}

func RegisterAuthRoutes(r *gin.RouterGroup, handler *handler.AuthHandler, authService *service.AuthService) {
	r.POST("/codes", handler.SendVerificationCode)
	r.POST("/register", handler.Register)
	r.POST("/login/password", handler.LoginWithPassword)
	r.POST("/login/email-code", handler.LoginWithEmailCode)
	r.GET("/me", middleware.RequireSession(authService), handler.Me)
	r.POST("/logout", middleware.RequireSession(authService), handler.Logout)
}

func allowedCORSOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	}
	origins := make([]string, 0)
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func corsOriginAllowed(c *gin.Context, origin string) bool {
	for _, allowed := range allowedCORSOrigins() {
		if origin == allowed {
			return true
		}
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return parsed.Host != "" && strings.EqualFold(parsed.Host, c.Request.Host)
}

func RegisterPromptRoutes(r *gin.RouterGroup, handler *handler.PromptHandler) {
	prompts := r.Group("/prompts")
	prompts.GET("", handler.List)
	prompts.GET("/:category/:template_id/history", handler.History)
	prompts.PUT("/:category/:template_id", handler.Update)
	prompts.POST("/:category/:template_id/rollback/:version", handler.Rollback)
}

// RegisterChunkerDebugRoutes wires the read-only chunker preview endpoint
// used by the KB editor's debug panel. Stateless — uses no service deps.
func RegisterChunkerDebugRoutes(r *gin.RouterGroup) {
	r.POST("/chunker/preview", handler.PreviewChunking)
}

// RegisterChunkRoutes 注册分块相关的路由
//
// Mutating routes addressed via :knowledge_id inherit per-KB ownership
// from the owning knowledge entry's KB (PR 5, #1303); the chain hop is
// shared with RegisterKnowledgeRoutes via OwnedChunkKBOrAdmin so the
// same "creator-of-the-KB OR Admin+" rule applies to chunk edits.
func RegisterChunkRoutes(r *gin.RouterGroup, handler *handler.ChunkHandler, share *service.KnowledgeBaseShareService) {
	// 分块路由组
	chunks := r.Group("/chunks")
	{
		// 获取分块列表
		chunks.GET("/:knowledge_id", middleware.KnowledgeAccess(share, types.KBRoleReader, "knowledge_id"), handler.ListKnowledgeChunks)
		// 通过chunk_id获取单个chunk（不需要knowledge_id）
		chunks.GET("/by-id/:id", middleware.ChunkAccess(share, types.KBRoleReader, "id"), handler.GetChunkByIDOnly)
		// 删除分块
		chunks.DELETE("/:knowledge_id/:id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "knowledge_id"), handler.DeleteChunk)
		// 删除知识下的所有分块 — KB owner OR Admin+，且对父 KB 有 write 权限
		chunks.DELETE("/:knowledge_id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "knowledge_id"), handler.DeleteChunksByKnowledgeID)
		// 更新分块信息 — KB owner OR Admin+，且对父 KB 有 write 权限
		chunks.PUT("/:knowledge_id/:id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "knowledge_id"), handler.UpdateChunk)
		// 删除单个生成的问题（通过分块 id） — 与其它 chunk mutation 一致：
		// KB owner OR Admin+。早期这里因为链路 (chunk_id -> knowledge_id ->
		// kb -> creator_id) 还没接通，被临时降级成 Contributor，导致一个
		// 「能编辑所有 chunk 的同样规则在这条路由上反而更宽松」的不一致。
		// 现在通过 KBCreatorLookupFromChunkIDParam 把那一跳补上，统一矩阵。
		chunks.DELETE("/by-id/:id/questions", middleware.ChunkAccess(share, types.KBRoleAdmin, "id"), handler.DeleteGeneratedQuestion)
	}
}

// RegisterKnowledgeRoutes 注册知识相关的路由
//
// Per-KB ownership applies on the per-:id mutating routes (PR 5,
// #1303): the URL :id is a knowledge id, OwnedKnowledgeKBOrAdmin
// walks it back to KB.CreatorID so a Contributor who owns the KB can
// edit/delete any of its documents while a non-owner Contributor gets
// 403. KB-scoped upload routes (`/knowledge-bases/:id/knowledge/...`)
// reuse OwnedKBOrAdmin because the URL :id is the KB id directly.
// Cross-:id batch operations stay Contributor-gated — they don't have
// a single owning KB to check against.
func RegisterKnowledgeRoutes(r *gin.RouterGroup, handler *handler.KnowledgeHandler, share *service.KnowledgeBaseShareService) {
	// 知识库下的知识路由组（URL :id is the KB id）
	kb := r.Group("/knowledge-bases/:id/knowledge")
	{
		kb.POST("/file", middleware.KnowledgeBaseAccess(share, types.KBRoleWriter, "id"), middleware.AuditKBMutation(share, "document_uploaded", "id", ""), handler.CreateKnowledgeFromFile)
		kb.GET("", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.ListKnowledge)
		// Clearing all contents under a KB is a destructive op; gate
		// behind Admin instead of Contributor.
		kb.DELETE("", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), middleware.AuditKBMutation(share, "documents_cleared", "id", ""), handler.ClearKnowledgeBaseContents)
	}

	// 知识路由组（URL :id is a knowledge id; the guard walks it to the parent KB）
	k := r.Group("/knowledge")
	{
		// Cross-knowledge endpoints (no :id) can't be gated on a single
		// KB — they accept arbitrary knowledge IDs and the handler must
		// fan out the access check itself. So /batch and /search keep
		// the role-only floor; /move and /batch-delete stay Contributor.
		k.GET("/batch", handler.GetKnowledgeBatch)
		k.GET("/:id", middleware.KnowledgeAccess(share, types.KBRoleReader, "id"), handler.GetKnowledge)
		k.GET("/:id/spans", middleware.KnowledgeAccess(share, types.KBRoleReader, "id"), handler.GetKnowledgeSpans)
		k.DELETE("/:id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "id"), middleware.AuditKBMutation(share, "document_deleted", "", "id"), handler.DeleteKnowledge)
		k.PUT("/:id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "id"), handler.UpdateKnowledge)
		k.POST("/:id/reparse", middleware.KnowledgeReparseAccess(share, "id"), middleware.AuditKBMutation(share, "document_reparse_requested", "", "id"), handler.ReparseKnowledge)
		k.POST("/:id/cancel-parse", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "id"), handler.CancelKnowledgeParse)
		k.GET("/:id/download", middleware.KnowledgeAccess(share, types.KBRoleReader, "id"), handler.DownloadKnowledgeFile)
		k.GET("/:id/preview", middleware.KnowledgeAccess(share, types.KBRoleReader, "id"), handler.PreviewKnowledgeFile)
		k.PUT("/image/:id/:chunk_id", middleware.KnowledgeAccess(share, types.KBRoleAdmin, "id"), handler.UpdateImageInfo)
		// Batch / cross-KB ops stay Contributor-gated: there is no
		// single owning KB to walk back to. A future PR could add a
		// "must own every targeted KB" guard if the requirement
		// surfaces.
		k.GET("/search", handler.SearchKnowledge)
		k.POST("/batch-delete", middleware.KnowledgeBaseJSONBodyAccess(share, types.KBRoleAdmin, "kb_id"), handler.BatchDeleteKnowledge)
		k.POST("/move", middleware.KnowledgeBaseJSONBodyAccess(share, types.KBRoleAdmin, "source_kb_id", "target_kb_id"), handler.MoveKnowledge)
		k.GET("/move/progress/:task_id", handler.GetKnowledgeMoveProgress)
	}

	trash := r.Group("/trash/knowledge")
	{
		trash.GET("", handler.ListTrashedKnowledge)
		trash.POST("/:id/restore", handler.RestoreTrashedKnowledge)
		trash.DELETE("/:id", handler.PurgeTrashedKnowledge)
	}
}

// RegisterFAQRoutes 注册 FAQ 相关路由
//
// FAQ entries are KB content: reads are Viewer+, all mutations
// (create / update / upsert / delete / batch field updates,
// import display flag) are Contributor+. Search is read-only.
func RegisterFAQRoutes(r *gin.RouterGroup, handler *handler.FAQHandler, share *service.KnowledgeBaseShareService) {
	if handler == nil {
		return
	}
	// FAQ entries 是 KB 的子资源（FAQ-type KB 的内容主体）。修改 FAQ
	// 等价于修改 KB 内容，必须遵循 KB 的"creator OR Admin+"矩阵 ——
	// 跟 chunks / wiki pages 保持一致。Viewer+ 可以读，Contributor 不能
	// 改不属于自己的 KB 的 FAQ。
	faq := r.Group("/knowledge-bases/:id/faq")
	{
		// KBAccessRead/Write resolve own/shared/agent-visible access and
		// rewrite the request's tenant context — handler no longer
		// carries an effectiveCtxForKB helper.
		faq.GET("/entries", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.ListEntries)
		faq.GET("/entries/export", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.ExportEntries)
		faq.GET("/entries/:entry_id", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.GetEntry)
		faq.POST("/entries", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.UpsertEntries)
		faq.POST("/entry", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.CreateEntry)
		faq.PUT("/entries/:entry_id", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.UpdateEntry)
		// Unified batch update API - supports is_enabled and is_recommended
		faq.PUT("/entries/fields", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.UpdateEntryFieldsBatch)
		faq.DELETE("/entries", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.DeleteEntries)
		faq.POST("/search", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.SearchFAQ)
		// FAQ import result display status
		faq.PUT("/import/last-result/display", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.UpdateLastImportResultDisplayStatus)
	}
	// FAQ import progress route (outside of knowledge-base scope) — Viewer+
	faqImport := r.Group("/faq/import")
	{
		faqImport.GET("/progress/:task_id", handler.GetImportProgress)
	}
}

// RegisterKnowledgeBaseRoutes 注册知识库相关的路由
func RegisterKnowledgeBaseRoutes(r *gin.RouterGroup, handler *handler.KnowledgeBaseHandler, shareHandler *handler.KnowledgeBaseShareHandler, share *service.KnowledgeBaseShareService) {
	// 知识库路由组
	kb := r.Group("/knowledge-bases")
	{
		// 创建知识库 — Contributor+ (no :id, role-only floor)
		kb.POST("", handler.CreateKnowledgeBase)
		// 获取知识库列表 — Viewer+ (no :id, role-only floor)
		kb.GET("", handler.ListKnowledgeBases)
		// 获取知识库详情 — Viewer+ 且对 KB 有 read 权限
		kb.GET("/:id", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.GetKnowledgeBase)
		// 更新知识库 — 创建者本人 OR Admin+ 且对 KB 有 write 权限
		kb.PUT("/:id", middleware.KnowledgeBaseAccess(share, types.KBRoleOwner, "id"), handler.UpdateKnowledgeBase)
		// Full rebuild publishes vectors and Wiki atomically.
		kb.POST("/:id/rebuild-index", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), middleware.AuditKBMutation(share, "knowledge_base_rebuild_requested", "id", ""), handler.StartKBFullRebuild)
		kb.GET("/:id/rebuild-index/status", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.GetKBFullRebuildState)
		// 删除知识库 — 创建者本人 OR Admin+ 且对 KB 有 write 权限
		kb.DELETE("/:id", middleware.KnowledgeBaseAccess(share, types.KBRoleOwner, "id"), handler.DeleteKnowledgeBase)
		// 置顶/取消置顶知识库 — 创建者本人 OR Admin+ 且对 KB 有 write 权限
		// Pin state is now per-(user, kb) (migration 000050). Anyone with
		// at least Viewer-level read access to the KB — including users
		// who reached it via a shared agent — may pin it for themselves;
		// no edit permission is required. The OwnedKBOrAdmin guard was
		// removed accordingly. The route still requires KB read access
		// so callers can't poke at KBs they can't see.
		kb.PUT("/:id/pin", middleware.KnowledgeBaseAccess(share, types.KBRoleReader, "id"), handler.TogglePinKnowledgeBase)
		// 获取可移动目标知识库列表 — Viewer+ 且对 KB 有 read 权限
		kb.GET("/:id/move-targets", middleware.KnowledgeBaseAccess(share, types.KBRoleAdmin, "id"), handler.ListMoveTargets)

		kb.GET("/:id/sharing", shareHandler.GetSettings)
		kb.PUT("/:id/sharing/invitation", shareHandler.UpdateInvitation)
		kb.GET("/:id/sharing/members", shareHandler.ListMembers)
		kb.PUT("/:id/sharing/members/:user_id", shareHandler.UpdateMemberRole)
		kb.DELETE("/:id/sharing/members/:user_id", shareHandler.RemoveMember)
		kb.POST("/:id/sharing/leave", shareHandler.Leave)
		kb.GET("/:id/sharing/requests", shareHandler.ListJoinRequests)
		kb.POST("/:id/sharing/requests/:request_id/review", shareHandler.ReviewJoinRequest)
		kb.GET("/:id/sharing/logs", shareHandler.ListAuditLogs)
	}
	r.POST("/knowledge-base-invitations/lookup", shareHandler.LookupInvitation)
	r.POST("/knowledge-base-join-requests", shareHandler.SubmitJoinRequest)
	r.GET("/knowledge-base-join-requests/mine", shareHandler.ListMyJoinRequests)

	trash := r.Group("/trash/knowledge-bases")
	{
		trash.GET("", handler.ListTrashedKnowledgeBases)
		trash.POST("/:id/restore", handler.RestoreTrashedKnowledgeBase)
		trash.DELETE("/:id", handler.PurgeTrashedKnowledgeBase)
	}
}

// RegisterMessageRoutes 注册消息相关的路由。
//
// Per-session ownership is enforced inside each handler.
func RegisterMessageRoutes(r *gin.RouterGroup, handler *handler.MessageHandler) {
	messages := r.Group("/messages")
	{
		messages.POST("/search", handler.SearchMessages)
		messages.GET("/:session_id/load", handler.LoadMessages)
		messages.DELETE("/:session_id/:id", handler.DeleteMessage)
	}
}

// RegisterSessionRoutes 注册路由。
//
// Sessions are local-workspace resources and the handler enforces ownership.
func RegisterSessionRoutes(r *gin.RouterGroup, handler *session.Handler) {
	sessions := r.Group("/sessions")
	{
		sessions.POST("", handler.CreateSession)
		sessions.DELETE("/batch", handler.BatchDeleteSessions)
		sessions.GET("/:id", handler.GetSession)
		sessions.GET("", handler.GetSessionsByTenant)
		sessions.DELETE("/:id", handler.DeleteSession)
		sessions.DELETE("/:id/messages", handler.ClearSessionMessages)
		sessions.POST("/:session_id/stop", handler.StopSession)
		// POST and DELETE share this path but gin maintains a separate radix tree
		// per HTTP verb, and the existing trees use different wildcard names
		// (POST uses :session_id, DELETE uses :id). Use whatever matches each
		// tree to avoid "wildcard conflicts" panic at route registration.
		sessions.POST("/:session_id/pin", handler.PinSession)
		sessions.DELETE("/:id/pin", handler.UnpinSession)
		// 继续接收活跃流
		sessions.GET("/continue-stream/:session_id", handler.ContinueStream)
	}
}

// RegisterChatRoutes registers quick RAG chat and direct knowledge search.
func RegisterChatRoutes(r *gin.RouterGroup, handler *session.Handler) {
	knowledgeChat := r.Group("/knowledge-chat")
	{
		knowledgeChat.POST("/:session_id", handler.KnowledgeQA)
	}

	// 新增知识检索接口，不需要session_id
	knowledgeSearch := r.Group("/knowledge-search")
	{
		knowledgeSearch.POST("", handler.SearchKnowledge)
	}
}

// ZealRAG keeps tenant configuration internal to the single local workspace.
// The UI only needs the namespaced KV surface for parser, chat memory, and
// retrieval settings.
func RegisterWorkspaceRoutes(
	r *gin.RouterGroup,
	handler *handler.WorkspaceHandler,
) {
	workspaceRoutes := r.Group("/workspace")
	workspaceRoutes.GET("/config/:key", handler.GetConfig)
	workspaceRoutes.PUT("/config/:key", handler.UpdateConfig)
}

// Models are tenant-wide infrastructure (LLM credentials, embeddings,
// rerankers); Viewer+ for reads, Admin+ for any mutation. Credential
// subresource writes are also Admin+ since secrets are tenant-scoped.
func RegisterModelRoutes(
	r *gin.RouterGroup,
	handler *handler.ModelHandler,
	credHandler *handler.ModelCredentialsHandler,
) {
	// 模型路由组
	models := r.Group("/models")
	{
		// 获取模型厂商列表 — Viewer+
		models.GET("/providers", handler.ListModelProviders)
		// 创建模型 — Admin+
		models.POST("", handler.CreateModel)
		// 获取模型列表 — Viewer+
		models.GET("", handler.ListModels)
		// 获取单个模型 — Viewer+
		models.GET("/:id", handler.GetModel)
		// 更新模型 — Admin+
		models.PUT("/:id", handler.UpdateModel)
		models.PUT("/:id/default", handler.SetDefaultModel)
		// 删除模型 — Admin+
		models.DELETE("/:id", handler.DeleteModel)
		// Per-field credential subresource (see internal/handler/model_credentials.go) — Admin+
		models.PUT("/:id/credentials", credHandler.Put)
		models.DELETE("/:id/credentials/:field", credHandler.DeleteField)
	}
}

func RegisterModelAPIConfigRoutes(
	r *gin.RouterGroup,
	handler *handler.ModelAPIConfigHandler,
) {
	configs := r.Group("/model-api-configs")
	{
		configs.GET("", handler.List)
		configs.POST("", handler.Create)
		configs.PUT("/:id", handler.Update)
		configs.DELETE("/:id", handler.Delete)
	}
}

func RegisterInitializationRoutes(r *gin.RouterGroup, handler *handler.InitializationHandler, share *service.KnowledgeBaseShareService) {
	r.PUT("/initialization/config/:kbId", middleware.KnowledgeBaseAccess(share, types.KBRoleOwner, "kbId"), handler.UpdateKBConfig)

	// Ollama and remote model checks back the model settings page.
	r.GET("/initialization/ollama/status", handler.CheckOllamaStatus)
	r.GET("/initialization/ollama/models", handler.ListOllamaModels)
	r.POST("/initialization/ollama/models/check", handler.CheckOllamaModels)
	r.POST("/initialization/ollama/models/download", handler.DownloadOllamaModel)
	r.GET("/initialization/ollama/download/progress/:taskId", handler.GetDownloadProgress)
	r.GET("/initialization/ollama/download/tasks", handler.ListDownloadTasks)

	// 远程API相关接口
	r.POST("/initialization/remote/check", handler.CheckRemoteModel)
	r.POST("/initialization/embedding/test", handler.TestEmbeddingModel)
	r.POST("/initialization/rerank/check", handler.CheckRerankModel)
	r.POST("/initialization/multimodal/test", handler.TestMultimodalFunction)

}

// RegisterSystemRoutes registers system information routes
//
// Read routes are Viewer+; parser checks and reconnects are Admin+ because
// they actively probe remote document parsing services.
func RegisterSystemRoutes(r *gin.RouterGroup, handler *handler.SystemHandler) {
	systemRoutes := r.Group("/system")
	{
		systemRoutes.GET("/parser-engines", handler.ListParserEngines)
		systemRoutes.POST("/parser-engines/check", handler.CheckParserEngines)
		systemRoutes.POST("/docreader/reconnect", handler.ReconnectDocReader)
	}
}

// RegisterWebSearchProviderRoutes registers CRUD routes for web search
// provider configurations.
//
// Provider rows hold external service credentials (Bing, Tavily, Google,
// etc.); reads are Viewer+, all mutations / connection tests (which
// probe external systems with stored credentials) and the per-field
// credential subresource are Admin+.
func RegisterWebSearchProviderRoutes(
	r *gin.RouterGroup,
	h *handler.WebSearchProviderHandler,
	credHandler *handler.WebSearchProviderCredentialsHandler,
) {
	providers := r.Group("/web-search-providers")
	{
		// List available provider types (metadata for UI forms) — Viewer+
		providers.GET("/types", h.ListProviderTypes)
		// Test with raw credentials (no persistence) — Admin+
		providers.POST("/test", h.TestProviderRaw)
		// CRUD
		providers.POST("", h.CreateProvider)
		providers.GET("", h.ListProviders)
		providers.GET("/:id", h.GetProvider)
		providers.PUT("/:id", h.UpdateProvider)
		providers.DELETE("/:id", h.DeleteProvider)
		// Per-field credential subresource — Admin+
		providers.PUT("/:id/credentials", credHandler.Put)
		providers.DELETE("/:id/credentials/:field", credHandler.DeleteField)
		// Test existing saved provider — Admin+
		providers.POST("/:id/test", h.TestProviderByID)
	}
}

// RegisterAnswerModeRoutes exposes the read-only endpoints used by the
// ZealRAG dual-mode question composer.
func RegisterAnswerModeRoutes(r *gin.RouterGroup, modeHandler *handler.AnswerModeHandler) {
	r.GET("/answer-modes", modeHandler.ListAnswerModes)
}

// trustedProxies returns the proxy CIDRs/IPs whose X-Forwarded-For headers
// gin should trust when resolving the client IP. Defaults to loopback and
// private ranges (covers the bundled nginx in a container network); override
// with ZEALRAG_TRUSTED_PROXIES (comma-separated). An explicit empty value
// disables proxy trust entirely so ClientIP() returns the direct peer.
func trustedProxies() []string {
	raw, ok := os.LookupEnv("ZEALRAG_TRUSTED_PROXIES")
	if !ok {
		return []string{
			"127.0.0.0/8",
			"::1/128",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"fc00::/7",
		}
	}
	proxies := make([]string, 0)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}

// serveFrontendStatic registers a middleware that serves the frontend SPA
// from the ./web directory if it exists. Must be called BEFORE auth middleware
// so static files are served without authentication.
func serveFrontendStatic(r *gin.Engine) {
	webDir := os.Getenv("ZEALRAG_WEB_DIR")
	if webDir == "" {
		webDir = "./web"
	}
	absDir, _ := filepath.Abs(webDir)
	indexPath := filepath.Join(absDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return
	}

	logger.Infof(context.Background(), "[Router] Serving frontend static files from %s", absDir)

	fs := http.Dir(absDir)
	fileServer := http.FileServer(fs)

	r.Use(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/health") {
			c.Next()
			return
		}
		fullPath := filepath.Join(absDir, path)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			setFrontendCacheHeaders(c.Writer, path)
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}
		setFrontendCacheHeaders(c.Writer, "/index.html")
		c.File(indexPath)
		c.Abort()
	})
}

// setFrontendCacheHeaders sets Cache-Control headers for frontend static resources.
// Vite 构建产物中 /assets/* 的文件名带 hash，可长期缓存；其余（index.html、config.js、favicon 等）
// 每次都需 revalidate，避免前端升级后用户看到旧版本。
func setFrontendCacheHeaders(w http.ResponseWriter, path string) {
	if strings.HasPrefix(path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
}

// serveFiles serves files via query parameters and tenant storage settings.
// It is registered after auth middleware, so tenant context comes from authentication.
//
// Route:
//   - /files?file_path=<provider://...>
type getRouteRegistrar interface {
	GET(string, ...gin.HandlerFunc) gin.IRoutes
}

// newFileServeHandler builds the file-proxy handler. It reads the tenant from
// the request context (set by whichever auth middleware precedes it), so the
// same handler backs both the authenticated /files route and the embed route
// (where EmbedAuth injects the channel's tenant). Tenant ownership of the
// requested path is enforced via ValidateStoragePathTenant either way.
func newFileServeHandler(globalFileService interfaces.FileService) gin.HandlerFunc {
	baseDir := os.Getenv("LOCAL_STORAGE_BASE_DIR")
	if baseDir == "" {
		baseDir = "/data/files"
	}
	absDir, _ := filepath.Abs(baseDir)
	if info, err := os.Stat(absDir); err != nil || !info.IsDir() {
		if err := os.MkdirAll(absDir, 0o755); err != nil {
			logger.Warnf(context.Background(), "[Router] Cannot create local storage dir %s: %v", absDir, err)
		}
	}

	return func(c *gin.Context) {
		filePath := strings.TrimSpace(c.Query("file_path"))
		if filePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameter: file_path"})
			return
		}
		if strings.Contains(filePath, "..") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
			return
		}

		tenant, _ := c.Request.Context().Value(types.TenantInfoContextKey).(*types.Tenant)
		if tenant == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: tenant context missing"})
			return
		}

		if err := secutils.ValidateStoragePathTenant(filePath, tenant.ID); err != nil {
			logger.Warnf(context.Background(), "[Router] /files denied cross-tenant or invalid path: tenant_id=%d file_path=%q err=%v", tenant.ID, filePath, err)
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: file path not accessible"})
			return
		}

		if globalFileService == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		reader, err := globalFileService.GetFile(c.Request.Context(), filePath)
		if err != nil {
			logger.Warnf(context.Background(), "[Router] /files get file failed: tenant_id=%d path=%q err=%v", tenant.ID, filePath, err)
			c.Status(http.StatusNotFound)
			return
		}
		defer reader.Close()

		ext := filepath.Ext(filePath)
		contentType := "application/octet-stream"
		switch strings.ToLower(ext) {
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		case ".bmp":
			contentType = "image/bmp"
		case ".svg":
			contentType = "image/svg+xml"
		case ".pdf":
			contentType = "application/pdf"
		case ".csv":
			contentType = "text/csv; charset=utf-8"
		}

		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=86400")
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, reader); err != nil {
			logger.Warnf(context.Background(), "[Router] /files write response failed: %v", err)
		}
	}
}

func serveFiles(r getRouteRegistrar, globalFileService interfaces.FileService) {
	logger.Infof(context.Background(), "[Router] Serving files from /files")
	r.GET("/files", newFileServeHandler(globalFileService))
}

// servePresignedFiles serves files via HMAC-signed URLs without requiring authentication.
// This is used to serve signed images referenced by generated answers.
//
// Routes:
//   - GET  /api/v1/files/presigned?file_path=<provider://...>&tenant_id=<id>&expires=<unix>&sig=<hmac>
//   - HEAD /api/v1/files/presigned?...  (clients may validate Content-Type and
//     Content-Length before rendering image previews)
//
// Failure paths log client IP + User-Agent + (truncated) file_path so operators
// can correlate an IM platform's fetch against the upstream signing log line.
// Without this it is otherwise impossible to tell whether a "broken image" is
// caused by an expired signature, a stale URL cached by the platform, the
// platform's IP being blocked, or the URL simply never reaching us.
func servePresignedFiles(r *gin.Engine, fileService interfaces.FileService) {
	handler := presignedFileHandler(fileService)
	r.GET("/api/v1/files/presigned", handler)
	r.HEAD("/api/v1/files/presigned", handler)
}

// presignedFileHandler returns the shared Gin handler used by both GET and HEAD.
// For HEAD requests it returns the same status + headers but does not stream
// the body — this is enough for IM platforms to validate the URL while saving
// us a full read of the backing object.
func presignedFileHandler(fileService interfaces.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		filePath := strings.TrimSpace(c.Query("file_path"))
		tenantIDStr := strings.TrimSpace(c.Query("tenant_id"))
		expiresStr := strings.TrimSpace(c.Query("expires"))
		sig := strings.TrimSpace(c.Query("sig"))

		if filePath == "" || tenantIDStr == "" || expiresStr == "" || sig == "" {
			logger.Warnf(ctx, "[Router] /files/presigned missing params: client_ip=%s ua=%q file_path=%q tenant_id=%q expires=%q has_sig=%v",
				clientIP, userAgent, filePath, tenantIDStr, expiresStr, sig != "")
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing required parameters"})
			return
		}
		if strings.Contains(filePath, "..") {
			logger.Warnf(ctx, "[Router] /files/presigned rejected path traversal: client_ip=%s file_path=%q", clientIP, filePath)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
			return
		}

		tenantID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned invalid tenant_id: client_ip=%s tenant_id=%q err=%v", clientIP, tenantIDStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
			return
		}

		// Verify HMAC signature and expiry. Logged at Warn because every 403
		// here is a signal worth investigating: either the URL was tampered
		// with, the IM platform cached an expired URL, or SYSTEM_AES_KEY was
		// rotated without invalidating in-flight links.
		if !secutils.VerifyFileURLSig(filePath, tenantID, expiresStr, sig) {
			logger.Warnf(ctx, "[Router] /files/presigned sig invalid or expired: client_ip=%s ua=%q tenant_id=%d file_path=%q expires=%s",
				clientIP, userAgent, tenantID, filePath, expiresStr)
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid or expired signature"})
			return
		}

		if err := secutils.ValidateStoragePathTenant(filePath, tenantID); err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned tenant path mismatch: client_ip=%s tenant_id=%d path=%q err=%v",
				clientIP, tenantID, filePath, err)
			c.Status(http.StatusForbidden)
			return
		}
		if fileService == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		ext := filepath.Ext(filePath)
		contentType := "application/octet-stream"
		switch strings.ToLower(ext) {
		case ".png":
			contentType = "image/png"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		case ".bmp":
			contentType = "image/bmp"
		case ".svg":
			contentType = "image/svg+xml"
		case ".pdf":
			contentType = "application/pdf"
		}

		// HEAD short-circuits the body read. We still need to confirm the
		// object exists, but we use a 0-byte content length and skip io.Copy.
		// Skipping GetFile entirely for HEAD would risk reporting 200 for a
		// signed URL that no longer points at a real object; that mismatch
		// would make subsequent GETs from the same client mysteriously fail.
		reader, err := fileService.GetFile(ctx, filePath)
		if err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned get file failed: client_ip=%s tenant_id=%d path=%q err=%v",
				clientIP, tenantID, filePath, err)
			c.Status(http.StatusNotFound)
			return
		}
		defer reader.Close()

		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=86400")
		if c.Request.Method == http.MethodHead {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, reader); err != nil {
			logger.Warnf(ctx, "[Router] /files/presigned write response failed: client_ip=%s tenant_id=%d err=%v", clientIP, tenantID, err)
		}
	}
}
