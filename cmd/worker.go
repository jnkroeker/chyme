package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/internal/worker"
	"kroekerlabs.dev/chyme/services/internal/worker/hooks"
)

func init() {
	workerCmd.AddCommand(workerStartCmd)

	MainCmd.AddCommand(workerCmd)
}

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Chyme task execution service",
}

var workerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Worker service.",
	Run: func(_ *cobra.Command, args []string) {

		sess := buildAwsSession()

		/* TODO: might need new sts role for executing tasks in docker?
		 *       look into using sts from aws-go-sdk rather than vault?
		 */
		s3Client := getS3Service(sess)

		// Resource loader stuff

		var defaultMetadata map[string]*string

		s3Loader := core.NewS3ResourceLoader(s3Client, defaultMetadata)
		resourceLoader := core.NewResourceLoader(map[string]core.ResourceLoader{s3Loader.Scheme(): s3Loader})

		workdir := filepath.Join(chConfig.WorkerWorkDir, "chyme")

		taskLoader := core.NewTaskLoader(resourceLoader, workdir)

		// SQS queue stuff

		sqs := getSQSService(sess)
		sqsQueue := getSQSQueue(sqs, chConfig.TaskQueueName)
		dlq := getSQSQueue(sqs, chConfig.TaskDeadLetterQueueName)
		taskQueue := core.NewSQSTaskQueue(sqsQueue, dlq)

		// Docker stuff

		dockerClient := getDockerClient()

		// Make executors

		dockerTaskExecutor := core.NewDockerTaskExecutor(
			dockerClient,
			chConfig.WorkerDockerUser,
			parseBoolOption(chConfig.WorkerDockerPull, false),
			parseBoolOption(chConfig.WorkerDockerRemove, true),
		)
		// inMemExecutor := core.NewInMemTaskExecutor(map[string]core.InmemExecutable{
		//	"BreadcrumbUpdate": &executable.BreadcrumbUpdate{S3: s3Client}
		// }, logger)
		taskExecutor := core.NewTaskExecutor(map[string]core.TaskExecutor{
			dockerTaskExecutor.Name(): dockerTaskExecutor,
			// inMemExecutor.Name():      inMemExecutor,
		})

		persister := worker.NewFSPersister(workdir)

		// Create the service

		svc := worker.New(
			taskQueue,
			taskLoader,
			taskExecutor,
			map[string]hooks.Interface{
				"mov": &hooks.MOV{
					TaskLoader:     taskLoader,
					ResourceLoader: resourceLoader,
				},
			},
			persister,
			"0.0.1",
		)

		// Channels

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		ctx, cancel := context.WithCancel(context.Background())
		go cancelOnSignal(cancel, sigCh)

		errCh := make(chan error)
		go func() {
			err := <-errCh
			fmt.Println(fmt.Errorf("SEVERE: unrecoverable error while processing Task: %s", err))
		}()

		// THE GOOD STUFF (svc.Poll)

		for {
			if err := ctx.Err(); err != nil {
				os.Exit(0)
			}

			if err := svc.Poll(ctx, errCh); err != nil {
				if err != nil {
					time.Sleep(time.Second * 10)
				}
			}
		}

	},
}

func parseBoolOption(opt string, defaultTo bool) bool {
	if opt == "" {
		return defaultTo
	}

	p, err := strconv.ParseBool(opt)
	CheckFatal(err)
	return p
}

func cancelOnSignal(f context.CancelFunc, sigCh <-chan os.Signal) {
	sig := <-sigCh
	fmt.Println(fmt.Errorf("caught signal, cancelling processing.: %s", sig.String()))
	// level.Info(logger).Log("msg", "Caught signal, cancelling processing.", "signal", sig.String())
	f()
}
