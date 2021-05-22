package core

import (
	"context"
	"encoding/json"
	"fmt"

	"kroekerlabs.dev/chyme/services/pkg/hash"
)

// Represents a type capable of executing a Task.
type TaskExecutor interface {
	Execute(ctx context.Context, task *Task) (*ExecutionResult, error)
	Clean(task *Task) error
	Name() string
}

type ExecutionResult struct {
	Err           error
	OutputPath    string
	MetadataPaths map[string]string
}

// Identifies and configures the Executor to be used for a Task.
type ExecutionStrategy struct {
	Executor string            `json:"name"`
	Config   map[string]string `json:"config"`
	hash     string
}

func (s *ExecutionStrategy) Hash() string {
	if s.hash == "" {
		hasher := hash.NewStruct()
		_ = hasher.Encode(struct {
			Executor string
			Config   [][2]string
		}{s.Executor, mapToSortedTuples(s.Config)})
		s.hash = hasher.Hash()
	}
	return s.hash
}

func (s *ExecutionStrategy) String() string {
	config, err := json.MarshalIndent(s.Config, "  ", "  ")
	if err != nil {
		config = []byte("(failed to marshal config map)")
	}
	return "Executor: " + s.Executor + "\nConfig:\n  " + string(config)
}

type ExecutorRegistry map[string]TaskExecutor

type taskExecutor struct {
	registry ExecutorRegistry
}

// Creates a new TaskExecutor that delegates execution to the Executor named in the ExecutionStrategy.
func NewTaskExecutor(registry ExecutorRegistry) TaskExecutor {
	return &taskExecutor{registry}
}

func (e *taskExecutor) Name() string {
	return ""
}

func (e *taskExecutor) Execute(ctx context.Context, task *Task) (*ExecutionResult, error) {
	executor := e.registry[task.ExecutionStrategy.Executor]
	if executor == nil {
		return nil, fmt.Errorf("unknown executor %s", task.ExecutionStrategy.Executor)
	}
	return executor.Execute(ctx, task)
}

func (e *taskExecutor) Clean(task *Task) error {
	executor := e.registry[task.ExecutionStrategy.Executor]
	if executor == nil {
		return fmt.Errorf("unknown executor %s", task.ExecutionStrategy.Executor)
	}
	return executor.Clean(task)
}