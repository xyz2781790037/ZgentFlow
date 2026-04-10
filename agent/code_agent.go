package agent

import (
	"context"
	"fmt"
	pb "ZgentFlow/proto/workflow"
	"ZgentFlow/storage"
	"ZgentFlow/tool"

	"github.com/sashabaranov/go-openai"
)

// CodeAgent 实现了 protobuf 中定义的 AgentWorker 接口
type CodeAgent struct {
	pb.UnimplementedAgentWorkerServer

	llmClient *openai.Client
	model     string
	memory    storage.MemoryStore
	skills    *tool.SkillManager
}

func NewCodeAgent(apiKey string, baseURL string, model string, memory storage.MemoryStore, sm *tool.SkillManager) *CodeAgent {
	// 不要使用默认的 openai.NewClient(apiKey)
	// 使用自定义配置，将其指向兼容的第三方 URL
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	return &CodeAgent{
		llmClient: openai.NewClientWithConfig(config), // 注入自定义配置
		model:     model,
		memory:    memory,
		skills:    sm,
	}
}

// ExecuteTask 核心处理逻辑
func (a *CodeAgent) ExecuteTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	// 【修改】使用自研结构化日志
	tool.Log.Info("开始执行任务", "role", req.AgentRole, "taskID", req.TaskID)

	// 【修改】去除了写死的工具名称，改为动态引导
	systemPrompt := "你是一个全能的 AI Agent。请利用你拥有的技能解决用户问题。如果需要实时数据、客观事实或未知信息，请主动调用 search_web 技能。你可以读取和修改本地代码。"
	userPrompt := fmt.Sprintf("任务要求: %s\n前置输入: %s", req.Instruction, string(req.InputPayload))

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}

	// 【修改】动态从技能背包中获取 OpenAI 格式的工具定义
	toolsDef := a.skills.GetOpenAITools()

	var finalOutput string
	var totalPromptTokens, totalCompletionTokens int

	for i := 0; i < 5; i++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("流转被引擎熔断")
		}

		resp, err := a.llmClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    a.model,
			Messages: messages,
			Tools:    toolsDef,
		})
		if err != nil {
			tool.Log.Error("大模型调用失败", "err", err)
			return &pb.TaskResponse{TaskID: req.TaskID, Status: pb.TaskStatus_STATUS_FAILED, ErrorMessage: err.Error()}, nil
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)
		totalPromptTokens += resp.Usage.PromptTokens
		totalCompletionTokens += resp.Usage.CompletionTokens

		if len(msg.ToolCalls) == 0 {
			finalOutput = msg.Content
			break
		}

		// 解析并调用本地工具
		for _, toolCall := range msg.ToolCalls {
			tool.Log.Info("大模型请求调用技能", "skill", toolCall.Function.Name)

			// 【修改】彻底废弃 if-else，交由 SkillManager 统一路由调度
			toolResult := a.skills.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    toolResult,
				ToolCallID: toolCall.ID,
			})
		}
	}

	tool.Log.Info("任务完成，准备返回结果", "taskID", req.TaskID)

	return &pb.TaskResponse{
		TaskID:           req.TaskID,
		Status:           pb.TaskStatus_STATUS_SUCCESS,
		ResultData:       []byte(finalOutput),
		PromptTokens:     int32(totalPromptTokens),
		CompletionTokens: int32(totalCompletionTokens),
	}, nil
}
