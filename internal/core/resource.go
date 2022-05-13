package core

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-redis/redis"
)

type Resource struct {
	Url *url.URL `json:"url"`
	// Phony indicates whether or not resource should be downloaded/uploaded during processing
	Phony bool `json:"phony"`
	hash  string
}

func (r *Resource) String() string {
	return r.Url.String()
}

// SHA1 Hash of the Resource URL
func (r *Resource) Hash() string {
	if r.hash == "" {
		h := sha1.New()
		h.Write([]byte(r.String()))
		r.hash = hex.EncodeToString(h.Sum(nil))
	}
	return r.hash
}

// Concrete redis implementation of ResourceRepository.
// NOTE: remember Bill saying it is never safe to make a copy of an address that a pointer is pointing to?
// i.e. it is never safe to go from pointer to value semantics
// is this why the ResourceRepository interface is implemented using pointer semantics?
type RedisResourceRepository struct {
	client *redis.Client
}

func NewRedisResourceRepository(client *redis.Client) *RedisResourceRepository {
	return &RedisResourceRepository{client}
}

// Pops max `count` Resources from the Set at key `setKey`. If the set contains less than `count`, the number of
// Resources returned will equal the size of the set.
func (r *RedisResourceRepository) Pop(setKey string, count int) ([]*Resource, error) {
	fmt.Println("setKey")
	fmt.Println(setKey)
	fmt.Println("count")
	fmt.Println(count)
	elements, err := r.client.SPopN(setKey, int64(count)).Result()
	if err != nil {
		return nil, err
	}

	resources := make([]*Resource, 0)
	for _, el := range elements {
		resourceUrl, err := url.Parse(el)
		if err != nil {
			// TODO: Move bad URL to reject set.
			continue
		}
		resources = append(resources, &Resource{Url: resourceUrl})
	}

	return resources, nil
}

func (r *RedisResourceRepository) Add(setKey string, resources ...*Resource) (int, error) {
	nResources := len(resources)
	urls := make([]interface{}, nResources)
	for i, resource := range resources {
		fmt.Println(resource)
		urls[i] = resource.String()
	}

	count, err := r.client.SAdd(setKey, urls...).Result()
	return int(count), err
}

func (r *RedisResourceRepository) BulkInsert(setKey string) (BulkResourceInserter, error) {
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

func (r *RedisResourceRepository) Count(setKey string) (int, error) {
	count, err := r.client.SCard(setKey).Result()
	return int(count), err
}

type BulkResourceInserter interface {
	// Insert a resource into the repository. The implementation of this function should be thread-safe.
	Insert(resource *Resource) error
	Close() error
}

// Concrete redis implementation of the BulkResourceInserter
type redisBulkInserter struct {
	cmd    *exec.Cmd
	setKey string
	wc     io.WriteCloser
}

func (i *redisBulkInserter) Insert(resource *Resource) error {
	// _, err := io.WriteString(i.wc, Encode([]string{"SADD", i.setKey, resource}))
	_, err := i.wc.Write([]byte(Encode([]string{"SADD", i.setKey, resource.String()})))
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
