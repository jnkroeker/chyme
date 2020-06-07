package ingest

import (
	"fmt"
	"chyme/pkg/aws"
	"chyme/internal/core"
	"path/filepath"
	"errors"
	"net/url"
	"github.com/aws/aws-sdk-go/service/s3"
)

type IngestService interface {
	Ingest(*url.URL, string, int) (int64, error)
}

/* fields that start with a lowercase letter are package internal
* `ResourceRepository:` must be uppercase to be send on to New()
*/
// the redis.Client is now in the resourceRepository
type Config struct {
	ResourceRepository  core.ResourceRepository
	// redis			    *redis.Client
	ResourceSetKey      string
	S3                  *s3.S3
}

type ingestService struct {
	Config
}

func New(config Config) IngestService {

	svc := &ingestService{
		config,
	}

	return svc
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
	fmt.Println(bucketUrl)
	fmt.Println(filterString)
	fmt.Println(filtered)
	fmt.Println(i.ResourceSetKey)
	if err != nil {
		return 0, err
	}
	res, err := i.ResourceRepository.Add(i.ResourceSetKey, filtered)
	if err != nil {
		return 0, ErrEmpty
	}
	return int64(res), nil
}

func (i *ingestService) ingestBucket(bucketUrl *url.URL, filter FilterFunc, depth int) (int, error) {
	bi, err := i.ResourceRepository.BulkInsert(i.ResourceSetKey) 

	bucket := aws.NewS3Bucket(i.S3, bucketUrl.Host)
	
	err = bucket.ListObjects(&aws.ListObjectsOptions{
		Path: bucketUrl.Path,
		Depth: depth,
	}, func(obj *s3.Object) error {
		newResource, err := filter(&url.URL{
			Scheme: bucketUrl.Scheme, 
			Host: bucketUrl.Host,
			Path: *obj.Key,
		})
		fmt.Println(newResource)
		if newResource == "" {
			return nil
		}
		if err != nil {
			return err
		}
		return bi.Insert(newResource)
	})

	if err != nil {
		return 05, err
	}

	bi.Close()

	bucketObjectCount, err := i.ResourceRepository.Count(i.ResourceSetKey)
	if err != nil {
		return 0, err
	}
	return bucketObjectCount, nil
}

var ErrEmpty = errors.New("Got to SAdd but it errored out ")