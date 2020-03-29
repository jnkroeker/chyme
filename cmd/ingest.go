package cmd

import (
	"os"
	"fmt"
	"bytes"
	"errors"
	"net/http"
	"encoding/json"
	"github.com/spf13/cobra"
	"github.com/joho/godotenv"
	"chyme/internal/ingest"
	httptransport "github.com/go-kit/kit/transport/http"
)

func init() {
	ingestCmd.Flags().IntVarP(&recursionDepth, "recursion", "r", 0, "ingest recusion depth")
	ingestCmd.Flags().StringVarP(&filter, "filter", "f", "", "file type filter")

	MainCmd.AddCommand(ingestStartCmd)
	MainCmd.AddCommand(ingestCmd)
}

var (
	recursionDepth int
	filter         string
)

var ingestStartCmd = &cobra.Command{
	Use: "start",
	Short: "start listening at /ingest for ingest service requests",
	Run: func(cmd *cobra.Command, args []string) {
		//load environment from .env
		err := godotenv.Load()
		if err != nil {
			fmt.Println(err)
		}

		svc := buildService()

		ingestHandler := httptransport.NewServer(
			ingest.MakeIngestEndpoint(svc),
			ingest.DecodeIngestRequest,
			ingest.EncodeIngestResponse,
		)

		http.Handle("/ingest", ingestHandler)
		http.ListenAndServe(":8080", nil)
	},
}

var ingestCmd = &cobra.Command{
	Use: "ingest",
	Short: "ingest an S3 bucket to redis",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args [] string) {

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

func buildService() ingest.IngestService {
	awsSession := buildAwsSession()
	s3Client := getS3Service(awsSession)
	redisClient := getRedisClient()

	resourceRepository := getResourceRepository(redisClient)
	setKey             := os.Getenv("RESOURCE_SET_KEY")

	svc := ingest.New(ingest.Config{
		ResourceRepository: resourceRepository,
		ResourceSetKey: setKey,
		S3: s3Client,
	});

	return svc
}
