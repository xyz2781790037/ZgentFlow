package types

// QARequest consolidates all parameters for quick knowledge QA service calls.
// EventBus is passed separately to avoid circular dependency with the event package.
type QARequest struct {
	Session            *Session           // The conversation session
	Query              string             // User query text
	AssistantMessageID string             // Pre-created assistant message ID
	AnswerMode         *AnswerMode        // Optional answer mode for config override
	KnowledgeBaseIDs   []string           // Knowledge base IDs to search (from request + @mentions)
	KnowledgeIDs       []string           // Specific knowledge (file) IDs to search
	ImageURLs          []string           // Image URLs for multimodal input
	ImageDescription   string             // VLM-generated image description (fallback for non-vision models)
	UserMessageID      string             // Created user message ID
	WebSearchEnabled   bool               // Whether web search is enabled for this request
	QuotedContext      string             // Quoted message content from IM quote-reply (appended at LLM prompt stage, not used for retrieval)
	Attachments        MessageAttachments // File attachments (processed and ready for prompt injection)
}
