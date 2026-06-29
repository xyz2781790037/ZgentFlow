// Package types defines data structures and types used throughout the system
package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// History represents a conversation history entry
// Contains query-answer pairs and associated knowledge references
// Used for tracking conversation context and history
type History struct {
	Query               string     // User query text
	Answer              string     // System response text
	CreateAt            time.Time  // When this history entry was created
	KnowledgeReferences References // Knowledge references used in the answer
}

// MentionedItem represents a mentioned knowledge base or file
type MentionedItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`    // "kb" for knowledge base, "file" for file
	KBType string `json:"kb_type"` // "document" or "faq" (only for kb type)
}

// MessageImage represents an image attached to a chat message
type MessageImage struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
}

// MessageImages is a slice of MessageImage for database storage
type MessageImages []MessageImage

// Value implements the driver.Valuer interface for database serialization
func (m MessageImages) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal([]MessageImage{})
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface for database deserialization
func (m *MessageImages) Scan(value interface{}) error {
	if value == nil {
		*m = make(MessageImages, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*m = make(MessageImages, 0)
		return nil
	}
	return json.Unmarshal(b, m)
}

// MessageAttachment represents a file attachment in a chat message
type MessageAttachment struct {
	URL         string `json:"url"`                    // Storage URL (provider://path)
	FileName    string `json:"file_name"`              // Original filename
	FileType    string `json:"file_type"`              // File extension (e.g., ".pdf", ".docx")
	FileSize    int64  `json:"file_size"`              // File size in bytes
	Content     string `json:"content,omitempty"`      // Extracted text content (for small text files)
	IsTruncated bool   `json:"is_truncated,omitempty"` // Whether content was truncated
	LineCount   int    `json:"line_count,omitempty"`   // Total line count (for text files)
}

// MessageAttachments is a slice of MessageAttachment for database storage
type MessageAttachments []MessageAttachment

// BuildPrompt returns a formatted prompt section for all attachments,
// injecting file metadata and extracted content into the LLM context.
func (attachments MessageAttachments) BuildPrompt() string {
	if len(attachments) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n<attachments>\n")

	for i, att := range attachments {
		sb.WriteString(fmt.Sprintf("<attachment index=\"%d\" name=\"%s\">\n", i+1, att.FileName))
		sb.WriteString("<metadata>\n")
		sb.WriteString(fmt.Sprintf("<type>%s</type>\n", att.FileType))
		sb.WriteString(fmt.Sprintf("<size_kb>%.2f</size_kb>\n", float64(att.FileSize)/1024))
		sb.WriteString("</metadata>\n")

		if att.Content != "" {
			sb.WriteString("<content>\n")
			sb.WriteString(att.Content)
			sb.WriteString("\n</content>\n")

			if att.IsTruncated {
				sb.WriteString(fmt.Sprintf("<note>This file has a total of %d lines, truncated to show only the first 500 lines.</note>\n",
					att.LineCount))
			}
		} else {
			sb.WriteString("<note>File content extraction failed or is unsupported.</note>\n")
		}
		sb.WriteString("</attachment>\n")
	}
	sb.WriteString("</attachments>\n\n")

	return sb.String()
}

// Value implements the driver.Valuer interface for database serialization
func (m MessageAttachments) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal([]MessageAttachment{})
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface for database deserialization
func (m *MessageAttachments) Scan(value interface{}) error {
	if value == nil {
		*m = make(MessageAttachments, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*m = make(MessageAttachments, 0)
		return nil
	}
	return json.Unmarshal(b, m)
}

// MentionedItems is a slice of MentionedItem for database storage
type MentionedItems []MentionedItem

// Value implements the driver.Valuer interface for database serialization
func (m MentionedItems) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal([]MentionedItem{})
	}
	return json.Marshal(m)
}

// Scan implements the sql.Scanner interface for database deserialization
func (m *MentionedItems) Scan(value interface{}) error {
	if value == nil {
		*m = make(MentionedItems, 0)
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*m = make(MentionedItems, 0)
		return nil
	}
	return json.Unmarshal(b, m)
}

// Message represents a conversation message
// Each message belongs to a conversation session and can be from either user or system
// Messages can contain references to knowledge chunks used to generate responses
type Message struct {
	// Unique identifier for the message
	ID string `json:"id"                    gorm:"type:varchar(36);primaryKey"`
	// ID of the session this message belongs to
	SessionID string `json:"session_id"`
	// Request identifier for tracking API requests
	RequestID string `json:"request_id"`
	// Message text content
	Content string `json:"content"`
	// Message role: "user", "assistant", "system"
	Role string `json:"role"`
	// References to knowledge chunks used in the response
	KnowledgeReferences References `json:"knowledge_references"  gorm:"type:json,column:knowledge_references"`
	// Mentioned knowledge bases and files (for user messages)
	// Stores the @mentioned items when user sends a message
	MentionedItems MentionedItems `json:"mentioned_items,omitempty" gorm:"type:jsonb,column:mentioned_items"`
	// Attached images with OCR/Caption text (for user messages)
	Images MessageImages `json:"images,omitempty" gorm:"type:jsonb;column:images"`
	// Attached files (documents, audio, etc., for user messages)
	Attachments MessageAttachments `json:"attachments,omitempty" gorm:"type:jsonb;column:attachments"`
	// Whether message generation is complete
	IsCompleted bool `json:"is_completed"`
	// Whether this response is a fallback (no knowledge base match found)
	IsFallback bool `json:"is_fallback,omitempty"`
	// RenderedContent stores the full RAG-augmented user message (with retrieved context)
	// sent to the LLM. Used to preserve retrieval context across conversation turns.
	// Empty for non-retrieval intents or assistant messages.
	RenderedContent string `json:"-" gorm:"type:text;column:rendered_content;default:''"`
	// Message creation timestamp
	CreatedAt time.Time `json:"created_at"`
	// Last update timestamp
	UpdatedAt time.Time `json:"updated_at"`
	// Soft delete timestamp
	DeletedAt gorm.DeletedAt `json:"deleted_at"            gorm:"index"`
}

// BeforeCreate is a GORM hook that runs before creating a new message record
// Automatically generates a UUID for new messages and initializes knowledge references
// Parameters:
//   - tx: GORM database transaction
//
// Returns:
//   - error: Any error encountered during the hook execution
func (m *Message) BeforeCreate(tx *gorm.DB) (err error) {
	m.ID = uuid.New().String()
	if m.KnowledgeReferences == nil {
		m.KnowledgeReferences = make(References, 0)
	}
	if m.MentionedItems == nil {
		m.MentionedItems = make(MentionedItems, 0)
	}
	if m.Images == nil {
		m.Images = make(MessageImages, 0)
	}
	if m.Attachments == nil {
		m.Attachments = make(MessageAttachments, 0)
	}
	return nil
}

// MessageSearchParams defines the parameters for searching stored messages.
type MessageSearchParams struct {
	// Query text for search
	Query string `json:"query" binding:"required"`
	// Maximum number of results to return (default: 20)
	Limit int `json:"limit"`
	// Filter by specific session IDs (optional, empty means all sessions)
	SessionIDs []string `json:"session_ids"`
}

// MessageWithSession extends Message with session title for search results
type MessageWithSession struct {
	Message
	// Title of the session this message belongs to
	SessionTitle string `json:"session_title"`
}

// MessageSearchResultItem represents a single search result item (internal, pre-merge)
type MessageSearchResultItem struct {
	// The matched message with session info
	MessageWithSession
	// Search relevance score (higher is better)
	Score float64 `json:"score"`
	// How this result was matched: "keyword", "vector", or "hybrid"
	MatchType string `json:"match_type"`
}

// MessageSearchGroupItem represents a merged Q&A pair in search results.
// Messages sharing the same request_id are grouped together so that the user query
// and assistant answer are displayed side by side.
type MessageSearchGroupItem struct {
	// The request_id that groups Q&A together
	RequestID string `json:"request_id"`
	// Session info
	SessionID    string `json:"session_id"`
	SessionTitle string `json:"session_title"`
	// User query content (role=user)
	QueryContent string `json:"query_content"`
	// Assistant answer content (role=assistant), may be empty if only Q matched
	AnswerContent string `json:"answer_content"`
	// Best score among the matched messages in this group
	Score float64 `json:"score"`
	// How this result was matched: "keyword", "vector", or "hybrid"
	MatchType string `json:"match_type"`
	// Timestamp of the earliest message in the group
	CreatedAt time.Time `json:"created_at"`
}

// MessageSearchResult represents the search result for message search
type MessageSearchResult struct {
	// List of merged Q&A pairs
	Items []*MessageSearchGroupItem `json:"items"`
	// Total number of results
	Total int `json:"total"`
}
