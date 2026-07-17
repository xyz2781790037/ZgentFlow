package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// messageService implements the MessageService interface for managing messaging operations
// It handles creating, retrieving, updating, and deleting messages within sessions.
type messageService struct {
	messageRepo interfaces.MessageRepository
	sessionRepo interfaces.SessionRepository
}

// NewMessageService creates a new message service instance with the required repositories
func NewMessageService(messageRepo interfaces.MessageRepository,
	sessionRepo interfaces.SessionRepository,
) interfaces.MessageService {
	return &messageService{
		messageRepo: messageRepo,
		sessionRepo: sessionRepo,
	}
}

func sessionTenantIDForLookup(ctx context.Context) (uint64, bool) {
	return types.TenantIDFromContext(ctx)
}

func sessionUserIDForLookup(ctx context.Context) string {
	userID, _ := types.UserIDFromContext(ctx)
	return userID
}

// CreateMessage creates a new message within an existing session
func (s *messageService) CreateMessage(ctx context.Context, message *types.Message) (*types.Message, error) {
	logger.Info(ctx, "Start creating message")
	logger.Infof(ctx, "Creating message for session ID: %s", message.SessionID)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d, session ID: %s", tenantID, message.SessionID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), message.SessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Session exists, creating message")
	createdMessage, err := s.messageRepo.CreateMessage(ctx, message)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": message.SessionID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Message created successfully, ID: %s", createdMessage.ID)
	return createdMessage, nil
}

// GetMessage retrieves a specific message by its ID within a session
func (s *messageService) GetMessage(ctx context.Context, sessionID string, messageID string) (*types.Message, error) {
	logger.Info(ctx, "Start getting message")
	logger.Infof(ctx, "Getting message, session ID: %s, message ID: %s", sessionID, messageID)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Session exists, getting message")
	message, err := s.messageRepo.GetMessage(ctx, sessionID, messageID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"message_id": messageID,
		})
		return nil, err
	}

	logger.Info(ctx, "Message retrieved successfully")
	return message, nil
}

// GetMessagesBySession retrieves paginated messages for a specific session
func (s *messageService) GetMessagesBySession(ctx context.Context,
	sessionID string, page int, pageSize int,
) ([]*types.Message, error) {
	logger.Info(ctx, "Start getting messages by session")
	logger.Infof(ctx, "Getting messages for session ID: %s, page: %d, pageSize: %d", sessionID, page, pageSize)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Session exists, getting messages")
	messages, err := s.messageRepo.GetMessagesBySession(ctx, sessionID, page, pageSize)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"page":       page,
			"page_size":  pageSize,
		})
		return nil, err
	}

	logger.Infof(ctx, "Retrieved %d messages successfully", len(messages))
	return messages, nil
}

// GetRecentMessagesBySession retrieves the most recent messages from a session
func (s *messageService) GetRecentMessagesBySession(ctx context.Context,
	sessionID string, limit int,
) ([]*types.Message, error) {
	logger.Info(ctx, "Start getting recent messages by session")
	logger.Infof(ctx, "Getting recent messages for session ID: %s, limit: %d", sessionID, limit)

	tenantID, ok := sessionTenantIDForLookup(ctx)
	if !ok {
		logger.Error(ctx, "Tenant ID not found in context for session lookup")
		return nil, errors.New("tenant ID not found in context")
	}
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Session exists, getting recent messages")
	messages, err := s.messageRepo.GetRecentMessagesBySession(ctx, sessionID, limit)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"limit":      limit,
		})
		return nil, err
	}

	logger.Infof(ctx, "Retrieved %d recent messages successfully", len(messages))
	return messages, nil
}

// GetMessagesBySessionBeforeTime retrieves messages sent before a specific time
func (s *messageService) GetMessagesBySessionBeforeTime(ctx context.Context,
	sessionID string, beforeTime time.Time, limit int,
) ([]*types.Message, error) {
	logger.Info(ctx, "Start getting messages before time")
	logger.Infof(ctx, "Getting messages before %v for session ID: %s, limit: %d", beforeTime, sessionID, limit)

	tenantID, ok := sessionTenantIDForLookup(ctx)
	if !ok {
		logger.Error(ctx, "Tenant ID not found in context for session lookup")
		return nil, errors.New("tenant ID not found in context")
	}
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Session exists, getting messages before time")
	messages, err := s.messageRepo.GetMessagesBySessionBeforeTime(ctx, sessionID, beforeTime, limit)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id":  sessionID,
			"before_time": beforeTime,
			"limit":       limit,
		})
		return nil, err
	}

	logger.Infof(ctx, "Retrieved %d messages before time successfully", len(messages))
	return messages, nil
}

// UpdateMessage updates an existing message's content or metadata
func (s *messageService) UpdateMessage(ctx context.Context, message *types.Message) error {
	logger.Info(ctx, "Start updating message")
	logger.Infof(ctx, "Updating message, ID: %s, session ID: %s", message.ID, message.SessionID)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), message.SessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return err
	}

	logger.Info(ctx, "Session exists, updating message")
	err = s.messageRepo.UpdateMessage(ctx, message)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": message.SessionID,
			"message_id": message.ID,
		})
		return err
	}

	logger.Info(ctx, "Message updated successfully")
	return nil
}

// UpdateMessageImages updates only the images JSONB column for a message.
func (s *messageService) UpdateMessageImages(ctx context.Context, sessionID, messageID string, images types.MessageImages) error {
	return s.messageRepo.UpdateMessageImages(ctx, sessionID, messageID, images)
}

// UpdateMessageRenderedContent updates the rendered_content column for a user message.
func (s *messageService) UpdateMessageRenderedContent(ctx context.Context, sessionID, messageID string, renderedContent string) error {
	return s.messageRepo.UpdateMessageRenderedContent(ctx, sessionID, messageID, renderedContent)
}

// DeleteMessage removes a message from a session.
func (s *messageService) DeleteMessage(ctx context.Context, sessionID string, messageID string) error {
	logger.Info(ctx, "Start deleting message")
	logger.Infof(ctx, "Deleting message, session ID: %s, message ID: %s", sessionID, messageID)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Checking if session exists, tenant ID: %d", tenantID)
	_, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return err
	}

	logger.Info(ctx, "Session exists, deleting message")
	err = s.messageRepo.DeleteMessage(ctx, sessionID, messageID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": sessionID,
			"message_id": messageID,
		})
		return err
	}

	logger.Info(ctx, "Message deleted successfully")
	return nil
}

// ClearSessionMessages deletes all messages in a session.
func (s *messageService) ClearSessionMessages(ctx context.Context, sessionID string) error {
	logger.Infof(ctx, "Start clearing all messages for session: %s", sessionID)

	tenantID := types.MustTenantIDFromContext(ctx)
	if _, err := s.sessionRepo.Get(ctx, tenantID, sessionUserIDForLookup(ctx), sessionID); err != nil {
		logger.Errorf(ctx, "Failed to get session: %v", err)
		return err
	}

	if err := s.messageRepo.DeleteMessagesBySessionID(ctx, sessionID); err != nil {
		logger.Errorf(ctx, "Failed to delete messages for session %s: %v", sessionID, err)
		return err
	}

	logger.Infof(ctx, "All messages cleared for session: %s", sessionID)
	return nil
}

// SearchMessages searches stored messages by keyword across all local sessions.
func (s *messageService) SearchMessages(ctx context.Context, params *types.MessageSearchParams) (*types.MessageSearchResult, error) {
	logger.Infof(ctx, "Start searching messages, query: %s", params.Query)

	tenantID := types.MustTenantIDFromContext(ctx)

	if params.Limit <= 0 {
		params.Limit = 20
	}

	keywordResults, err := s.messageRepo.SearchMessagesByKeyword(ctx, tenantID, params.Query, params.SessionIDs, params.Limit*3)
	if err != nil {
		logger.Errorf(ctx, "Keyword search failed: %v", err)
		return nil, err
	}
	items := convertKeywordResults(keywordResults)

	items = s.fetchPartnerMessages(ctx, items)
	grouped := groupByRequestID(items)

	// Apply limit
	if len(grouped) > params.Limit {
		grouped = grouped[:params.Limit]
	}

	result := &types.MessageSearchResult{
		Items: grouped,
		Total: len(grouped),
	}

	logger.Infof(ctx, "Message search completed, returning %d grouped results", result.Total)
	return result, nil
}

// convertKeywordResults converts keyword search results to MessageSearchResultItem
func convertKeywordResults(results []*types.MessageWithSession) []*types.MessageSearchResultItem {
	items := make([]*types.MessageSearchResultItem, 0, len(results))
	for i, msg := range results {
		items = append(items, &types.MessageSearchResultItem{
			MessageWithSession: *msg,
			Score:              float64(len(results)-i) / float64(len(results)),
			MatchType:          "keyword",
		})
	}
	return items
}

// fetchPartnerMessages looks at the search results and, for each request_id that
// has only one role (Q-only or A-only), fetches the partner message from DB so
// that groupByRequestID can produce complete Q&A pairs.
func (s *messageService) fetchPartnerMessages(ctx context.Context, items []*types.MessageSearchResultItem) []*types.MessageSearchResultItem {
	// Collect request_ids and track which roles we already have
	type roleSet struct {
		hasUser      bool
		hasAssistant bool
	}
	seen := make(map[string]*roleSet)
	existingIDs := make(map[string]bool)
	for _, item := range items {
		existingIDs[item.ID] = true
		rid := item.RequestID
		if rid == "" {
			continue
		}
		rs, ok := seen[rid]
		if !ok {
			rs = &roleSet{}
			seen[rid] = rs
		}
		if item.Role == "user" {
			rs.hasUser = true
		} else if item.Role == "assistant" {
			rs.hasAssistant = true
		}
	}

	// Find request_ids that need partner lookup
	var needFetch []string
	for rid, rs := range seen {
		if !rs.hasUser || !rs.hasAssistant {
			needFetch = append(needFetch, rid)
		}
	}
	if len(needFetch) == 0 {
		return items
	}

	// Fetch partner messages
	partners, err := s.messageRepo.GetMessagesByRequestIDs(ctx, needFetch)
	if err != nil {
		logger.Warnf(ctx, "Failed to fetch partner messages: %v", err)
		return items
	}

	// Append only messages not already in results
	for _, p := range partners {
		if existingIDs[p.ID] {
			continue
		}
		existingIDs[p.ID] = true
		items = append(items, &types.MessageSearchResultItem{
			MessageWithSession: *p,
			Score:              0, // partner is not directly matched
			MatchType:          "",
		})
	}

	return items
}

// groupByRequestID merges individual message search results into Q&A pairs
// grouped by request_id. Messages without a request_id become standalone items.
func groupByRequestID(items []*types.MessageSearchResultItem) []*types.MessageSearchGroupItem {
	type groupState struct {
		item  *types.MessageSearchGroupItem
		order int // preserve the order of first appearance
	}
	groups := make(map[string]*groupState)
	nextOrder := 0

	for _, item := range items {
		key := item.RequestID
		if key == "" {
			// No request_id — treat as standalone
			key = item.ID
		}

		g, exists := groups[key]
		if !exists {
			g = &groupState{
				item: &types.MessageSearchGroupItem{
					RequestID:    item.RequestID,
					SessionID:    item.SessionID,
					SessionTitle: item.SessionTitle,
					CreatedAt:    item.CreatedAt,
				},
				order: nextOrder,
			}
			nextOrder++
			groups[key] = g
		}

		// Assign content based on role
		switch item.Role {
		case "user":
			g.item.QueryContent = item.Content
		case "assistant":
			g.item.AnswerContent = item.Content
		}

		// Keep the best score and merge match types
		if item.Score > g.item.Score {
			g.item.Score = item.Score
		}
		if g.item.MatchType == "" {
			g.item.MatchType = item.MatchType
		} else if g.item.MatchType != item.MatchType {
			g.item.MatchType = "hybrid"
		}

		// Use earliest created_at
		if item.CreatedAt.Before(g.item.CreatedAt) {
			g.item.CreatedAt = item.CreatedAt
		}
	}

	// Collect and sort by original order (which reflects score ranking)
	result := make([]*types.MessageSearchGroupItem, 0, len(groups))
	ordered := make([]*groupState, 0, len(groups))
	for _, g := range groups {
		ordered = append(ordered, g)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})
	for _, g := range ordered {
		result = append(result, g.item)
	}

	return result
}
