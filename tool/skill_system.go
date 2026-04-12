package tool

import (
	"context"
	"encoding/json"
)

type ShellSkill struct {
	executor *ToolExecutor // 依赖底层物理执行器
}

func NewShellSkill(exec *ToolExecutor) *ShellSkill {
	return &ShellSkill{executor: exec}
}

func (s *ShellSkill) Name() string { return "execute_shell" }

func (s *ShellSkill) Description() string {
	return "在宿主机执行 Linux Shell 命令。你可以使用它来执行 ls, cat, grep 查看代码，或者使用 go build, python 运行程序。"
}

func (s *ShellSkill) Parameters() string {
	return `{"type": "object", "properties": {"command": {"type": "string", "description": "需要执行的终端命令"}}, "required": ["command"]}`
}

func (s *ShellSkill) Execute(ctx context.Context, args string) (string, error) {
	var params struct{ Command string }
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", err
	}
	
	// 调用现有的 executor 方法
	out, err := s.executor.ExecuteShell(ctx, params.Command, 15000)
	if err != nil {
		return err.Error(), nil // 注意：把错误作为文本返回给大模型，让它知道自己写错命令了
	}
	return string(out), nil
}

// ==========================================
// 2. 文件写入技能
// ==========================================
type WriteFileSkill struct {
	executor *ToolExecutor
}

func NewWriteFileSkill(exec *ToolExecutor) *WriteFileSkill {
	return &WriteFileSkill{executor: exec}
}

func (s *WriteFileSkill) Name() string { return "write_file" }

func (s *WriteFileSkill) Description() string {
	return "将文本或代码内容直接写入工作区的文件中。如果文件不存在则自动创建，存在则直接覆盖。"
}

func (s *WriteFileSkill) Parameters() string {
	return `{"type": "object", "properties": {"filename": {"type": "string", "description": "文件名，例如 main.go"}, "content": {"type": "string", "description": "文件完整内容"}}, "required": ["filename", "content"]}`
}

func (s *WriteFileSkill) Execute(ctx context.Context, args string) (string, error) {
	var params struct{ Filename, Content string }
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return "", err
	}
	
	err := s.executor.WriteFile(params.Filename, []byte(params.Content))
	if err != nil {
		return "写入失败: " + err.Error(), nil
	}
	return "写入成功", nil
}