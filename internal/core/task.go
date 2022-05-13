package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	amzaws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-redis/redis"
	"kroekerlabs.dev/chyme/services/pkg/aws"
	"kroekerlabs.dev/chyme/services/pkg/hash"
)

// Represents a processing task.
type Task struct {
	sync.Mutex

	InputResource     *Resource          `json:"inputResource"`
	OutputResource    *Resource          `json:"outputResource"`
	MetadataResource  *Resource          `json:"metadataResource"`
	ExecutionStrategy *ExecutionStrategy `json:"executionStrategy"`
	Hooks             string             `json:"hooks"`
	Workspace         *TaskWorkspace     `json:"workspace"`
	Timeout           time.Duration      `json:"timeout"`
	Version           string             `json:"version"`

	isDeleted bool
	hash      string
}

// TaskWorkspace represents the filesystem location allocated for Task execution
type TaskWorkspace struct {
	InputDir    string
	OutputDir   string
	InternalDir string
}

func (t *Task) Hash() string {
	if t.hash == "" {
		t.hash = hash.Collate(t.InputResource, t.OutputResource).Hash()
	}

	return t.hash
}

/*
 *  TASK QUEUE
 */

type TaskMessage struct {
	Task          *Task
	MessageHandle interface{}
	Timeout       time.Time
}

func (m *TaskMessage) Handle() interface{} {
	return m.MessageHandle
}

// SQS-backed TaskQueue
type SqsTaskQueue struct {
	sqsQueue        *aws.SqsQueue
	deadLetterQueue *aws.SqsQueue
}

func NewSQSTaskQueue(backingQueue *aws.SqsQueue, deadLetterQueue *aws.SqsQueue) *SqsTaskQueue {
	return &SqsTaskQueue{backingQueue, deadLetterQueue}
}

// Enqueues a Task.
func (q *SqsTaskQueue) Enqueue(task *Task) error {
	return q.sqsQueue.Enqueue(task)
}

// Dequeues tasks.
func (q *SqsTaskQueue) Dequeue(max int) ([]*TaskMessage, error) {
	messages, err := q.sqsQueue.Dequeue(max)
	if err != nil {
		return nil, err
	}

	taskMessages := make([]*TaskMessage, 0)
	for _, message := range messages {
		var task Task
		if err := json.Unmarshal([]byte(*message.Body), &task); err != nil {
			continue
		}

		taskMessage := &TaskMessage{
			Task:          &task,
			MessageHandle: message.ReceiptHandle,
			Timeout:       time.Now().Add(time.Second * (aws.MaxVisbilityTimeoutSeconds - 10)),
		}

		taskMessages = append(taskMessages, taskMessage)
	}

	return taskMessages, nil
}

// Deletes a task from the Queue. This method is idempotent.
func (q *SqsTaskQueue) Delete(message *TaskMessage) error {
	message.Task.Lock()
	defer message.Task.Unlock()

	if message.Task.isDeleted {
		return nil
	}

	if err := q.sqsQueue.Delete(message); err != nil {
		return err
	}
	message.Task.isDeleted = true
	return nil
}

// Marks a task as failed by moving it to the DLQ.
func (q *SqsTaskQueue) Fail(message *TaskMessage, err error) error {
	if err := q.sqsQueue.Delete(message); err != nil {
		return err
	}

	return q.deadLetterQueue.EnqueueWithAttrs(message.Task, map[string]*sqs.MessageAttributeValue{
		"Error": {
			DataType:    amzaws.String("String"),
			StringValue: amzaws.String(err.Error()),
		},
		"Hash": {
			DataType:    amzaws.String("String"),
			StringValue: amzaws.String(message.Task.Hash()),
		},
	})
}

func (q *SqsTaskQueue) MessageCount() (int, error) {
	return q.sqsQueue.MessageCount()
}

/*
 * TASK REPOSITORY
 */

type TaskRepository interface {
	Add(task *Task) error
	Remove(task *Task) error
}

type redisTaskRepository struct {
	client *redis.Client
	setKey string
}

func NewRedisTaskRepository(client *redis.Client, setKey string) TaskRepository {
	return &redisTaskRepository{client, setKey}
}

func (r *redisTaskRepository) Add(task *Task) (err error) {
	_, err = r.client.SAdd(r.setKey, task.Hash()).Result()
	return
}

func (r *redisTaskRepository) Remove(task *Task) (err error) {
	_, err = r.client.SRem(r.setKey, task.Hash()).Result()
	return
}

/*
 * TASK LOADER
 */

// Represents a type capable of downloading and uploading the source files and artifacts of a Task.
type TaskLoader interface {
	CreateWorkspace(task *Task) error
	CheckCapacity(task *Task, scaleFactor uint64) (bool, error)
	Download(ctx context.Context, task *Task) error
	Upload(ctx context.Context, task *Task, filePath string) error
	UploadMetadata(task *Task, metadata *ExecutionResult) error
	// WatchUpload(task *Task, doneCh <-chan bool) error
	Clean(task *Task) error
}

type taskLoader struct {
	loader  ResourceLoader
	workDir string
}

func NewTaskLoader(loader ResourceLoader, workDir string) TaskLoader {
	return &taskLoader{loader, workDir}
}

func (l *taskLoader) CreateWorkspace(task *Task) error {
	taskDir := filepath.Join(l.workDir, task.Hash())
	inDir := filepath.Join(taskDir, "input")
	outDir := filepath.Join(taskDir, "output")
	internalDir := filepath.Join(taskDir, "internal")

	for _, dir := range []string{inDir, outDir, internalDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	task.Workspace = &TaskWorkspace{
		InputDir:    inDir,
		OutputDir:   outDir,
		InternalDir: internalDir,
	}

	return nil
}

func (l *taskLoader) CheckCapacity(task *Task, scaleFactor uint64) (bool, error) {
	return l.loader.CheckCapacityPosix(task.InputResource, task.Workspace.InputDir, scaleFactor)
}

// Downloads a Task's source resource.
func (l *taskLoader) Download(ctx context.Context, task *Task) (err error) {
	if err := removeContents(task.Workspace.InputDir, 0700); err != nil {
		return err
	}
	_, err = l.loader.Download(ctx, task.InputResource, task.Workspace.InputDir)
	return
}

// Uploads a task's artifacts.
func (l *taskLoader) Upload(ctx context.Context, task *Task, filePath string) (err error) {
	if filePath == "" {
		return errors.New("empty filepath")
	}
	_, err = l.loader.Upload(ctx, task.OutputResource, filePath, map[string]*string{}, true)
	return
}

func (l *taskLoader) UploadMetadata(task *Task, metadata *ExecutionResult) error {
	if task.MetadataResource == nil || metadata == nil || metadata.MetadataPaths == nil {
		return nil
	}

	for name, filePath := range metadata.MetadataPaths {
		r := *task.MetadataResource
		r.Url.Path = path.Join(r.Url.Path, task.Hash(), name)
		if _, err := l.loader.Upload(context.Background(), &r, filePath, map[string]*string{}, true); err != nil {
			return err
		}
	}
	return nil
}

// Frees any resources used by the loader.
func (l *taskLoader) Clean(task *Task) error {
	return os.RemoveAll(filepath.Join(l.workDir, task.Hash()))
}

// The laziest possible way to remove a directory's contents.
func removeContents(path string, mode os.FileMode) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, mode)
}
