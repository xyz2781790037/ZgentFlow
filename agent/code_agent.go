package agent

import (
	pb "ZgentFlow/proto/workflow"
	"ZgentFlow/storage"
	"ZgentFlow/tool"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const aixjChatFallbackModel = "chatgpt-4o-latest"

// CodeAgent 实现了 protobuf 中定义的 AgentWorker 接口
type CodeAgent struct {
	pb.UnimplementedAgentWorkerServer

	llmClient *openai.Client
	model     string
	memory    storage.MemoryStore
	skills    *tool.SkillManager
	baseURL   string
}

func NewCodeAgent(apiKey string, baseURL string, model string, memory storage.MemoryStore, sm *tool.SkillManager) *CodeAgent {
	// 不要使用默认的 openai.NewClient(apiKey)
	// 使用自定义配置，将其指向兼容的第三方 URL
	config := openai.DefaultConfig(apiKey)
	baseURL = NormalizeBaseURL(baseURL)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	return &CodeAgent{
		llmClient: openai.NewClientWithConfig(config), // 注入自定义配置
		model:     model,
		memory:    memory,
		skills:    sm,
		baseURL:   config.BaseURL,
	}
}

// ExecuteTask 核心处理逻辑
func (a *CodeAgent) ExecuteTask(ctx context.Context, req *pb.TaskRequest) (*pb.TaskResponse, error) {
	// 【修改】使用自研结构化日志
	tool.Log.Info("开始执行任务", "role", req.AgentRole, "taskID", req.TaskID)

	// 【修改】去除了写死的工具名称，改为动态引导
	systemPrompt := "你是一个全能的 AI Agent。对于简单算术、常识问答、总结和一般推理，优先直接回答，不要滥用技能。只有在需要实时数据、客观事实、本地代码读取、文件写入或命令执行时，才调用相应技能。完成工具调用后，必须基于已有结果给出最终答案。"
	userPrompt := fmt.Sprintf("任务要求: %s\n前置输入: %s", req.Instruction, string(req.InputPayload))

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}

	// 【修改】动态从技能背包中获取 OpenAI 格式的工具定义
	toolsDef := a.skills.GetOpenAITools()

	var finalOutput string
	var totalPromptTokens, totalCompletionTokens int
	const maxToolRounds = 5
	activeModel := a.model

	for toolRounds := 0; toolRounds < maxToolRounds; toolRounds++ {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("流转被引擎熔断")
		}

		resp, err := a.createChatCompletion(ctx, &activeModel, messages, toolsDef)
		if err != nil {
			tool.Log.Error("大模型调用失败", "err", err)
			return &pb.TaskResponse{TaskID: req.TaskID, Status: pb.TaskStatus_STATUS_FAILED, ErrorMessage: err.Error()}, nil
		}
		if len(resp.Choices) == 0 {
			return &pb.TaskResponse{TaskID: req.TaskID, Status: pb.TaskStatus_STATUS_FAILED, ErrorMessage: "模型未返回任何候选结果"}, nil
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)
		totalPromptTokens += resp.Usage.PromptTokens
		totalCompletionTokens += resp.Usage.CompletionTokens

		if len(msg.ToolCalls) == 0 {
			finalOutput = strings.TrimSpace(msg.Content)
			break
		}

		// 解析并调用本地工具
		for _, toolCall := range msg.ToolCalls {
			tool.Log.Info("大模型请求调用技能", "skill", toolCall.Function.Name)

			// 【修改】彻底废弃 if-else，交由 SkillManager 统一路由调度
			toolResult := a.skills.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       toolCall.Function.Name,
				Content:    toolResult,
				ToolCallID: toolCall.ID,
			})
		}
	}

	if finalOutput == "" {
		tool.Log.Warn("工具调用轮次耗尽，发起最终总结", "taskID", req.TaskID)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "请基于上面的工具结果直接给出最终答案，不要再调用任何技能。",
		})

		resp, err := a.createChatCompletion(ctx, &activeModel, messages, nil)
		if err != nil {
			tool.Log.Error("最终总结失败", "err", err)
			return &pb.TaskResponse{TaskID: req.TaskID, Status: pb.TaskStatus_STATUS_FAILED, ErrorMessage: err.Error()}, nil
		}
		if len(resp.Choices) == 0 {
			return &pb.TaskResponse{TaskID: req.TaskID, Status: pb.TaskStatus_STATUS_FAILED, ErrorMessage: "模型未返回任何候选结果"}, nil
		}

		totalPromptTokens += resp.Usage.PromptTokens
		totalCompletionTokens += resp.Usage.CompletionTokens
		finalOutput = strings.TrimSpace(resp.Choices[0].Message.Content)
	}

	if finalOutput == "" {
		return &pb.TaskResponse{
			TaskID:       req.TaskID,
			Status:       pb.TaskStatus_STATUS_FAILED,
			ErrorMessage: "模型未返回有效结果",
		}, nil
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

func (a *CodeAgent) createChatCompletion(
	ctx context.Context,
	activeModel *string,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
) (openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model:    *activeModel,
		Messages: messages,
		Tools:    tools,
	}

	resp, err := a.llmClient.CreateChatCompletion(ctx, req)
	if err == nil {
		return resp, nil
	}

	fallbackModel := fallbackChatModelFor(a.baseURL, req.Model, err)
	if fallbackModel == "" || fallbackModel == req.Model {
		return resp, wrapLLMRequestError(err, a.baseURL)
	}

	tool.Log.Warn(
		"当前网关不支持该模型的 chat/completions，切换兼容模型重试",
		"requestedModel", req.Model,
		"fallbackModel", fallbackModel,
		"baseURL", a.baseURL,
	)

	req.Model = fallbackModel
	resp, err = a.llmClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return resp, wrapLLMRequestError(err, a.baseURL)
	}

	*activeModel = fallbackModel
	return resp, nil
}

func NormalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")

	switch {
	case strings.HasSuffix(raw, "/v1/chat/completions"):
		return strings.TrimSuffix(raw, "/chat/completions")
	case strings.HasSuffix(raw, "/chat/completions"):
		return strings.TrimSuffix(raw, "/chat/completions")
	case strings.HasSuffix(raw, "/v1/responses"):
		return strings.TrimSuffix(raw, "/responses")
	case strings.HasSuffix(raw, "/responses"):
		return strings.TrimSuffix(raw, "/responses")
	default:
		return raw
	}
}

func wrapLLMRequestError(err error, baseURL string) error {
	if err == nil {
		return nil
	}

	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		if looksLikeHTML(string(reqErr.Body)) || strings.Contains(reqErr.Error(), "invalid character '<'") {
			return fmt.Errorf("%s: %w", buildHTMLResponseHint(baseURL), err)
		}
	}

	if strings.Contains(err.Error(), "invalid character '<'") {
		return fmt.Errorf("%s: %w", buildHTMLResponseHint(baseURL), err)
	}

	return err
}

func fallbackChatModelFor(baseURL string, model string, err error) string {
	if !strings.Contains(strings.ToLower(baseURL), "aixj.vip") {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(model), "gpt-5") {
		return ""
	}
	if !isUnsupportedContentTypeError(err) {
		return ""
	}
	return aixjChatFallbackModel
}

func isUnsupportedContentTypeError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return strings.EqualFold(strings.TrimSpace(apiErr.Message), "Unsupported content type")
	}

	return strings.Contains(strings.ToLower(err.Error()), "unsupported content type")
}

func buildHTMLResponseHint(baseURL string) string {
	baseURL = NormalizeBaseURL(baseURL)
	endpoint := buildChatCompletionsEndpoint(baseURL)
	return fmt.Sprintf(
		"模型接口返回了 HTML 而不是 JSON，请检查模型 API 地址。当前 BaseURL=%s，SDK 实际请求的是 %s；BaseURL 应填写 API 根地址，不要填写网页地址、控制台地址，若已填完整接口路径也只保留到上级目录",
		baseURL,
		endpoint,
	)
}

func buildChatCompletionsEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "/chat/completions"
	}
	return baseURL + "/chat/completions"
}

func looksLikeHTML(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "<!DOCTYPE html") || strings.HasPrefix(body, "<html") || strings.HasPrefix(body, "<")
}
