package ingest

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/s3"
	"kroekerlabs.dev/chyme/services/internal/core"
	"kroekerlabs.dev/chyme/services/pkg/aws"
)

// the redis.Client is now in the resourceRepository
type IngestService struct {
	ResourceRepository core.ResourceRepository
	ResourceSetKey     string
	// redis			    *redis.Client
	S3 *s3.S3
}

func New(repository core.ResourceRepository, key string, s3 *s3.S3) IngestService {

	svc := IngestService{
		repository,
		key,
		s3,
	}

	return svc
}

func (i IngestService) Ingest(resource *core.Resource, filterString string, recursionDepth int) (int, error) {
	//send 'ext/pdf' to use NewExtFilter or 'identity/...' for other method
	filter, err := NewFilter(filterString)
	if err != nil {
		return 0, fmt.Errorf("invalid filter %s: %s", filterString, err.Error())
	}

	if recursionDepth > 0 {
		_, object := filepath.Split(resource.Url.Path)

		if object == "" {
			//if there is no file, we want to index an entire prefix
			return i.ingestPrefix(resource, filter, recursionDepth)
		} else {
			return 0, fmt.Errorf("recursion depth specified but key %s is not a prefix.\n"+
				"\tIf you want to ingest a prefix recursively, append a '/' to the key", resource.String())
		}
	}

	filtered := filter(resource)

	if err != nil {
		return 0, err
	}
	res, err := i.ResourceRepository.Add(i.ResourceSetKey, filtered)
	if err != nil {
		return 0, ErrEmpty
	}
	return res, nil
}

func (i IngestService) ingestPrefix(resource *core.Resource, filter FilterFunc, depth int) (int, error) {
	fmt.Println("here in ingestPrefix")
	fmt.Println(resource.Url.Scheme)
	fmt.Println(resource.Url.Host)
	fmt.Println(resource.Url.Path)

	bucket := aws.NewS3Bucket(i.S3, resource.Url.Host) // strings.Trim(bucketUrl.Path, "/")

	bi, err := i.ResourceRepository.BulkInsert(i.ResourceSetKey)
	if err != nil {
		return 0, err
	}

	err = bucket.ListObjects(&aws.ListObjectsOptions{
		RootPrefix: resource.Url.Path,
		Depth:      depth,
	}, func(obj *s3.Object) error {
		newResource := filter(&core.Resource{
			Url: &url.URL{
				Scheme: resource.Url.Scheme,
				Host:   resource.Url.Host,
				Path:   *obj.Key,
			},
		})
		if newResource == nil {
			fmt.Println("no new resource")
			return nil
		}
		if err != nil {
			fmt.Println(fmt.Errorf("error: %s", err))
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
