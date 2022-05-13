package main

import (
	"fmt"
	"os"

	"docker.io/go-docker"
	amzaws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-redis/redis"
	"github.com/hashicorp/vault/api"
	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/pkg/aws"
)

/* Resource Builders */

// must return a pointer here, there can not be a copy of an aws session
// aws sdk returns a pointer from session.Must
func buildAwsSession() *session.Session {

	// build a new Vault client
	client, err := api.NewClient(&api.Config{
		Address: chConfig.VaultAddress,
	})

	if err != nil {
		fmt.Println("New Vault Client Fatal: " + err.Error())
		return nil
	}

	// setting the token on the Vault client prepares it to handle requests
	// client.Logical() returns a logical-backend for API calls
	client.SetToken(chConfig.VaultStaticToken)
	c := client.Logical()

	// empty interface required by call to Vault API c.Write
	options := map[string]interface{}{
		"ttl": "30m",
	}

	s, err := c.Write(chConfig.VaultStsSecret, options)
	if err != nil {
		fmt.Println("Write Secret Fatal: " + err.Error())
		return nil
	}

	// pull relevant information from assumed role to create AWS session

	key := s.Data["access_key"].(string)
	secret := s.Data["secret_key"].(string)
	token := s.Data["security_token"].(string)

	creds := credentials.NewStaticCredentials(key, secret, token)

	// creates and returns a pointer to a new AWS session
	sess := session.Must(session.NewSession(&amzaws.Config{
		Credentials: creds,
		MaxRetries:  amzaws.Int(3),
		Region:      amzaws.String("us-east-1"),
	}))

	return sess
}

// S3 api factory function New() returns a pointer
func getS3Service(sess *session.Session) *s3.S3 {
	endpoint := os.Getenv("AWS_S3_ENDPOINT") // this should be an environment variable (look into use of github.com/joho/godotenv)
	return s3.New(sess, &amzaws.Config{
		Endpoint: amzaws.String(endpoint),
	})
}

func getSQSService(sess *session.Session) *sqs.SQS {
	endpoint := os.Getenv("AWS_SQS_ENDPOINT") // this should be an environment variable
	return sqs.New(sess, &amzaws.Config{
		Endpoint: amzaws.String(endpoint),
	})
}

func getSQSQueue(client *sqs.SQS, name string) *aws.SqsQueue {
	q, err := aws.NewSQSQueue(client, name)
	if err != nil {
		fmt.Println("Fatal: " + err.Error())
		os.Exit(1)
	}
	return q
}

// go redis sdk factory function NewClient() returns a pointer
func getRedisClient() *redis.Client {
	redisAddr := chConfig.RedisAddress //"localhost:6379"
	redisPwd := chConfig.RedisPassword //"" // no password set on dev redis-server

	cli := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPwd,
		DB:       0,
	})

	return cli
}

func getResourceRepository(client *redis.Client) *core.RedisResourceRepository {
	return core.NewRedisResourceRepository(client)
}

func getDockerClient() *docker.Client {
	cli, err := docker.NewEnvClient()
	if err != nil {
		fmt.Println(fmt.Errorf("could not connect to Docker: %s", err))
		os.Exit(1)
	}
	return cli
}

/* Signal handling */

func doneOnSignal(doneCh chan<- bool, sigCh <-chan os.Signal) {
	sig := <-sigCh
	// level.Info(logger).Log("msg", "Caught signal, terminating gracefully.", "signal", sig)
	fmt.Println(fmt.Errorf("caught signal %s , terminating gracefully", sig))
	doneCh <- true
}

func CheckFatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %s\n", err.Error())
		os.Exit(1)
	}
}
