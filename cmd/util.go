package cmd

import (
	"vault_aws/internal/core"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hashicorp/vault/api"
	"github.com/go-redis/redis"
)

const (
	vaultAddr = "http://localhost:8200"
	vaultStaticToken = "s.FMtWzRspvkYIvNerpUVBwxg7" // this value will change each time a new vault -dev server is created
	vaultStsSecret = "aws/sts/assume_role_s3_sqs"
)

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

func getResourceRepository(client *redis.Client) core.ResourceRepository {
	return core.NewRedisResourceRepository(client)
}