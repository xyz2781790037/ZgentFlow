package service

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

var ErrAnswerModeNotFound = errors.New("answer mode not found")

type answerModeService struct{}

func NewAnswerModeService() interfaces.AnswerModeService {
	return &answerModeService{}
}

func (s *answerModeService) GetAnswerModeByID(ctx context.Context, id string) (*types.AnswerMode, error) {
	if id != types.BuiltinQuickAnswerID {
		return nil, ErrAnswerModeNotFound
	}
	mode := types.GetBuiltinAgentWithContext(ctx, id)
	if mode == nil {
		return nil, ErrAnswerModeNotFound
	}
	return mode, nil
}

func (s *answerModeService) ListAnswerModes(ctx context.Context) ([]*types.AnswerMode, error) {
	result := make([]*types.AnswerMode, 0, 1)
	for _, id := range []string{types.BuiltinQuickAnswerID} {
		mode, err := s.GetAnswerModeByID(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, mode)
	}
	return result, nil
}
