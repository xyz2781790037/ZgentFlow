package service

import (
	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/config"
)

func documentProcessTaskOptions(cfg *config.Config, extra ...asynq.Option) []asynq.Option {
	opts := []asynq.Option{
		asynq.Queue("default"),
		asynq.Timeout(config.DocumentProcessTimeout(cfg)),
	}
	opts = append(opts, extra...)
	return opts
}
