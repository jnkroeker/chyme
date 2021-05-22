package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-kit/kit/log/level"

	"github.com/go-kit/kit/log"
	"github.com/joho/godotenv"
)

// Globals
var (
	logger   log.Logger
	chConfig ChymeConfig
)

// Unified configuration for Chyme
type ChymeConfig struct {
	WorkerWorkDir           string
	WorkerDockerUser        string
	WorkerDockerPull        string
	WorkerDockerRemove      string
	RedisAddress            string
	RedisPassword           string
	ResourceSetKey          string
	TaskSetKey              string
	TaskQueueName           string
	TaskDeadLetterQueueName string
	TaskBatchSize           string
	VaultAddress            string
	VaultStaticToken        string // this value will change each time a new vault -dev server is created
	VaultStsSecret          string
}

func (c *ChymeConfig) String() string {
	str, err := json.MarshalIndent(c, "  ", "  ")
	if err != nil {
		str = []byte("error marshaling struct: " + err.Error())
	}
	return fmt.Sprintf("\n==> Chyme configuration:\n\n  %s\n", string(str))
}

func loadConfigFromEnv() ChymeConfig {
	return ChymeConfig{
		WorkerWorkDir:           os.Getenv("CH_WORKER_WORKDIR"),
		WorkerDockerUser:        os.Getenv("CH_WORKER_DOCKER_USER"),
		WorkerDockerPull:        os.Getenv("CH_WORKER_DOCKER_PULL"),
		WorkerDockerRemove:      os.Getenv("CH_WORKER_DOCKER_REMOVE"),
		RedisAddress:            os.Getenv("CH_REDIS_ADDR"),
		RedisPassword:           os.Getenv("CH_REDIS_PASSWORD"),
		ResourceSetKey:          os.Getenv("CH_RESOURCE_SET"),
		TaskSetKey:              os.Getenv("CH_TASK_SET"),
		TaskQueueName:           os.Getenv("CH_TASK_QUEUE"),
		TaskDeadLetterQueueName: os.Getenv("CH_TASK_DLQ"),
		TaskBatchSize:           os.Getenv("CH_TASK_BATCH_SIZE"),
		VaultAddress:            os.Getenv("CH_VAULT_ADDR"),
		VaultStaticToken:        os.Getenv("CH_VAULT_STATIC_TKN"),
		VaultStsSecret:          os.Getenv("CH_VAULT_STS_SECRET"),
	}
}

func main() {

	loglevel := level.AllowInfo()

	_ = godotenv.Load()

	chConfig = loadConfigFromEnv()

	logger = log.NewLogfmtLogger(os.Stdout)
	logger = level.NewFilter(logger, loglevel)
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)
	level.Debug(logger).Log("msg", "Chyme Wave System starting")

	_ = MainCmd.Execute()
}
