package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/xyz2781790037/ZealRAG/internal/config"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/gorm"
)

type PromptTemplateView struct {
	Category    string `json:"category"`
	TemplateID  string `json:"template_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	UserPrompt  string `json:"user_prompt,omitempty"`
	Version     int    `json:"version"`
}

type promptGroup struct {
	category  string
	templates *[]config.PromptTemplate
}

// PromptService owns editable prompt versions and keeps the active runtime
// configuration synchronized with PostgreSQL.
type PromptService struct {
	db  *gorm.DB
	cfg *config.Config
	mu  sync.Mutex
}

func NewPromptService(db *gorm.DB, cfg *config.Config) *PromptService {
	s := &PromptService{db: db, cfg: cfg}
	if err := s.initialize(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Prompt] initialization failed: %v", err)
	}
	return s
}

func (s *PromptService) groups() []promptGroup {
	if s.cfg == nil || s.cfg.PromptTemplates == nil {
		return nil
	}
	pt := s.cfg.PromptTemplates
	return []promptGroup{
		{"system_prompt", &pt.SystemPrompt},
		{"context_template", &pt.ContextTemplate},
		{"rewrite", &pt.Rewrite},
		{"fallback", &pt.Fallback},
		{"generate_session_title", &pt.GenerateSessionTitle},
		{"generate_summary", &pt.GenerateSummary},
		{"generate_questions", &pt.GenerateQuestions},
		{"intent_prompts", &pt.IntentPrompts},
	}
}

func (s *PromptService) initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return errors.New("database is unavailable")
	}
	for _, group := range s.groups() {
		for i := range *group.templates {
			template := &(*group.templates)[i]
			var active types.PromptVersion
			err := s.db.WithContext(ctx).
				Where("category = ? AND template_id = ? AND is_active = ?", group.category, template.ID, true).
				First(&active).Error
			switch {
			case err == nil:
				template.Content = active.Content
				template.User = active.UserPrompt
			case errors.Is(err, gorm.ErrRecordNotFound):
				row := types.PromptVersion{
					Category: group.category, TemplateID: template.ID, Name: template.Name,
					Content: template.Content, UserPrompt: template.User, Version: 1, IsActive: true,
				}
				if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}
	}
	config.RefreshPromptBindings(s.cfg)
	return nil
}

func (s *PromptService) List(ctx context.Context) ([]PromptTemplateView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []PromptTemplateView
	for _, group := range s.groups() {
		for i := range *group.templates {
			template := (*group.templates)[i]
			var active types.PromptVersion
			err := s.db.WithContext(ctx).
				Where("category = ? AND template_id = ? AND is_active = ?", group.category, template.ID, true).
				First(&active).Error
			if err != nil {
				return nil, err
			}
			out = append(out, PromptTemplateView{
				Category: group.category, TemplateID: template.ID, Name: template.Name,
				Description: template.Description, Content: active.Content,
				UserPrompt: active.UserPrompt, Version: active.Version,
			})
		}
	}
	return out, nil
}

func (s *PromptService) History(
	ctx context.Context, category, templateID string,
) ([]types.PromptVersion, error) {
	var rows []types.PromptVersion
	err := s.db.WithContext(ctx).
		Where("category = ? AND template_id = ?", category, templateID).
		Order("version DESC").
		Find(&rows).Error
	return rows, err
}

func (s *PromptService) Update(
	ctx context.Context, category, templateID, content, userPrompt string,
) (*types.PromptVersion, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("prompt content cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	template, err := s.findTemplate(category, templateID)
	if err != nil {
		return nil, err
	}
	var created types.PromptVersion
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var latest int
		if err := tx.Model(&types.PromptVersion{}).
			Where("category = ? AND template_id = ?", category, templateID).
			Select("COALESCE(MAX(version), 0)").Scan(&latest).Error; err != nil {
			return err
		}
		if err := tx.Model(&types.PromptVersion{}).
			Where("category = ? AND template_id = ? AND is_active = ?", category, templateID, true).
			Update("is_active", false).Error; err != nil {
			return err
		}
		created = types.PromptVersion{
			Category: category, TemplateID: templateID, Name: template.Name,
			Content: content, UserPrompt: userPrompt, Version: latest + 1, IsActive: true,
		}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, err
	}
	template.Content = created.Content
	template.User = created.UserPrompt
	config.RefreshPromptBindings(s.cfg)
	return &created, nil
}

func (s *PromptService) Rollback(
	ctx context.Context, category, templateID string, version int,
) (*types.PromptVersion, error) {
	var selected types.PromptVersion
	if err := s.db.WithContext(ctx).
		Where("category = ? AND template_id = ? AND version = ?", category, templateID, version).
		First(&selected).Error; err != nil {
		return nil, err
	}
	return s.Update(ctx, category, templateID, selected.Content, selected.UserPrompt)
}

func (s *PromptService) findTemplate(category, templateID string) (*config.PromptTemplate, error) {
	for _, group := range s.groups() {
		if group.category != category {
			continue
		}
		for i := range *group.templates {
			if (*group.templates)[i].ID == templateID {
				return &(*group.templates)[i], nil
			}
		}
	}
	return nil, fmt.Errorf("prompt template %s/%s not found", category, templateID)
}
