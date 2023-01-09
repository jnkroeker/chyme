package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/internal/tasker"
	"kroekerlabs.dev/chyme/services/internal/tasker/template"
)

func init() {
	taskerCommand.AddCommand(taskerStartCmd)

	MainCmd.AddCommand(taskerCommand)
}

var taskerCommand = &cobra.Command{
	Use:   "tasker",
	Short: "Chyme task management service",
}

var taskerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Tasker service.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("starting tasker")
		redis := getRedisClient()
		resourceRepository := getResourceRepository(redis)

		taskRepository := core.NewRedisTaskRepository(redis, chConfig.TaskSetKey)

		sess := buildAwsSession()

		sqs := getSQSService(sess)
		sqsQueue := getSQSQueue(sqs, chConfig.TaskQueueName)
		dlq := getSQSQueue(sqs, chConfig.TaskDeadLetterQueueName)
		taskQueue := core.NewSQSTaskQueue(sqsQueue, dlq)

		templater := buildTemplater()

		batchSize, err := strconv.Atoi(chConfig.TaskBatchSize)
		if err != nil {
			fmt.Println("Fatal: " + err.Error())
			os.Exit(1)
		}

		svc := tasker.New(&tasker.Config{
			ResourceSetKey:     chConfig.ResourceSetKey,
			ResourceRepository: resourceRepository,
			TaskRepository:     taskRepository,
			TaskQueue:          taskQueue,
			Templater:          templater,
			BatchSize:          batchSize,
		})

		/*
		 * GOLANG CHANNELS
		 */

		// read only channel
		doneCh := make(chan bool)
		// channel capacity 1
		sigCh := make(chan os.Signal, 1)
		// causes package signal to relay incoming signals to channel
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		/*
		 * GOROUTINE
		 */
		go doneOnSignal(doneCh, sigCh)

		/*
		 * SELECT STATEMENT
		 * lets a goroutine wait on multiple communication options
		 * the select statement blocks until one of its cases can run
		 */

		// TODO: make ticker interval configurable
		ticker := time.NewTicker(time.Second * 30)
		for {
			select {
			case <-doneCh:
				fmt.Println("done")
				ticker.Stop()
				return
			case <-ticker.C:
				fmt.Println("tick...")
				if err := svc.Poll(); err != nil {
					fmt.Println(fmt.Errorf("error: %s", err))
				}
			}
		}
	},
}

func buildTemplater() tasker.Templater {
	// Register additional templates here.
	return tasker.NewInMemTemplater([]*tasker.Template{
		// template.Mie4NitfV2,
		template.Mov,
		template.Mp4,
		// template.MEI4NITFChunked,
		// template.MP2TS,
		// template.MEI4NITFBreadcrumb,
	}, "0.0.1")
}
