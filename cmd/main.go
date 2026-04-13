package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"ZgentFlow/agent"
	"ZgentFlow/flow"
	pb "ZgentFlow/proto/workflow"
	"ZgentFlow/storage"
	"ZgentFlow/tool"

	"github.com/chzyer/readline"
)

const (
	forcedLLMAPIKey  = "sk-09f436f6ce604355b85438313ad752d6"
	forcedLLMBaseURL = "https://api.deepseek.com"
	forcedLLMModel   = "deepseek-chat"
)

func main() {
	if err := loadFirstEnvFile(".env", "cmd/.env", "../.env"); err != nil {
		tool.Log.Fatal("读取 .env 失败", "err", err)
	}

	eventStore, err := storage.NewEventStore("./badger_data")
	if err != nil {
		tool.Log.Fatal("无法初始化 BadgerDB", "err", err)
	}
	defer eventStore.Close()

	qdrantStore, err := storage.NewQdrantStore("localhost:6334")
	if err != nil {
		tool.Log.Fatal("无法连接 Qdrant", "err", err)
	}
	defer qdrantStore.Close()

	workspacePath := resolveWorkspacePath()
	executor, err := tool.NewToolExecutor(workspacePath)
	if err != nil {
		tool.Log.Fatal("无法初始化工具工作区", "err", err)
	}

	skillManager := tool.NewSkillManager()
	skillManager.Register(&tool.SearchSkill{})
	skillManager.Register(tool.NewShellSkill(executor))
	skillManager.Register(tool.NewWriteFileSkill(executor))

	apiKey := forcedLLMAPIKey
	baseURL := forcedLLMBaseURL
	model := forcedLLMModel
	tool.Log.Warn("模型配置已切回写死 DeepSeek 模式", "baseURL", baseURL, "model", model)

	codeAgent := agent.NewCodeAgent(apiKey, baseURL, model, qdrantStore, skillManager)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		tool.Log.Fatal("Agent 端口监听失败", "err", err)
	}
	s := grpc.NewServer()
	pb.RegisterAgentWorkerServer(s, codeAgent)
	reflection.Register(s)

	tool.Log.Info("CoderAgent 微服务已启动", "port", ":50051", "workspace", workspacePath, "model", model, "baseURL", baseURL)
	go func() {
		if err := s.Serve(lis); err != nil {
			tool.Log.Fatal("Agent 停止运行", "err", err)
		}
	}()

	engine := flow.NewFlowEngine(eventStore)
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		tool.Log.Fatal("引擎无法连接 Agent", "err", err)
	}
	client := pb.NewAgentWorkerClient(conn)
	engine.RegisterAgent("coder_agent", client)

	time.Sleep(1 * time.Second)
	fmt.Println("======================================================")
	fmt.Println("🚀 ZgentFlow 多智能体交互终端已启动")
	fmt.Printf("📁 工作区: %s\n", workspacePath)
	fmt.Println("💡 提示: 已经支持 readline。按 ↑/↓ 查找历史记录，输入 'exit' 退出。")
	fmt.Println("======================================================")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[36mZgentFlow >\033[0m ",
		HistoryFile:     "/tmp/zgentflow_history.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		tool.Log.Fatal("初始化 Readline 失败", "err", err)
	}
	defer rl.Close()

	for {
		input, err := rl.Readline()
		if err != nil {
			fmt.Println("Bye!")
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.EqualFold(input, "exit") {
			fmt.Println("Bye!")
			break
		}

		workflowID := fmt.Sprintf("wf_%d", time.Now().Unix())
		tasks := []*flow.FlowTask{{
			TaskID:      "dynamic_task_1",
			AgentRole:   "coder_agent",
			Instruction: input,
			TimeoutMs:   120000,
			Retries:     1,
		}}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		tool.Log.Info("收到新指令，创建动态工作流", "workflowID", workflowID)

		err = engine.ExecuteWorkflow(ctx, workflowID, tasks)
		if err != nil {
			tool.Log.Error("任务执行失败", "err", err)
		} else {
			tool.Log.Info("任务流转完毕", "workspace", workspacePath)
		}

		cancel()
	}
}

func resolveWorkspacePath() string {
	workspacePath := "./zgent_workspace"
	if _, err := os.Stat(workspacePath); err == nil {
		return workspacePath
	}

	possiblePaths := []string{
		"/work/zgyx/ZgentFlow/zgent_workspace",
		"/tmp/zgent_workspace",
		"./workspace",
	}
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return workspacePath
}

func loadFirstEnvFile(paths ...string) error {
	for _, path := range paths {
		err := loadEnvFile(path)
		if err == nil {
			return nil
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return err
	}
	return nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
