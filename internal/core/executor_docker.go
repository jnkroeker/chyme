package core

import (
	"context"
	"errors"
	"fmt"
	// "io"
	// "os"
	// "path/filepath"
	"strings"
	"time"

	"docker.io/go-docker"
	"docker.io/go-docker/api/types"
	"docker.io/go-docker/api/types/container"
)

// Docker-backed TaskExecutor.
type dockerTaskExecutor struct {
	client       *docker.Client
	user         string
	shouldPull   bool
	shouldRemove bool
}

func NewDockerTaskExecutor(cli *docker.Client, user string, shouldPull bool, shouldRemove bool) TaskExecutor {
	return &dockerTaskExecutor{cli, user, shouldPull, shouldRemove}
}

func (e *dockerTaskExecutor) Name() string {
	return "docker"
}

// Processes a task using a Docker container.
func (e *dockerTaskExecutor) Execute(ctx context.Context, task *Task) (*ExecutionResult, error) {
	if task.ExecutionStrategy.Executor != e.Name() {
		return nil, fmt.Errorf("invalid executor (%s) should be %s", task.ExecutionStrategy.Executor, e.Name())
	}
	image := task.ExecutionStrategy.Config["image"]
	if image == "" {
		return nil, errors.New("invalid configuration: no image specified")
	}

	var (
		timeoutTimer *time.Timer
		timeoutChan  = make(<-chan time.Time)
	)
	if task.Timeout != 0 {
		timeoutTimer = time.NewTimer(task.Timeout)
		timeoutChan = timeoutTimer.C
	}

	localCtx := context.Background()

	containerID, err := e.containerIDForTask(task)
	if err != nil {
		return nil, err
	}

	if containerID == "" {
		if e.shouldPull {
			if err := e.pullImage(localCtx, image); err != nil {
				return nil, err
			}
		}

		resp, err := e.makeContainer(localCtx, image, task)
		// fmt.Println("make container response: ")
		// fmt.Println(resp)
		if err != nil {
			return nil, err
		}
		if err := e.client.ContainerStart(localCtx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return nil, err
		}
		containerID = resp.ID
	}

	var execErr error
	statusCh, errCh := e.client.ContainerWait(localCtx, containerID, container.WaitConditionNotRunning)
	select {
	case <-timeoutChan:
		if killErr := e.killContainer(ctx, containerID, "SIGKILL"); killErr != nil {
			err = fmt.Errorf("exceeded timeout (%s) and failed to kill container: %s", task.Timeout.String(), killErr.Error())
		} else {
			err = fmt.Errorf("exceeded timeout (%s), container killed", task.Timeout.String())
		}
	case <-ctx.Done():
		err = ctx.Err()
	case e := <-errCh:
		err = e
	case status := <-statusCh:
		if status.StatusCode != 0 {
			execErr = fmt.Errorf("container returned non-zero status %d", status.StatusCode)
		}
	}

	if timeoutTimer != nil {
		timeoutTimer.Stop()
	}

	if err != nil {
		return nil, err
	}

	return e.makeResult(task, execErr)
}

func (e *dockerTaskExecutor) Clean(task *Task) error {
	if !e.shouldRemove {
		return nil
	}

	id, err := e.containerIDForTask(task)
	if err != nil {
		return err
	}
	if id == "" {
		return errors.New("container not found for task")
	}
	return e.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
}

func (e *dockerTaskExecutor) makeResult(task *Task, execErr error) (*ExecutionResult, error) {
	id, err := e.containerIDForTask(task)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, errors.New("container not found for task")
	}

	result := &ExecutionResult{
		Err:           execErr,
		OutputPath:    task.Workspace.OutputDir, //filepath.Join(task.Workspace.OutputDir, "upload")
		MetadataPaths: make(map[string]string),
	}

	// logName, logPath, err := e.writeLogs(id, task)
	// if err == nil {
	// 	result.MetadataPaths[logName] = logPath
	// }

	return result, err
}

func (e *dockerTaskExecutor) containerIDForTask(task *Task) (string, error) {
	list, err := e.client.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return "", err
	}

	// fmt.Println("list containers on client")
	// fmt.Println(list)

	// Is the image we are looking for on the local docker host? print them out below
	// images, err := e.client.ImageList(context.Background(), types.ImageListOptions{All: true})
	// if err != nil {
	// 	return "", err
	// }
	// fmt.Println("list images on client")
	// fmt.Println(images)

	for _, cntnr := range list {
		for _, name := range cntnr.Names {
			if strings.TrimLeft(name, "/") == task.Hash() {
				return cntnr.ID, nil
			}
		}
	}
	return "", nil
}

func (e *dockerTaskExecutor) pullImage(ctx context.Context, image string) error {
	fmt.Println("pull image: " + image)
	out, err := e.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		fmt.Println("error pulling image")
		return err
	}
	return out.Close()
}

func (e *dockerTaskExecutor) makeContainer(ctx context.Context, image string, task *Task) (container.ContainerCreateCreatedBody, error) {
	// define the volume bindings for the container
	in :=  "/" + task.Workspace.InputDir + ":/in"
	out := "/" + task.Workspace.OutputDir + ":/out"

	env := make([]string, 0)
	if envStr := task.ExecutionStrategy.Config["env"]; envStr != "" {
		env = e.envStrToSlice(envStr)
	}

	// User is the user that will run the commands inside the container: this user needs to exist on the container
	return e.client.ContainerCreate(ctx, &container.Config{
		Image:        image,
		User:         e.user,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          env,
	}, &container.HostConfig{
		Binds: []string{in, out},
	}, nil, task.Hash())
}

func (e *dockerTaskExecutor) killContainer(ctx context.Context, containerID string, signal string) error {
	return e.client.ContainerKill(ctx, containerID, signal)
}

// func (e *dockerTaskExecutor) writeLogs(id string, task *Task) (string, string, error) {
// 	logPath := filepath.Join(task.Workspace.InternalDir, "container-logs.txt")
// 	logs, err := e.client.ContainerLogs(context.Background(), id, types.ContainerLogsOptions{
// 		ShowStderr: true,
// 		ShowStdout: true,
// 	})
// 	if err != nil {
// 		return "", "", err
// 	}
// 	defer logs.Close()
// 	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0600)
// 	if err != nil {
// 		return "", "", err
// 	}
// 	defer f.Close()
// 	_, err = io.Copy(f, logs)
// 	return "docker-logs.txt", logPath, nil
// }

func (e *dockerTaskExecutor) envStrToSlice(envStr string) []string {
	env := make([]string, 0)
	if envStr == "" {
		return env
	}
	for _, e := range strings.Split(envStr, "\n") {
		env = append(env, e)
	}
	return env
}
