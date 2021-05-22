package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/spf13/cobra"
	"kroekerlabs.dev/chyme/services/internal/ingest"
)

func init() {
	ingestCmd.Flags().IntVarP(&recursionDepth, "recursion", "r", 0, "ingest recusion depth")
	ingestCmd.Flags().StringVarP(&filter, "filter", "f", "", "file type filter")

	indexCommand.AddCommand(ingestStartCmd)
	indexCommand.AddCommand(ingestCmd)

	MainCmd.AddCommand(indexCommand)
}

var indexCommand = &cobra.Command{
	Use:   "indexer",
	Short: "Chyme s3 indexing service",
}

var (
	recursionDepth int
	filter         string
)

var ingestStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start listening at /ingest for ingest service requests",
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("start ingest")
		level.Debug(logger).Log("cmd", "start")
		logger := log.With(logger, "svc", "ingest")

		svc := buildService(logger)

		ingestHandler := httptransport.NewServer(
			ingest.MakeIngestEndpoint(svc),
			ingest.DecodeIngestRequest,
			ingest.EncodeIngestResponse,
		)

		http.Handle("/ingest", ingestHandler)
		level.Info(logger).Log("msg", "Listening", "transport", "http")
		// TODO: make :8080 an env variable in chConfig: 'ingestListenPort'
		http.ListenAndServe(":8080", nil)
	},
}

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "ingest an S3 bucket to redis",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		level.Debug(logger).Log("cmd", "ingest", "url", args[0])

		req := &ingest.IngestRequest{
			URL:            args[0],
			Filter:         filter,
			RecursionDepth: recursionDepth,
		}

		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(req)

		res, err := http.Post("http://localhost:8080/ingest", "application/json", &buf)
		if err != nil {
			errors.New("error making ingest request: " + err.Error())
		}
		if res.StatusCode != 200 {
			errors.New("response not ok: " + res.Status)
		}

		var ingestResponse ingest.IngestResponse
		json.NewDecoder(res.Body).Decode(&ingestResponse)
		if ingestResponse.Err != "" {
			errors.New("ingest failed: " + ingestResponse.Err)
		}

		fmt.Println("Ingest Success")
	},
}

func buildService(logger log.Logger) ingest.IngestService {
	awsSession := buildAwsSession()
	s3Client := getS3Service(awsSession)
	redisClient := getRedisClient()

	resourceRepository := getResourceRepository(redisClient)
	// TODO: create logging resource repository
	// resourceRepository = core.NewLoggingResourceRepository(resourceRepository, logger)

	setKey := chConfig.ResourceSetKey //os.Getenv("RESOURCE_SET_KEY")

	svc := ingest.New(ingest.Config{
		ResourceRepository: resourceRepository,
		ResourceSetKey:     setKey,
		S3:                 s3Client,
	})

	return svc
}
