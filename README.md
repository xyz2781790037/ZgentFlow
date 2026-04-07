# ZgentFlow
### workflow 
我们正是把 Temporal 的“事件溯源”和 Argo 的“DAG 并发”融合在一起！
架构亮点剖析（面试直接拿去讲）：
``` bash
    死锁免疫（Deadlock Immunity）：
    注意 taskCh 和 resultCh 的容量是 len(tasks)。很多新手写 Channel 容易死锁，就是因为队列满了导致发送方阻塞。我们将缓冲池设为最大任务数，从物理上隔绝了阻塞死锁的可能。

    上下文级联取消（Context Cascading Cancellation）：
    ctx, cancel := context.WithCancel(parentCtx) 是 Go 语言并发编程的精髓。只要图里任何一个 Agent 发生严重错误（触发 errCh），主循环立刻调用 cancel()。瞬间，所有还在排队的、或者正在通过 gRPC 调用的 Worker 协程，都会收到 <-ctx.Done() 信号并优雅退出，绝不浪费 CPU 和大模型 API 的钱。

    完全的状态机解耦：
    引擎本身完全不关心大模型说了什么。它只看 res.Status，并死板地推进 inDegree 减 1 的数学图论逻辑。这就是控制面（Flow）与数据面（Agent）的完美隔离。
```