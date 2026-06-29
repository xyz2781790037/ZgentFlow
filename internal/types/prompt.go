package types

import "time"

// PromptVersion is an immutable snapshot of one editable runtime prompt.
type PromptVersion struct {
	ID         uint64    `json:"id" gorm:"primaryKey"`
	Category   string    `json:"category" gorm:"type:varchar(64);not null;uniqueIndex:idx_prompt_versions_identity,priority:1"`
	TemplateID string    `json:"template_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_prompt_versions_identity,priority:2"`
	Name       string    `json:"name" gorm:"type:varchar(255);not null"`
	Content    string    `json:"content" gorm:"type:text;not null"`
	UserPrompt string    `json:"user_prompt" gorm:"type:text;not null"`
	Version    int       `json:"version" gorm:"not null;uniqueIndex:idx_prompt_versions_identity,priority:3"`
	IsActive   bool      `json:"is_active" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at"`
}

func (PromptVersion) TableName() string {
	return "prompt_versions"
}
