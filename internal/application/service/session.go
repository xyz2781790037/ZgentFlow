package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/xyz2781790037/ZealRAG/internal/config"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/event"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/chat"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"

	chatpipeline "github.com/xyz2781790037/ZealRAG/internal/application/service/chat_pipeline"
)

func sessionUserIDFromContext(ctx context.Context) string {
	userID, _ := types.UserIDFromContext(ctx)
	return userID
}

// generateEventID generates a unique event ID with type suffix for better traceability
func generateEventID(suffix string) string {
	return fmt.Sprintf("%s-%s", uuid.New().String()[:8], suffix)
}

// sessionService implements the SessionService interface for managing conversation sessions.
// History for multi-turn conversations is rebuilt from the messages table on demand
// (see chat_pipeline history loading) — there is no
// separate cross-turn cache layer.
type sessionService struct {
	cfg                   *config.Config                         // Application configuration
	sessionRepo           interfaces.SessionRepository           // Repository for session data
	messageRepo           interfaces.MessageRepository           // Repository for message data
	knowledgeBaseService  interfaces.KnowledgeBaseService        // Service for knowledge base operations
	modelService          interfaces.ModelService                // Service for model operations
	tenantService         interfaces.TenantService               // Service for tenant operations
	eventManager          *chatpipeline.EventManager             // Event manager for chat pipeline
	knowledgeService      interfaces.KnowledgeService            // Service for knowledge operations
	chunkService          interfaces.ChunkService                // Service for chunk operations
	webSearchStateRepo    interfaces.WebSearchStateService       // Service for web search state
	webSearchProviderRepo interfaces.WebSearchProviderRepository // Repository for web search provider entities
	shareService          *KnowledgeBaseShareService
}

// NewSessionService creates a new session service instance with all required dependencies
func NewSessionService(cfg *config.Config,
	sessionRepo interfaces.SessionRepository,
	messageRepo interfaces.MessageRepository,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	chunkService interfaces.ChunkService,
	modelService interfaces.ModelService,
	tenantService interfaces.TenantService,
	eventManager *chatpipeline.EventManager,
	webSearchStateRepo interfaces.WebSearchStateService,
	webSearchProviderRepo interfaces.WebSearchProviderRepository,
	shareService *KnowledgeBaseShareService,
) interfaces.SessionService {
	return &sessionService{
		cfg:                   cfg,
		sessionRepo:           sessionRepo,
		messageRepo:           messageRepo,
		knowledgeBaseService:  knowledgeBaseService,
		knowledgeService:      knowledgeService,
		chunkService:          chunkService,
		modelService:          modelService,
		tenantService:         tenantService,
		eventManager:          eventManager,
		webSearchStateRepo:    webSearchStateRepo,
		webSearchProviderRepo: webSearchProviderRepo,
		shareService:          shareService,
	}
}

// CreateSession creates a new conversation session
func (s *sessionService) CreateSession(ctx context.Context, session *types.Session) (*types.Session, error) {
	logger.Info(ctx, "Start creating session")

	// Validate tenant ID
	if session.TenantID == 0 {
		logger.Error(ctx, "Failed to create session: tenant ID cannot be empty")
		return nil, stderrors.New("tenant ID is required")
	}

	logger.Infof(ctx, "Creating session, tenant ID: %d", session.TenantID)

	// Create session in repository
	createdSession, err := s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, err
	}

	logger.Infof(ctx, "Session created successfully, ID: %s, tenant ID: %d", createdSession.ID, createdSession.TenantID)
	return createdSession, nil
}

// GetSession retrieves a session by its ID
func (s *sessionService) GetSession(ctx context.Context, id string) (*types.Session, error) {
	logger.Info(ctx, "Start retrieving session")

	// Validate session ID
	if id == "" {
		logger.Error(ctx, "Failed to get session: session ID cannot be empty")
		return nil, stderrors.New("session id is required")
	}

	// Get tenant ID from context
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)
	logger.Infof(ctx, "Retrieving session, ID: %s, tenant ID: %d", id, tenantID)

	// Get session from repository
	session, err := s.sessionRepo.Get(ctx, tenantID, userID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": id,
			"tenant_id":  tenantID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Session retrieved successfully, ID: %s, tenant ID: %d", session.ID, session.TenantID)
	if strings.TrimSpace(session.Title) != "" {
		session.Title = normalizeGeneratedSessionTitle(session.Title)
	}
	return session, nil
}

// ListSessions returns a page of sessions with search/source filters, scoped to
// the current tenant (and user when the caller is an authenticated user).
func (s *sessionService) ListSessions(
	ctx context.Context, query *types.SessionListQuery,
) (*types.PageResult, error) {
	if query == nil {
		query = &types.SessionListQuery{}
	}
	query.TenantID = types.MustTenantIDFromContext(ctx)
	if uid, ok := types.UserIDFromContext(ctx); ok {
		query.UserID = uid
	}

	items, total, err := s.sessionRepo.QueryPaged(ctx, query)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": query.TenantID,
			"user_id":   query.UserID,
			"keyword":   query.Keyword,
			"agent_id":  query.AgentID,
		})
		return nil, err
	}
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.Title) != "" {
			item.Title = normalizeGeneratedSessionTitle(item.Title)
		}
	}

	pagination := &types.Pagination{Page: query.Page, PageSize: query.PageSize}
	return types.NewPageResult(total, pagination, items), nil
}

// SetSessionPinned pins or unpins a session for the current user scope.
// Returns the number of rows affected; 0 means the session doesn't exist
// or is not owned by the caller so the handler can respond 404.
func (s *sessionService) SetSessionPinned(
	ctx context.Context, sessionID string, pinned bool,
) (int64, error) {
	if sessionID == "" {
		return 0, stderrors.New("session id is required")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)
	return s.sessionRepo.SetPinned(ctx, tenantID, userID, sessionID, pinned)
}

// UpdateSessionLastRequestState persists the input-bar state used by the most
// recent QA request on this session. Called from the QA handler after a
// request is accepted so the UI can rehydrate the same settings on reopen.
// Best-effort: scope mismatches are logged and swallowed — failing to record
// the UI memo should never fail the user's chat request.
func (s *sessionService) UpdateSessionLastRequestState(
	ctx context.Context, sessionID string, state *types.SessionLastRequestState,
) error {
	if sessionID == "" {
		return stderrors.New("session id is required")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)
	affected, err := s.sessionRepo.UpdateLastRequestState(ctx, tenantID, userID, sessionID, state)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"tenant_id":  tenantID,
		})
		return err
	}
	if affected == 0 {
		logger.Warnf(ctx, "UpdateSessionLastRequestState: no rows affected for session %s", sessionID)
	}
	return nil
}

// DeleteSession removes a session by its ID
func (s *sessionService) DeleteSession(ctx context.Context, id string) error {
	// Validate session ID
	if id == "" {
		logger.Error(ctx, "Failed to delete session: session ID cannot be empty")
		return stderrors.New("session id is required")
	}

	// Get tenant ID from context
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)

	if _, err := s.sessionRepo.Get(ctx, tenantID, userID, id); err != nil {
		return err
	}

	// Cleanup temporary KB stored in Redis for this session
	if err := s.webSearchStateRepo.DeleteWebSearchTempKBState(ctx, id); err != nil {
		logger.Warnf(ctx, "Failed to cleanup temporary KB for session %s: %v", id, err)
	}

	// Delete session from repository
	rows, err := s.sessionRepo.Delete(ctx, tenantID, userID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": id,
			"tenant_id":  tenantID,
		})
		return err
	}
	if rows == 0 {
		return apperrors.ErrSessionNotFound
	}

	return nil
}

// BatchDeleteSessions deletes multiple sessions by IDs
func (s *sessionService) BatchDeleteSessions(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		logger.Error(ctx, "Failed to batch delete sessions: IDs list is empty")
		return stderrors.New("session ids are required")
	}

	// Get tenant ID from context
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)

	visibleIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := s.sessionRepo.Get(ctx, tenantID, userID, id); err == nil {
			visibleIDs = append(visibleIDs, id)
		} else if !stderrors.Is(err, apperrors.ErrSessionNotFound) {
			return err
		}
	}
	if len(visibleIDs) == 0 {
		return apperrors.ErrSessionNotFound
	}

	// Cleanup temporary web-search knowledge bases for each session.
	for _, id := range visibleIDs {
		if err := s.webSearchStateRepo.DeleteWebSearchTempKBState(ctx, id); err != nil {
			logger.Warnf(ctx, "Failed to cleanup temporary KB for session %s: %v", id, err)
		}
	}

	// Batch delete sessions from repository
	if _, err := s.sessionRepo.BatchDelete(ctx, tenantID, userID, visibleIDs); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_ids": visibleIDs,
			"tenant_id":   tenantID,
		})
		return err
	}

	return nil
}

// DeleteAllSessions deletes all sessions for the current tenant
func (s *sessionService) DeleteAllSessions(ctx context.Context) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	userID := sessionUserIDFromContext(ctx)
	logger.Infof(ctx, "Deleting all sessions for tenant %d", tenantID)

	sessions, err := s.sessionRepo.GetByTenantID(ctx, tenantID, userID)
	if err != nil {
		logger.Warnf(ctx, "Failed to list sessions for cleanup: %v", err)
	} else {
		for _, session := range sessions {
			if err := s.webSearchStateRepo.DeleteWebSearchTempKBState(ctx, session.ID); err != nil {
				logger.Warnf(ctx, "Failed to cleanup temporary KB for session %s: %v", session.ID, err)
			}
		}
	}

	if _, err := s.sessionRepo.DeleteAllByTenantID(ctx, tenantID, userID); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		return err
	}

	logger.Infof(ctx, "All sessions deleted for tenant %d", tenantID)
	return nil
}

func (s *sessionService) generateTitle(ctx context.Context,
	session *types.Session, messages []types.Message,
) (string, error) {
	if session == nil {
		logger.Error(ctx, "Failed to generate title: session cannot be empty")
		return "", stderrors.New("session cannot be empty")
	}

	// Skip if title already exists
	if session.Title != "" {
		return session.Title, nil
	}
	var err error
	// Get the first user message, either from provided messages or repository
	var message *types.Message
	if len(messages) == 0 {
		message, err = s.messageRepo.GetFirstMessageOfUser(ctx, session.ID)
		if err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"session_id": session.ID,
			})
			return "", err
		}
	} else {
		for _, m := range messages {
			if m.Role == "user" {
				message = &m
				break
			}
		}
	}

	// Ensure a user message was found
	if message == nil {
		logger.Error(ctx, "No user message found, cannot generate title")
		return "", stderrors.New("no user message found")
	}

	model, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeKnowledgeQA)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return "", fmt.Errorf("failed to resolve default chat model: %w", err)
	}

	chatModel, err := s.modelService.GetChatModel(ctx, model.ID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": model.ID,
		})
		return "", err
	}

	// Prepare messages for title generation
	titlePrompt := types.RenderPromptPlaceholders(s.cfg.Conversation.GenerateSessionTitlePrompt, types.PlaceholderValues{
		"language": types.LanguageNameFromContext(ctx),
	})
	var chatMessages []chat.Message
	chatMessages = append(chatMessages,
		chat.Message{Role: "system", Content: titlePrompt},
	)
	chatMessages = append(chatMessages,
		chat.Message{Role: "user", Content: message.Content},
	)

	// Call model to generate title
	thinking := false
	response, err := chatModel.Chat(ctx, chatMessages, &chat.ChatOptions{
		Temperature: 0.3,
		Thinking:    &thinking,
	})
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return "", err
	}

	// Process and store the generated title. Some models repeat the instruction
	// in their answer; keep only the title so it does not leak into the sidebar.
	session.Title = normalizeGeneratedSessionTitle(response.Content)

	// Update session with new title
	_, err = s.sessionRepo.Update(ctx, session, session.UserID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		return "", err
	}

	return session.Title, nil
}

func normalizeGeneratedSessionTitle(raw string) string {
	title := strings.TrimSpace(raw)
	if strings.HasPrefix(title, "<think>") {
		if end := strings.Index(title, "</think>"); end >= 0 {
			title = title[end+len("</think>"):]
		}
	}
	title = strings.TrimSpace(title)
	for _, prefix := range []string{
		"根据用户的问题，生成的短会话标题为：",
		"根据用户的问题，生成的短会话标题为",
		"根据用户的问题生成的短会话标题为：",
		"根据用户的问题生成的短会话标题为",
		"短会话标题为：",
		"短会话标题为",
		"会话标题：",
		"会话标题:",
		"标题：",
		"标题:",
		"The short conversation title is:",
		"The conversation title is:",
		"Title:",
	} {
		if strings.HasPrefix(title, prefix) {
			title = strings.TrimSpace(strings.TrimPrefix(title, prefix))
			break
		}
	}
	if lines := strings.FieldsFunc(title, func(r rune) bool { return r == '\n' || r == '\r' }); len(lines) > 0 {
		title = strings.TrimSpace(lines[0])
	}
	title = strings.Trim(title, "`* \t\"'“”《》【】")
	if title == "" {
		return "未命名会话"
	}
	return title
}

// GenerateTitleAsync generates a title for the session asynchronously
// This method clones the session and generates the title in a goroutine
// It emits an event when the title is generated
func (s *sessionService) GenerateTitleAsync(
	ctx context.Context,
	session *types.Session,
	userQuery string,
	eventBus *event.EventBus,
) {
	// Use context tenant (effective tenant when using shared agent) so ListModels/GetChatModel find the agent's model.
	// The session row itself is still updated by its persisted tenant/user owner scope.
	tenantID := ctx.Value(types.TenantIDContextKey)
	requestID := ctx.Value(types.RequestIDContextKey)
	language := ctx.Value(types.LanguageContextKey)
	// Keep the Langfuse trace handle so the async title generation shows up
	// as a child of the same trace as the originating chat request.
	langfuseTrace := ctx.Value(types.LangfuseTraceContextKey)
	go func() {
		bgCtx := context.Background()
		if tenantID != nil {
			bgCtx = context.WithValue(bgCtx, types.TenantIDContextKey, tenantID)
		}
		if requestID != nil {
			bgCtx = context.WithValue(bgCtx, types.RequestIDContextKey, requestID)
		}
		if language != nil {
			bgCtx = context.WithValue(bgCtx, types.LanguageContextKey, language)
		}
		if langfuseTrace != nil {
			bgCtx = context.WithValue(bgCtx, types.LangfuseTraceContextKey, langfuseTrace)
		}

		// Skip if title already exists
		if session.Title != "" {
			return
		}

		// Generate title using the first user message
		messages := []types.Message{
			{
				Role:    "user",
				Content: userQuery,
			},
		}

		title, err := s.generateTitle(bgCtx, session, messages)
		if err != nil {
			logger.ErrorWithFields(bgCtx, err, map[string]interface{}{
				"session_id": session.ID,
			})
			return
		}

		// Emit title update event - BUG FIX: use bgCtx instead of ctx
		// The original ctx is from the HTTP request and may be cancelled by the time we get here
		if eventBus != nil {
			if err := eventBus.Emit(bgCtx, event.Event{
				Type:      event.EventSessionTitle,
				SessionID: session.ID,
				Data: event.SessionTitleData{
					SessionID: session.ID,
					Title:     title,
				},
			}); err != nil {
				logger.ErrorWithFields(bgCtx, err, map[string]interface{}{
					"session_id": session.ID,
				})
			} else {
				logger.Infof(bgCtx, "Title update event emitted successfully, session ID: %s, title: %s", session.ID, title)
			}
		}
	}()
}
