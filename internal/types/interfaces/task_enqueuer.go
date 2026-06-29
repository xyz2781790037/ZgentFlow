package interfaces

import "github.com/hibiken/asynq"

// TaskEnqueuer abstracts Redis-backed task enqueueing. *asynq.Client satisfies this interface.
type TaskEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
