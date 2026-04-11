package main

import (
	"context"
	"fmt"
	"net"
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

func main() {
	// 1. 启动基础设施
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

	executor, _ := tool.NewToolExecutor("./zgent_workspace")

	// 【新增】2. 组装技能背包 (Skill Engine 装配)
	skillManager := tool.NewSkillManager()
	skillManager.Register(&tool.SearchSkill{})             // 赐予网络
	skillManager.Register(tool.NewShellSkill(executor))    // 赐予双手
	skillManager.Register(tool.NewWriteFileSkill(executor)) // 赐予笔刷

	// 3. 模型配置 (建议使用 SiliconFlow 白嫖测试)
	apiKey := "sk-09f436f6ce604355b85438313ad752d6"
	baseURL := "https://api.deepseek.com" // 指向 DeepSeek 的服务器
	model := "deepseek-chat" // 这是他们免费提供的高性能模型

	// 【修改】注入的不再是 Executor，而是装配好的 skillManager
	codeAgent := agent.NewCodeAgent(apiKey, baseURL, model, qdrantStore, skillManager)

	// 4. 启动 Agent 微服务
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		tool.Log.Fatal("Agent 端口监听失败", "err", err)
	}
	s := grpc.NewServer()
	pb.RegisterAgentWorkerServer(s, codeAgent)
	reflection.Register(s)

	tool.Log.Info("CoderAgent 微服务已启动", "port", ":50051")
	go func() {
		if err := s.Serve(lis); err != nil {
			tool.Log.Fatal("Agent 停止运行", "err", err)
		}
	}()

	// 5. 初始化调度引擎并连接 Agent
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
	fmt.Println("💡 提示: 已经支持 readline。按 ↑/↓ 查找历史记录，输入 'exit' 退出。")
	fmt.Println("======================================================")

	// 6. Readline 交互核心
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
		if strings.ToLower(input) == "exit" {
			fmt.Println("Bye!")
			break
		}

		workflowID := fmt.Sprintf("wf_%d", time.Now().Unix())

		tasks := []*flow.FlowTask{
			{
				TaskID:       "dynamic_task_1",
				AgentRole:    "coder_agent",
				Instruction:  input,
				Dependencies: nil,
				TimeoutMs:    120000,
				Retries:      1,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)

		tool.Log.Info("收到新指令，创建动态工作流", "workflowID", workflowID)
		err = engine.ExecuteWorkflow(ctx, workflowID, tasks)

		if err != nil {
			tool.Log.Error("任务执行失败", "err", err)
		} else {
			tool.Log.Info("任务流转完毕", "workspace", "./zgent_workspace")
		}

		cancel()
	}
}