package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
)

// 1. 终极抽象：所有的技能都必须实现这个接口
type Skill interface {
	Name() string                           // 技能名称 (例如: search_web)
	Description() string                    // 技能描述 (大模型就是看这个来决定用不用的)
	Parameters() string                     // 参数的 JSON Schema
	Execute(ctx context.Context, args string) (string, error) // 真正的物理执行逻辑
}

// 2. 技能背包 (Registry)：管理 Agent 当前拥有的所有能力
type SkillManager struct {
	skills map[string]Skill
}

func NewSkillManager() *SkillManager {
	return &SkillManager{
		skills: make(map[string]Skill),
	}
}

// Register 挂载技能
func (m *SkillManager) Register(skill Skill) {
	m.skills[skill.Name()] = skill
}

// GetOpenAITools 将背包里的技能，自动翻译成大模型认识的格式
func (m *SkillManager) GetOpenAITools() []openai.Tool {
	var tools []openai.Tool
	for _, s := range m.skills {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        s.Name(),
				Description: s.Description(),
				Parameters:  json.RawMessage(s.Parameters()),
			},
		})
	}
	return tools
}

// Execute 统一路由调度
func (m *SkillManager) Execute(ctx context.Context, name string, args string) string {
	skill, exists := m.skills[name]
	if !exists {
		return fmt.Sprintf("Error: 未知技能 %s", name)
	}

	Log.Info("⚙️ 触发技能", "skill", name, "args", args)
	result, err := skill.Execute(ctx, args)
	if err != nil {
		return fmt.Sprintf("执行失败: %v", err)
	}
	return result
}