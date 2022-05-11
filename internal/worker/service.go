package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/internal/worker/hooks"
)

type Service struct {
	TaskQueue    core.TaskQueue
	TaskLoader   core.TaskLoader
	TaskExecutor core.TaskExecutor
	Hooks        hooks.Registry
	Persister    Persister
	Version      string
	// Concurrency  int
	sync.Mutex
	inProcess              map[string]*core.Task
	inProcNotificationChan chan int
}

// Return a pointer becasue we have a mutex involved. It is never safe to copy (value semantics) a mutex
func New(q core.TaskQueue, tl core.TaskLoader, te core.TaskExecutor, hooks hooks.Registry, p Persister, v string) *Service {
	inProcess := make(map[string]*core.Task)

	// TODO: this channel was left out of the original New() method
	// There is a method to check if this channel exists below
	// what is its significance?
	inProcNotificationChan := make(chan int)
	return &Service{
		q,
		tl,
		te,
		hooks,
		p,
		v,
		sync.Mutex{},
		inProcess,
		inProcNotificationChan,
	}
}

// Polls the task queue and processes received Tasks.
func (s *Service) Poll(ctx context.Context, processErrCh chan error) error {
	// nInProc := len(s.InProcess())
	// for nInProc >= s.Concurrency {
	// 	nInProc = <-s.requestInprocNotification()
	// }

	messages, err := s.TaskQueue.Dequeue(1)
	if err != nil {
		return err
	}

	for _, message := range messages {
		fmt.Println("message pulled from task queue")
		s.setInProcess(message.Task)
		go func(msg *core.TaskMessage) {
			if err := s.processMessage(ctx, msg, Start); err != nil {
				processErrCh <- err
			}
			s.clearInProcess(msg.Task)
		}(message)
	}

	return nil
}

func (s *Service) setInProcess(task *core.Task) {
	s.Lock()
	defer s.Unlock()

	s.inProcess[task.Hash()] = task
	s.inProcNotify(len(s.inProcess))
}

func (s *Service) clearInProcess(task *core.Task) {
	s.Lock()
	defer s.Unlock()

	delete(s.inProcess, task.Hash())
	s.inProcNotify(len(s.inProcess))
}

func (s *Service) processMessage(ctx context.Context, message *core.TaskMessage, stage ProcessStage) error {
	// Resolve the hooks specified for the Task
	taskHooks, ok := s.Hooks[message.Task.Hooks]
	if !ok {
		return s.TaskQueue.Fail(message, fmt.Errorf("unknown task hooks %s", message.Task.Hooks))
	}

	// Delete this message from the queue if we exceed its timeout while processing.
	// If timeout already exceeded, then a different machine will pick it up
	untilTimeout := time.Until(message.Timeout)
	if untilTimeout < time.Second*10 {
		return nil
	}
	timeout := time.AfterFunc(untilTimeout, func() { s.TaskQueue.Delete(message) })

	// Process the Task and persist state if Process errors due to the provided context being canceled
	currentStage, err := s.Process(ctx, message.Task, taskHooks, stage)
	timeout.Stop()
	if isCtxCanceled(err) {
		return s.Persister.Persist(&State{currentStage, message, s.Version})
	}
	errs := &multierror.Error{}
	errs = multierror.Append(errs, err)

	// Clean up any resources used by this Task
	errs = multierror.Append(errs, s.TaskLoader.Clean(message.Task))
	errs = multierror.Append(errs, s.TaskExecutor.Clean(message.Task))

	if errs.ErrorOrNil() != nil {
		return s.TaskQueue.Fail(message, errs)
	}

	return s.TaskQueue.Delete(message)
}

type ProcessStage string

const (
	Start    ProcessStage = "start"
	Download ProcessStage = "download"
	Execute  ProcessStage = "execute"
	Metadata ProcessStage = "metadata"
	Upload   ProcessStage = "upload"
	Complete ProcessStage = "complete"
)

// Downloads, executes and uploads a single Task.
func (s *Service) Process(ctx context.Context, task *core.Task, taskHooks hooks.Interface, stage ProcessStage) (ProcessStage, error) {
	result := &core.ExecutionResult{}
	execErr := &multierror.Error{}

	switch stage {
	case Start:
		if err := s.TaskLoader.CreateWorkspace(task); err != nil {
			return Start, fmt.Errorf("failed to create workspace: %s", err.Error())
		}
		fallthrough
	case Download:
		if err := taskHooks.PreDownload(ctx, task); err != nil {
			return Download, fmt.Errorf("during pre-download hook: %s", err.Error())
		}
		if err := s.TaskLoader.Download(ctx, task); err != nil {
			return Download, fmt.Errorf("during download: %s", err.Error())
		}
		fallthrough
	case Execute:
		if err := taskHooks.PreExecute(ctx, task); err != nil {
			return Execute, fmt.Errorf("during pre-execute hook: %s", err.Error())
		}
		res, err := s.TaskExecutor.Execute(ctx, task)
		if err != nil {
			fmt.Println("Fatal: " + err.Error())
		}
		if isCtxCanceled(err) {
			return Execute, err
		}
		result = res
		execErr = multierror.Append(execErr, err)        // Error from Tsunami infrastructure
		execErr = multierror.Append(execErr, result.Err) // Error from the client code being executed (e.g. container)
		execErr = multierror.Append(execErr, s.TaskLoader.UploadMetadata(task, result))
		if err := execErr.ErrorOrNil(); err != nil {
			return Metadata, fmt.Errorf("error(s) during execution: %s", err.Error())
		}
		fallthrough
	case Upload:
		if err := taskHooks.PreUpload(ctx, task); err != nil {
			return Upload, fmt.Errorf("during pre-upload hook: %s", err.Error())
		}
		if err := s.TaskLoader.Upload(ctx, task, result.OutputPath); err != nil {
			return Upload, fmt.Errorf("failed to upload task output: %s", err.Error())
		}
		if err := taskHooks.PostUpload(ctx, task); err != nil {
			return Upload, fmt.Errorf("during post-upload hook: %s", err.Error())
		}
	default:
		return Start, fmt.Errorf("invalid process stage %s", stage)
	}

	return Complete, nil
}

// TODO: Make this return something that is not a pointer to this service's internal state.
func (s *Service) InProcess() []*core.Task {
	s.Lock()
	defer s.Unlock()
	return taskMapToSlice(s.inProcess)
}

// inProcNotify is called from a worker goroutine to notify the main goroutine when the number of in process tasks
// changes.
func (s *Service) inProcNotify(length int) {
	if s.inProcNotificationChan != nil {
		s.inProcNotificationChan <- length
		s.inProcNotificationChan = nil
	}
}

func isCtxCanceled(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "context canceled")
}

func taskMapToSlice(m map[string]*core.Task) []*core.Task {
	tasks := make([]*core.Task, 0)
	for _, v := range m {
		tasks = append(tasks, v)
	}
	return tasks
}
