package main

import (
	"os/exec"
	"strconv"
	"strings"
	"io"
	"fmt"

	"github.com/go-redis/redis"
)

type ResourceRepository interface {
	Add(string, string) (int, error)
	BulkInsert(string) (BulkResourceInserter, error)
	Count(string) (int, error)
}

// Concrete redis implementation of ResourceRepository.
type redisResourceRepository struct {
	client *redis.Client
}

func getResourceRepository(client *redis.Client) ResourceRepository {
	return &redisResourceRepository{client}
}

func (r *redisResourceRepository) Add(setKey string, resource string) (int, error) {
	count, err := r.client.SAdd(setKey, resource).Result()
	return int(count), err
}

func (r *redisResourceRepository) BulkInsert(setKey string) (BulkResourceInserter, error) {
	// uri := "redis://" + r.client.Options().Addr
	// cmd := exec.Command("redis-cli", "-u", uri, "--pipe")
	cmd := exec.Command("redis-cli", "--pipe")

	stdIn, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	//start the command so it is ready to accept inserts
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &redisBulkInserter{cmd, setKey, stdIn}, nil
}

func (r *redisResourceRepository) Count(setKey string) (int, error) {
	count, err := r.client.SCard(setKey).Result()
	return int(count), err
}

type BulkResourceInserter interface {
	// Insert a resource into the repository. The implementation of this function should be thread-safe.
	Insert(resource string) error
	Close() error
} 

// Concrete redis implementation of the BulkResourceInserter
type redisBulkInserter struct {
	cmd    *exec.Cmd
	setKey string
	wc     io.WriteCloser
}

func (i *redisBulkInserter) Insert(resource string) (error) {
	// _, err := io.WriteString(i.wc, Encode([]string{"SADD", i.setKey, resource}))
	_, err := i.wc.Write([]byte(Encode([]string{"SADD", i.setKey, resource})))
	if err != nil {
		fmt.Println("bulk insert failed")
		return err
	}
	return nil
}

func (i *redisBulkInserter) Close() error {
	if err := i.wc.Close(); err != nil {
		return err
	}
	return i.cmd.Wait()
}


// redis utilities
// functionality for working with Redis Serialization Protocol

const separator = "\r\n"

// Encodes a Redis command as Redis protocol.
// format bash command the redis way
func Encode(cmd []string) string {
	var sb strings.Builder

	sb.WriteRune('*')
	sb.WriteString(strconv.Itoa(len(cmd)))
	sb.WriteString(separator)

	for _, arg := range cmd {
		sb.WriteRune('$')
		sb.WriteString(strconv.Itoa(len(arg)))
		sb.WriteString(separator)
		sb.WriteString(arg)
		sb.WriteString(separator)
	}
	return sb.String()
}
