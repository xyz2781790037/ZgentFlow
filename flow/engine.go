package flow

import (
	pb "ZgentFlow/proto/workflow"
	"ZgentFlow/storage"
	"ZgentFlow/tool"
	"context"
	"fmt"
	"sync"
	"time"
)

type FlowTask struct {
	TaskID       string
	AgentRole    string   // 需要调用哪个 Agent (比如: coder_agent)
	Instruction  string   // 具体的 Prompt 指令
	Dependencies []string // 核心：前置依赖的任务 TaskID 列表 (用于构建 DAG)
	TimeoutMs    int64
	Retries      int32
}
type FlowEngine struct {
	eventStore *storage.EventStore
	agentConns map[string]pb.AgentWorkerClient // 路由表：AgentRole -> gRPC Client
	mu         sync.RWMutex
}

func NewFlowEngine(store *storage.EventStore) *FlowEngine {
	return &FlowEngine{
		eventStore: store,
		agentConns: make(map[string]pb.AgentWorkerClient),
	}
}
func (e *FlowEngine) RegisterAgent(role string, client pb.AgentWorkerClient) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.agentConns[role] = client
}
func (e *FlowEngine) ExecuteWorkflow(parentCtx context.Context, workflowID string, tasks []*FlowTask) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	inDegree := make(map[string]int)
	adjList := make(map[string][]string)
	taskMap := make(map[string]*FlowTask)

	for _, t := range tasks {
		taskMap[t.TaskID] = t
		inDegree[t.TaskID] = len(t.Dependencies)
		for _, dep := range t.Dependencies {
			adjList[dep] = append(adjList[dep], t.TaskID)
		}
	}
	_ = e.eventStore.AppendEvent(workflowID, pb.EventType_EVENT_WORKFLOW_STARTED, "", nil)
	taskCh := make(chan *FlowTask, len(tasks))
	resultCh := make(chan *pb.TaskResponse, len(tasks))
	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup
	var activeTasks int
	workerPoolSize := 5
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go e.workerLoop(ctx, &wg, workflowID, taskCh, resultCh, errCh)
	}
	for id, degree := range inDegree {
		if degree == 0 {
			activeTasks++
			taskCh <- taskMap[id]
		}
	}
	var workflowErr error
EventLoop:
	for activeTasks > 0 {
		select {
		case <-ctx.Done():
			workflowErr = ctx.Err()
			break EventLoop

		case err := <-errCh:
			workflowErr = err
			cancel()
			break EventLoop

		case res := <-resultCh:
			activeTasks--

			_ = e.eventStore.AppendEvent(workflowID, pb.EventType_EVENT_TASK_COMPLETED, res.TaskID, res.ResultData)

			if res.Status == pb.TaskStatus_STATUS_SUCCESS {
				fmt.Printf("\n\033[1;32m============== [Agent %s 结果] ==============\033[0m\n", res.TaskID)
				fmt.Printf("%s\n", string(res.ResultData))
				fmt.Printf("\033[1;32m====================================================\033[0m\n\n")
				for _, downStreamID := range adjList[res.TaskID] {
					inDegree[downStreamID]--
					if inDegree[downStreamID] == 0 {
						activeTasks++
						taskCh <- taskMap[downStreamID]
					}
				}
			} else {
				_ = e.eventStore.AppendEvent(workflowID, pb.EventType_EVENT_TASK_FAILED, res.TaskID, []byte(res.ErrorMessage))
				workflowErr = fmt.Errorf("task %s failed: %s", res.TaskID, res.ErrorMessage)
				cancel()
				break EventLoop
			}
		}
	}
	close(taskCh)
	wg.Wait()

	if workflowErr == nil {
		_ = e.eventStore.AppendEvent(workflowID, pb.EventType_EVENT_WORKFLOW_COMPLETED, "", nil)
	}

	return workflowErr
}
func (e *FlowEngine) workerLoop(ctx context.Context, wg *sync.WaitGroup, workflowID string, taskCh <-chan *FlowTask, resultCh chan<- *pb.TaskResponse, errCh chan<- error) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-taskCh:
			if !ok {
				return
			}
			_ = e.eventStore.AppendEvent(workflowID, pb.EventType_EVENT_TASK_SCHEDULED, task.TaskID, nil)
			e.mu.RLock()
			client, exists := e.agentConns[task.AgentRole]
			e.mu.RUnlock()
			if !exists {
				errCh <- fmt.Errorf("未找到对应的 Agent 微服务: %s", task.AgentRole)
				return
			}
			timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Duration(task.TimeoutMs)*time.Millisecond)
			req := &pb.TaskRequest{
				WorkflowID:  workflowID,
				TaskID:      task.TaskID,
				AgentRole:   task.AgentRole,
				Instruction: task.Instruction,
				RetryCount:  task.Retries,
				TimeoutMs:   task.TimeoutMs,
			}
			resp, err := client.ExecuteTask(timeoutCtx, req)
			timeoutCancel()

			if err != nil {
				tool.Log.Error("Agent 通信失败", "role", task.AgentRole, "err", err)
				errCh <- err
				return
			}
			select {
			case <-ctx.Done():
				return
			case resultCh <- resp:
			}
		}
	}
}
