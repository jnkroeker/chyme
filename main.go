package main

import (

	"strings"
	"errors"
	"os"
	"path/filepath"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"net/url"
	"encoding/json"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
	
	"github.com/hashicorp/vault/api"

	"github.com/joho/godotenv"
	
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	// "github.com/aws/aws-sdk-go/service/s3/s3manager"

	"github.com/go-redis/redis"

)

const (
	vaultAddr = "http://localhost:8200"
	vaultStaticToken = "s.FMtWzRspvkYIvNerpUVBwxg7" // this value will change each time a new vault -dev server is created
	vaultStsSecret = "aws/sts/assume_role_s3_sqs"
)


type IngestService interface {
	Ingest(*url.URL, string, int) (int64, error)
}

// the redis.Client is now in the resourceRepository
type Config struct {
	resourceRepository  ResourceRepository
	// redis			    *redis.Client
	resourceSetKey      string
	S3                  *s3.S3
}

type ingestService struct {
	Config
}

func (i *ingestService) Ingest(bucketUrl *url.URL, filterString string, recursionDepth int) (int64, error) {
	//send 'ext/pdf' to use NewExtFilter or 'identity/...' for other method
	filter, err := NewFilter(filterString)

	if recursionDepth > 0 {
		_, object := filepath.Split(bucketUrl.Path)

		if object == "" {
			//if there is no path we want to index an entire bucket
			res, err := i.ingestBucket(bucketUrl, filter, recursionDepth)
			if err != nil {
				return 0, err
			}
			return int64(res), nil
		}
	}

	filtered, err := filter(bucketUrl)
	if err != nil {
		return 0, err
	}
	res, err := i.resourceRepository.Add(i.resourceSetKey, filtered)
	if err != nil {
		return 0, ErrEmpty
	}
	return int64(res), nil
}

func (i *ingestService) ingestBucket(bucketUrl *url.URL, filter FilterFunc, depth int) (int, error) {
	bi, err := i.resourceRepository.BulkInsert(i.resourceSetKey) 
	
	/* TODO: &aws.ListObjectsOptions ?
		need to specify the path of the bucket (as in ingest ingestPrefix)
	 */
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketUrl.Host), //"aws-golang-vault"
		Prefix: aws.String(bucketUrl.Path),
	}

	result, err := i.S3.ListObjectsV2(input)

	// fmt.Println(result.String())

	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	for _, obj := range result.Contents {
		newResource, err := filter(&url.URL{
			Scheme: bucketUrl.Scheme, 
			Host: bucketUrl.Host,
			Path: *obj.Key,
		})

		if err != nil {
			return 0, err
		}

		ins_err := bi.Insert(newResource)

		if ins_err != nil {
			return 0, ins_err
		}
		// if err == nil {
		// 	err := bi.Insert(newResource)
		// 	if err != nil {
		// 		return 05, err
		// 	}
		// }
	}

	bi.Close()

	bucketObjectCount, err := i.resourceRepository.Count(i.resourceSetKey)
	if err != nil {
		return 0, err
	}
	return bucketObjectCount, nil
}

var ErrEmpty = errors.New("Got to SAdd but it errored out ")

// gRPC requests

type IngestRequest struct {
	URL            string `json:"url"`
	Filter         string `json:"filter"`
	RecursionDepth int    `json:"recursionDepth"`
}

type IngestResponse struct {
	RES int64 `json:"res"`
	Err string `json:"err,omitempty"`
}

func makeIngestEndpoint(svc IngestService) endpoint.Endpoint {
	return func (_ context.Context, request interface{}) (interface{}, error) {
		req := request.(IngestRequest)
		url, err := url.Parse(req.URL)

		if err != nil {
			return IngestResponse{0, err.Error()}, err
		}
		res, err := svc.Ingest(url, req.Filter, req.RecursionDepth)
		if err != nil {
			return IngestResponse{res, err.Error()}, nil
		}
		return IngestResponse{res, ""}, nil
	}
}

func decodeIngestRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func encodeIngestResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

func main() {
	//load environment from .env
	err := godotenv.Load()
	if err != nil {
		fmt.Println(err)
	}

	// svc := ingestService{}
	svc := buildService()

	ingestHandler := httptransport.NewServer(
		makeIngestEndpoint(svc),
		decodeIngestRequest,
		encodeIngestResponse,
	)

	http.Handle("/ingest", ingestHandler)
	http.ListenAndServe(":8080", nil)
}

// Service

func buildService() IngestService {
	awsSession := buildAwsSession()
	s3Client := getS3Service(awsSession)
	redisClient := getRedisClient()

	resourceRepository := getResourceRepository(redisClient)
	setKey             := os.Getenv("RESOURCE_SET_KEY")

	config := Config{
		resourceRepository: resourceRepository,
		resourceSetKey: setKey,
		S3: s3Client,
	}

	svc := &ingestService{
		config,
	}

	return svc
}

func buildAwsSession() *session.Session {
	config := &api.Config{
		Address: vaultAddr,
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil
	}
	client.SetToken(vaultStaticToken)
	c := client.Logical()
	options := map[string]interface{}{
		"ttl": "30m",
	}
	s, err := c.Write(vaultStsSecret, options)
	if err != nil {
		return nil
	}

	// pull relevant information from assumed role to create AWS session

	key := s.Data["access_key"].(string)
	secret := s.Data["secret_key"].(string)
	token := s.Data["security_token"].(string)

	creds := credentials.NewStaticCredentials(key, secret, token)

	sess := session.Must(session.NewSession(&aws.Config{
		Credentials: creds,
		MaxRetries: aws.Int(3),
		Region: aws.String("us-east-1"),
	}))

	return sess
}

func getS3Service(sess *session.Session) *s3.S3 {
	endpoint := "" // this should be an environment variable (look into use of github.com/joho/godotenv)
	return s3.New(sess, &aws.Config{
		Endpoint: aws.String(endpoint),
	})
}

func getRedisClient() *redis.Client {
	redisAddr := "localhost:6379" // should be environment variable
	redisPwd  := "" // should be environment variable, no password set on dev redis-server

	cli := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPwd,
		DB:       0,
	})

	return cli
}

/** Filters **/

type FilterFunc = func(resource *url.URL) (string, error)

type ResourceFilter struct {
	Description string
	Factory     func(args []string) (FilterFunc, error)
}

var FilterRegistry = map[string]ResourceFilter{
	// "identity": {"Applies no filter.", NewIdentityFilter},
	"ext":      {"Filters by file extension. Example: ext/txt", NewExtFilter},
}

func NewExtFilter(args []string) (FilterFunc, error) {
	// regex here looks for '.' speficifally and supplants %s with args[0], our extension
	// will accept anything before the '.'
	re, err := regexp.Compile(fmt.Sprintf(`^(.+)\.%s$`, args[0]))
	if err != nil {
		return nil, fmt.Errorf("extension regexp failed to compile: %s", err.Error())
	}
	return func(resource *url.URL) (string, error) {
		b, err := resource.MarshalBinary()
		if err != nil {
			return "error converting url to binary", err
		}
		if re.Match([]byte(b)) {
			return resource.String(), nil
		}
		return "", ErrPatternMatch
	}, nil
}

func NewFilter(filterString string) (FilterFunc, error) {
	split := strings.Split(filterString, "/")
	// use the first part of 'ext/pdf' find the right filter function in the map
	// the FilterRegistry maps a filter type (like extension) to functions to be performed
	filter, ok := FilterRegistry[split[0]] 
	if !ok {
		return nil, fmt.Errorf("invalid filter %s", split[0])
	}
	return filter.Factory(split[1:])
}

var ErrPatternMatch = errors.New("No match")