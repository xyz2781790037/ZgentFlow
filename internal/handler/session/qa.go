package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/event"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// qaRequestContext holds all the common data needed for QA requests
type qaRequestContext struct {
	ctx              context.Context
	c                *gin.Context
	sessionID        string
	requestID        string
	receivedAt       time.Time // Wall-clock time the handler started processing the request
	query            string
	session          *types.Session
	answerMode       *types.AnswerMode
	assistantMessage *types.Message
	knowledgeBaseIDs []string
	knowledgeIDs     []string
	webSearchEnabled bool
	mentionedItems   types.MentionedItems
	images           []ImageAttachment        // Uploaded images with analysis text
	userMessageID    string                   // Created user message ID (populated after createUserMessage)
	attachments      types.MessageAttachments // Processed file attachments

}

// buildQARequest converts the qaRequestContext into a types.QARequest for service invocation.
func (rc *qaRequestContext) buildQARequest() *types.QARequest {
	imageURLs, imageDescription := extractImageURLsAndOCRText(rc.images)
	return &types.QARequest{
		Session:            rc.session,
		Query:              rc.query,
		AssistantMessageID: rc.assistantMessage.ID,
		AnswerMode:         rc.answerMode,
		KnowledgeBaseIDs:   rc.knowledgeBaseIDs,
		KnowledgeIDs:       rc.knowledgeIDs,
		ImageURLs:          imageURLs,
		ImageDescription:   imageDescription,
		UserMessageID:      rc.userMessageID,
		WebSearchEnabled:   rc.webSearchEnabled,
		Attachments:        rc.attachments,
	}
}

// parseQARequest parses and validates a QA request, returns the request context
func (h *Handler) parseQARequest(c *gin.Context, logPrefix string) (*qaRequestContext, *CreateKnowledgeQARequest, error) {
	receivedAt := time.Now()
	ctx := logger.CloneContext(c.Request.Context())
	requestID := secutils.SanitizeForLog(c.GetString(types.RequestIDContextKey.String()))
	logger.Infof(ctx, "[%s] TTFB:start request_id=%s received_at=%d",
		logPrefix, requestID, receivedAt.UnixMilli())

	// Get session ID from URL parameter
	sessionID := secutils.SanitizeForLog(c.Param("session_id"))
	if sessionID == "" {
		logger.Error(ctx, "Session ID is empty")
		return nil, nil, errors.NewBadRequestError(errors.ErrInvalidSessionID.Error())
	}

	// Parse request body
	var request CreateKnowledgeQARequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to parse request data", err)
		return nil, nil, errors.NewBadRequestError(err.Error())
	}

	// Validate query content
	if request.Query == "" {
		logger.Error(ctx, "Query content is empty")
		return nil, nil, errors.NewBadRequestError("Query content cannot be empty")
	}

	// SSRF protection: strip client-supplied URL/Caption fields from image attachments.
	// The URL field must only be populated server-side by saveImageAttachments; an
	// attacker could inject internal network URLs to trigger SSRF via the LLM provider.
	for i := range request.Images {
		request.Images[i].URL = ""
		request.Images[i].Caption = ""
	}

	// Log request details
	if requestJSON, err := json.Marshal(request); err == nil {
		logger.Infof(ctx, "[%s] Request: session_id=%s, request=%s",
			logPrefix, sessionID, secutils.SanitizeForLog(secutils.CompactImageDataURLForLog(string(requestJSON))))
	}

	// Get session
	session, err := h.sessionService.GetSession(ctx, sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session, session ID: %s, error: %v", sessionID, err)
		return nil, nil, errors.NewNotFoundError("Session not found")
	}

	answerMode := h.resolveQuickAnswerMode(ctx)

	// Merge @mentioned items into knowledge_base_ids and knowledge_ids.
	kbIDs, knowledgeIDs := mergeKnowledgeTargets(request.KnowledgeBaseIDs, request.KnowledgeIds, request.MentionedItems)

	// Log merge results for debugging
	logger.Infof(ctx, "[%s] @mention merge: request.KnowledgeBaseIDs=%v, request.MentionedItems=%d, merged kbIDs=%v, merged knowledgeIDs=%v",
		logPrefix, request.KnowledgeBaseIDs, len(request.MentionedItems), kbIDs, knowledgeIDs)

	// Process inline base64 images: decode and save to storage.
	// VLM analysis for RAG paths is deferred to the pipeline rewrite step.
	// For pure chat paths with non-vision models, VLM analysis runs here as fallback.
	if len(request.Images) > 0 {
		if answerMode == nil || !answerMode.Config.ImageUploadEnabled {
			logger.Warnf(ctx, "[%s] Image upload is disabled for quick answer, rejecting %d images", logPrefix, len(request.Images))
			return nil, nil, errors.NewBadRequestError("Image upload is not enabled for this agent")
		}
		tenantID := c.GetUint64(types.TenantIDContextKey.String())
		if err := h.saveImageAttachments(ctx, request.Images, tenantID); err != nil {
			logger.Errorf(ctx, "[%s] Failed to save images: %v", logPrefix, err)
			return nil, nil, errors.NewBadRequestError(fmt.Sprintf("Image save failed: %v", err))
		}

		// VLM analysis is always deferred to after SSE stream is up:
		// - Agent mode: runs in async execution flow with tool_call/tool_result events
		// - Normal RAG mode: runs in the pipeline rewrite step with progress events
		// - Normal pure-chat mode: runs in the async goroutine with progress events
	}

	// Process file attachments: decode and save to storage, extract content
	var processedAttachments types.MessageAttachments
	if len(request.AttachmentUploads) > 0 {
		logger.Infof(ctx, "[%s] processing %d attachment(s)", logPrefix, len(request.AttachmentUploads))

		// MAX_FILE_SIZE_MB env (50MB default). See utils/filesize.go for
		// why this is deploy-time-only rather than a runtime setting.
		maxSizeMB := secutils.GetMaxFileSizeMB()
		maxSize := maxSizeMB * 1024 * 1024
		for i, upload := range request.AttachmentUploads {
			if upload.FileSize > maxSize {
				return nil, nil, errors.NewBadRequestError(
					fmt.Sprintf("attachment %d exceeds size limit of %dMB", i+1, maxSizeMB))
			}
		}

		tenantID := c.GetUint64(types.TenantIDContextKey.String())

		// Process all attachments concurrently.
		processedAttachments = make(types.MessageAttachments, len(request.AttachmentUploads))
		var wg sync.WaitGroup
		errChan := make(chan error, len(request.AttachmentUploads))

		for i, upload := range request.AttachmentUploads {
			wg.Add(1)
			go func(idx int, att AttachmentUpload) {
				defer wg.Done()

				data, err := DecodeBase64Attachment(att.Data)
				if err != nil {
					errChan <- fmt.Errorf("attachment %d decode failed: %w", idx+1, err)
					return
				}

				processed, err := h.attachmentProcessor.ProcessAttachment(
					ctx, data, att.FileName, att.FileSize, tenantID,
				)
				if err != nil {
					errChan <- fmt.Errorf("attachment %d processing failed: %w", idx+1, err)
					return
				}

				processedAttachments[idx] = *processed
			}(i, upload)
		}

		wg.Wait()
		close(errChan)

		if len(errChan) > 0 {
			err := <-errChan
			logger.Errorf(ctx, "[%s] attachment processing failed: %v", logPrefix, err)
			return nil, nil, errors.NewBadRequestError(fmt.Sprintf("attachment processing failed: %v", err))
		}

		logger.Infof(ctx, "[%s] all attachments processed", logPrefix)
	}

	// Build request context
	reqCtx := &qaRequestContext{
		ctx:        ctx,
		c:          c,
		sessionID:  sessionID,
		requestID:  requestID,
		receivedAt: receivedAt,
		query:      request.Query,
		session:    session,
		answerMode: answerMode,
		assistantMessage: &types.Message{
			SessionID:   sessionID,
			Role:        "assistant",
			RequestID:   c.GetString(types.RequestIDContextKey.String()),
			IsCompleted: false,
		},
		knowledgeBaseIDs: secutils.SanitizeForLogArray(kbIDs),
		knowledgeIDs:     secutils.SanitizeForLogArray(knowledgeIDs),
		webSearchEnabled: request.WebSearchEnabled,
		mentionedItems:   convertMentionedItems(request.MentionedItems),
		images:           request.Images,
		attachments:      processedAttachments,
	}

	return reqCtx, &request, nil
}

// resolveQuickAnswerMode resolves the sole built-in quick answer mode.
func (h *Handler) resolveQuickAnswerMode(ctx context.Context) *types.AnswerMode {
	answerMode, err := h.answerModeService.GetAnswerModeByID(ctx, types.BuiltinQuickAnswerID)
	if err != nil {
		logger.Warnf(ctx, "Failed to get quick answer mode: %v", err)
		return nil
	}
	logger.Infof(ctx, "Using quick answer mode: ID=%s, Name=%s", answerMode.ID, answerMode.Name)
	return answerMode
}

// mergeKnowledgeTargets merges request KB/knowledge IDs with @mentioned items into deduplicated slices.
func mergeKnowledgeTargets(requestKBIDs []string, requestKnowledgeIDs []string, mentionedItems []MentionedItemRequest) (kbIDs []string, knowledgeIDs []string) {
	kbIDSet := make(map[string]bool)
	kbIDs = make([]string, 0, len(requestKBIDs)+len(mentionedItems))
	for _, id := range requestKBIDs {
		if id != "" && !kbIDSet[id] {
			kbIDs = append(kbIDs, id)
			kbIDSet[id] = true
		}
	}

	knowledgeIDSet := make(map[string]bool)
	knowledgeIDs = make([]string, 0, len(requestKnowledgeIDs)+len(mentionedItems))
	for _, id := range requestKnowledgeIDs {
		if id != "" && !knowledgeIDSet[id] {
			knowledgeIDs = append(knowledgeIDs, id)
			knowledgeIDSet[id] = true
		}
	}

	for _, item := range mentionedItems {
		if item.ID == "" {
			continue
		}
		switch item.Type {
		case "kb":
			if !kbIDSet[item.ID] {
				kbIDs = append(kbIDs, item.ID)
				kbIDSet[item.ID] = true
			}
		case "file":
			if !knowledgeIDSet[item.ID] {
				knowledgeIDs = append(knowledgeIDs, item.ID)
				knowledgeIDSet[item.ID] = true
			}
		}
	}
	return kbIDs, knowledgeIDs
}

// sseStreamContext holds the context for SSE streaming
type sseStreamContext struct {
	eventBus         *event.EventBus
	asyncCtx         context.Context
	cancel           context.CancelFunc
	assistantMessage *types.Message
}

// setupSSEStream sets up the SSE streaming context
func (h *Handler) setupSSEStream(reqCtx *qaRequestContext, generateTitle bool) *sseStreamContext {
	// Set SSE headers
	setSSEHeaders(reqCtx.c)

	baseCtx := reqCtx.ctx

	// Create EventBus and cancellable context
	eventBus := event.NewEventBus()
	asyncCtx, cancel := context.WithCancel(logger.CloneContext(baseCtx))

	streamCtx := &sseStreamContext{
		eventBus:         eventBus,
		asyncCtx:         asyncCtx,
		cancel:           cancel,
		assistantMessage: reqCtx.assistantMessage,
	}

	// Setup stop event handler
	h.setupStopEventHandler(eventBus, reqCtx.sessionID, reqCtx.session.TenantID, reqCtx.assistantMessage, cancel)

	// Watch for stop events independently of the client SSE connection so a
	// user-requested stop reliably cancels generation even when the client
	// has already disconnected (e.g. API-Key callers that close the stream
	// before POSTing /stop). The watcher self-terminates on a terminal stream
	// event, so its lifetime is decoupled from when the QA service call
	// returns (KnowledgeQA returns immediately while streaming continues in a
	// background goroutine, whereas AgentQA blocks until done). Use a
	// connection-independent context derived from baseCtx so it survives the
	// client disconnect.
	h.startStopWatcher(logger.CloneContext(baseCtx), reqCtx.sessionID, reqCtx.assistantMessage.ID, eventBus)

	// Forward quick-answer events into the reconnectable Redis stream.
	h.setupQuickStreamForwarder(asyncCtx, reqCtx.sessionID, reqCtx.assistantMessage.ID, eventBus)

	// Generate title if needed
	if generateTitle && reqCtx.session.Title == "" {
		logger.Infof(reqCtx.ctx, "Session has no title, starting async title generation, session ID: %s", reqCtx.sessionID)
		h.sessionService.GenerateTitleAsync(asyncCtx, reqCtx.session, reqCtx.query, eventBus)
	}

	return streamCtx
}

// SearchKnowledge godoc
// @Summary      知识搜索
// @Description  在知识库中搜索（不使用LLM总结）
// @Tags         问答
// @Accept       json
// @Produce      json
// @Param        request  body      SearchKnowledgeRequest  true  "搜索请求"
// @Success      200      {object}  map[string]interface{}  "搜索结果"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Router       /sessions/search [post]
func (h *Handler) SearchKnowledge(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	logger.Info(ctx, "Start processing knowledge search request")

	// Parse request body
	var request SearchKnowledgeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to parse request data", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Validate request parameters
	if request.Query == "" {
		logger.Error(ctx, "Query content is empty")
		c.Error(errors.NewBadRequestError("Query content cannot be empty"))
		return
	}

	// Merge single knowledge_base_id into knowledge_base_ids for backward compatibility
	knowledgeBaseIDs := request.KnowledgeBaseIDs
	if request.KnowledgeBaseID != "" {
		// Check if it's already in the list to avoid duplicates
		found := false
		for _, id := range knowledgeBaseIDs {
			if id == request.KnowledgeBaseID {
				found = true
				break
			}
		}
		if !found {
			knowledgeBaseIDs = append(knowledgeBaseIDs, request.KnowledgeBaseID)
		}
	}

	if len(knowledgeBaseIDs) == 0 && len(request.KnowledgeIDs) == 0 {
		logger.Error(ctx, "No knowledge base IDs or knowledge IDs provided")
		c.Error(errors.NewBadRequestError("At least one knowledge_base_id, knowledge_base_ids or knowledge_ids must be provided"))
		return
	}

	logger.Infof(
		ctx,
		"Knowledge search request, knowledge base IDs: %v, knowledge IDs: %v, query: %s",
		secutils.SanitizeForLogArray(knowledgeBaseIDs),
		secutils.SanitizeForLogArray(request.KnowledgeIDs),
		secutils.SanitizeForLog(request.Query),
	)

	// Directly call knowledge retrieval service without LLM summarization
	searchResults, err := h.sessionService.SearchKnowledge(ctx, knowledgeBaseIDs, request.KnowledgeIDs, request.Query)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge search completed, found %d results", len(searchResults))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    searchResults,
	})
}

// KnowledgeQA godoc
// @Summary      知识问答
// @Description  基于知识库的问答（使用LLM总结），支持SSE流式响应
// @Tags         问答
// @Accept       json
// @Produce      text/event-stream
// @Param        session_id  path      string                   true  "会话ID"
// @Param        request     body      CreateKnowledgeQARequest true  "问答请求"
// @Success      200         {object}  map[string]interface{}   "问答结果（SSE流）"
// @Failure      400         {object}  errors.AppError          "请求参数错误"
// @Router       /sessions/{session_id}/knowledge-qa [post]
func (h *Handler) KnowledgeQA(c *gin.Context) {
	// Parse and validate request
	reqCtx, request, err := h.parseQARequest(c, "KnowledgeQA")
	if err != nil {
		c.Error(err)
		return
	}

	// Execute quick QA and generate a title unless disabled.
	h.executeQA(reqCtx, !request.DisableTitle)
}

// executeQA handles the quick RAG execution flow, including SSE setup,
// optional image analysis, persistence, and error handling.
func (h *Handler) executeQA(reqCtx *qaRequestContext, generateTitle bool) {
	ctx := reqCtx.ctx
	sessionID := reqCtx.sessionID

	// Persist the input-bar state used for this request so reopening the
	// session can rehydrate KB and web-search selections.
	// This is a pure UI memo (no behavioural effect) and runs in a goroutine
	// to avoid adding a DB round-trip to TTFB. Use WithoutCancel so a fast
	// client disconnect doesn't drop the write.
	go h.persistLastRequestState(ctx, reqCtx)

	// Create user message
	userMsg, err := h.createUserMessage(ctx, sessionID, reqCtx.query, reqCtx.requestID, reqCtx.mentionedItems, convertImageAttachments(reqCtx.images), reqCtx.attachments)
	if err != nil {
		reqCtx.c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	reqCtx.userMessageID = userMsg.ID

	// Create assistant message
	assistantMessagePtr, err := h.createAssistantMessage(ctx, reqCtx.assistantMessage)
	if err != nil {
		reqCtx.c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	reqCtx.assistantMessage = assistantMessagePtr

	logger.Infof(ctx, "Using knowledge bases: %v", reqCtx.knowledgeBaseIDs)

	// Setup SSE stream
	streamCtx := h.setupSSEStream(reqCtx, generateTitle)

	// Register completion handling for the quick answer stream.
	var normalCompletionOnce sync.Once
	completeNormalMessage := func(content, userQuery string) {
		normalCompletionOnce.Do(func() {
			if content != "" {
				streamCtx.assistantMessage.Content = content
			}
			updateCtx := context.WithValue(
				context.WithoutCancel(streamCtx.asyncCtx),
				types.TenantIDContextKey,
				reqCtx.session.TenantID,
			)
			h.completeAssistantMessage(updateCtx, streamCtx.assistantMessage)
			streamCtx.eventBus.Emit(context.WithoutCancel(streamCtx.asyncCtx), event.Event{
				Type:      event.EventAgentComplete,
				SessionID: sessionID,
				Data:      event.AgentCompleteData{FinalAnswer: streamCtx.assistantMessage.Content},
			})
		})
	}

	streamCtx.eventBus.On(event.EventAgentFinalAnswer, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		streamCtx.assistantMessage.Content += data.Content
		if data.IsFallback {
			streamCtx.assistantMessage.IsFallback = true
		}
		if data.Done {
			logger.Infof(streamCtx.asyncCtx, "Knowledge QA service completed for session: %s", sessionID)
			completeNormalMessage("", reqCtx.query)
		}
		return nil
	})

	// Execute QA asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 10240)
				runtime.Stack(buf, true)
				stageName := "Knowledge QA"
				logger.ErrorWithFields(streamCtx.asyncCtx,
					errors.NewInternalServerError(fmt.Sprintf("%s service panicked: %v\n%s", stageName, r, string(buf))),
					map[string]interface{}{"session_id": sessionID})
				errorContent := fmt.Sprintf("问答生成失败：%s 处理异常", stageName)
				streamCtx.eventBus.Emit(context.WithoutCancel(streamCtx.asyncCtx), event.Event{
					Type:      event.EventError,
					SessionID: sessionID,
					Data: event.ErrorData{
						Error:     errorContent,
						Stage:     "knowledge_qa_execution",
						SessionID: sessionID,
					},
				})
				completeNormalMessage(errorContent, "")
			}
		}()

		// Run VLM image analysis if applicable
		h.runVLMAnalysisIfNeeded(streamCtx, reqCtx)

		// Build QA request and invoke the appropriate service
		qaReq := reqCtx.buildQARequest()

		stageName := "knowledge_qa_execution"
		serviceErr := h.sessionService.KnowledgeQA(streamCtx.asyncCtx, qaReq, streamCtx.eventBus)

		if serviceErr != nil {
			// A user-requested stop cancels asyncCtx, which surfaces here as a
			// context cancellation. That is an expected outcome, not a failure:
			// the stop event already notifies the client, so don't emit a
			// spurious error event (which would otherwise show an error toast).
			if streamCtx.asyncCtx.Err() != nil {
				logger.Infof(streamCtx.asyncCtx, "QA cancelled by user stop for session: %s", sessionID)
			} else {
				logger.ErrorWithFields(streamCtx.asyncCtx, serviceErr, nil)
				errorContent := fmt.Sprintf("问答生成失败：%v", serviceErr)
				streamCtx.eventBus.Emit(context.WithoutCancel(streamCtx.asyncCtx), event.Event{
					Type:      event.EventError,
					SessionID: sessionID,
					Data: event.ErrorData{
						Error:     errorContent,
						Stage:     stageName,
						SessionID: sessionID,
					},
				})
				completeNormalMessage(errorContent, "")
			}
		}
	}()

	// Handle SSE events (blocking)
	shouldWaitForTitle := generateTitle && reqCtx.session.Title == ""
	h.handleStreamEventsForSSE(ctx, reqCtx.c, sessionID, reqCtx.assistantMessage.ID,
		reqCtx.requestID, streamCtx.eventBus, shouldWaitForTitle)
}

// runVLMAnalysisIfNeeded runs VLM image analysis within the async goroutine,
// emitting tool_call/tool_result events so the user can see progress.
// VLM only runs on the pure-chat path (no KB, no web search); RAG paths defer
// image handling to the pipeline rewrite step.
func (h *Handler) runVLMAnalysisIfNeeded(streamCtx *sseStreamContext, reqCtx *qaRequestContext) {
	if len(reqCtx.images) == 0 {
		return
	}
	vlmModel, err := h.modelService.GetDefaultModel(streamCtx.asyncCtx, types.ModelTypeVLLM)
	if err != nil {
		return
	}

	sessionID := reqCtx.sessionID

	hasRequestKBs := len(reqCtx.knowledgeBaseIDs) > 0 || len(reqCtx.knowledgeIDs) > 0
	modeWillResolveKBs := reqCtx.answerMode != nil &&
		!reqCtx.answerMode.Config.RetrieveKBOnlyWhenMentioned &&
		reqCtx.answerMode.Config.KBSelectionMode != "none"
	if hasRequestKBs || modeWillResolveKBs || reqCtx.webSearchEnabled {
		return // VLM will be handled by the pipeline rewrite step
	}

	// Emit VLM tool call/result events
	toolCallID := uuid.New().String()
	iteration := 0

	streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
		Type:      event.EventAgentToolCall,
		SessionID: sessionID,
		Data: event.AgentToolCallData{
			ToolCallID: toolCallID,
			ToolName:   "image_analysis",
			Iteration:  iteration,
		},
	})

	vlmStart := time.Now()
	h.analyzeImageAttachments(streamCtx.asyncCtx, reqCtx.images,
		vlmModel.ID, reqCtx.query)

	streamCtx.eventBus.Emit(streamCtx.asyncCtx, event.Event{
		Type:      event.EventAgentToolResult,
		SessionID: sessionID,
		Data: event.AgentToolResultData{
			ToolCallID: toolCallID,
			ToolName:   "image_analysis",
			Output:     "已分析图片内容",
			Success:    true,
			Duration:   time.Since(vlmStart).Milliseconds(),
			Iteration:  iteration,
		},
	})
}

// persistLastRequestState records the input-bar state the user just sent so
// that reopening this session restores mode/KB/web-search picks.
// Pure UI memo — failures are logged but never bubble up; the caller runs
// this in a goroutine and is safe to discard the returned context.
func (h *Handler) persistLastRequestState(parentCtx context.Context, reqCtx *qaRequestContext) {
	// Detach from the HTTP request lifetime: this write must survive both
	// SSE disconnects and the parent gin context being released after the
	// handler returns.
	ctx := logger.CloneContext(context.WithoutCancel(parentCtx))

	state := &types.SessionLastRequestState{
		KnowledgeBaseIDs: reqCtx.knowledgeBaseIDs,
		KnowledgeIDs:     reqCtx.knowledgeIDs,
		WebSearchEnabled: reqCtx.webSearchEnabled,
	}

	if err := h.sessionService.UpdateSessionLastRequestState(ctx, reqCtx.sessionID, state); err != nil {
		logger.Warnf(ctx, "persist last_request_state failed for session %s: %v", reqCtx.sessionID, err)
	}
}

// completeAssistantMessage marks an assistant message as complete and persists it.
func (h *Handler) completeAssistantMessage(ctx context.Context, assistantMessage *types.Message) {
	assistantMessage.UpdatedAt = time.Now()
	assistantMessage.IsCompleted = true
	_ = h.messageService.UpdateMessage(ctx, assistantMessage)
}
