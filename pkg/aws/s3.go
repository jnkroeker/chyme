package aws

import (
	"context"
	"errors"
	"path/filepath"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"

	"kroekerlabs.dev/chyme/services/pkg/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"golang.org/x/sync/errgroup"
)

const maxDeleteObjectsKeys = 1000

/** s3 bucket specifics **/

type Bucket interface {
	ListObjects(options *ListObjectsOptions, visit func(object *s3.Object) error) error
	Download(ctx context.Context, key string, w io.WriterAt) (int64, error)
	DownloadPrefix(ctx context.Context, key string, dir string, depth int) (int64, error)
	Upload(ctx context.Context, key string, r io.Reader, metadata map[string]*string) (int64, error)
	UploadDirectory(ctx context.Context, dir string, basePrefix string) (int64, error)
	Size(key string) (int64, error)
	Exists(key string) (bool, error)
	Delete(key string) error
	DeleteIfExists(key string) error
	DeletePrefix(options *ListObjectsOptions) error
}

type s3Bucket struct {
	name            string
	svc             *s3.S3
	downloader      *s3manager.Downloader 
	uploader        *s3manager.Uploader
	defaultMetadata map[string]*string
}

func NewS3Bucket(svc *s3.S3, name string) Bucket {
	downloader := s3manager.NewDownloaderWithClient(svc)
	uploader   := s3manager.NewUploaderWithClient(svc)

	return &s3Bucket{name, svc, downloader, uploader, nil}
}

/** list objects of s3 bucket specifics **/

type ListObjectsOptions struct {
	RootPrefix  string
	Depth int
}

func (b *s3Bucket) ListObjects(options *ListObjectsOptions, visit func(object *s3.Object) error) error {
	depth := options.Depth
	fmt.Println("here in ListObjects")
	fmt.Println(options.RootPrefix)
	fmt.Println(depth)
	fmt.Println(b.name)

	//user specified recusion depth is the maximum depth; we start from 1 when calling lister.list
	lister := lister{b.svc, visit, depth}
	return lister.list(&s3.ListObjectsV2Input{
		Bucket:    aws.String(b.name),
		// Prefix as below is exactly the same as above Bucket param
		// figure out how to parse the bit after the bucket in the URL
		// better yet, how to specify the URL's scheme
		// Prefix:    aws.String(strings.Trim(options.RootPrefix, "/")),
		Delimiter: aws.String("/"),
	}, 1)
}

func (b *s3Bucket) Download(ctx context.Context, key string, w io.WriterAt) (int64, error) {
	cw := &util.CountingWriterAt{WriterAt: w}
	_, err := b.downloader.DownloadWithContext(ctx, cw, &s3.GetObjectInput{
		Bucket: aws.String(b.name),
		Key:    aws.String(key),
	})
	return int64(cw.BytesWritten), err
}

func (b *s3Bucket) Upload(ctx context.Context, key string, r io.Reader, metadata map[string]*string) (int64, error) {
	cr := &util.CountingReader{Reader: r}
	// mergeMetadata(metadata, b.defaultMetadata)
	_, err := b.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Key: aws.String(key),
		Bucket: aws.String(b.name),
		Body: cr,
		Metadata: metadata,
	})
	return int64(cr.BytesRead), err
}

// Uploads a directory by appending the paths of its subtree (relative to the base directory)
func (b *s3Bucket) UploadDirectory(ctx context.Context, dir string, basePrefix string) (int64, error) {
	iter, readers, err := b.directoryToUploadIterator(dir, basePrefix)
	if err != nil {
		return 0, err 
	}
	if err := b.uploader.UploadWithIterator(ctx, iter); err != nil {
		return 0, err 
	}
	return readers.Sum(), nil
}

// Walks a directory and creates an UploadIterator with its contents.
func (b *s3Bucket) directoryToUploadIterator(dir string, basePrefix string) (*s3manager.UploadObjectsIterator, util.CountingReaders, error) {
	objects := make([]s3manager.BatchUploadObject, 0)
	readers := util.CountingReaders(make([]*util.CountingReader, 0))

	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		reader := &util.CountingReader{Reader: f}
		readers = append(readers, reader)

		objects = append(objects, s3manager.BatchUploadObject{
			Object: &s3manager.UploadInput{
				Key:      aws.String(PathToKey(dir, basePrefix, path)),
				Bucket:   aws.String(b.name),
				Body:     reader,
				Metadata: b.defaultMetadata,
			},
			After: func() error {
				return f.Close()
			},
		})

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return &s3manager.UploadObjectsIterator{Objects: objects}, readers, nil
}

// Takes the path relative to the base directory and converts it to a key, then appends that key to basePrefix.
func PathToKey(baseDir string, basePrefix string, path string) string {
	rel, _ := filepath.Rel(baseDir, path)
	return strings.Join([]string{basePrefix, rel}, "/")
}

func (b *s3Bucket) DownloadPrefix(ctx context.Context, key string, dir string, depth int) (int64, error) {
	if depth != 1 {
		return 0, errors.New("download recursion depth > 1 not implemented")
	}

	objects := make([]*s3.Object, 0)
	mtx := sync.Mutex{}
	err := b.ListObjects(&ListObjectsOptions{RootPrefix: key, Depth: depth}, func(object *s3.Object) error {
		mtx.Lock()
		objects = append(objects, object)
		mtx.Unlock()
		return nil 
	})
	if err != nil {
		return 0, err 
	}

	bdObjects := make([]s3manager.BatchDownloadObject, len(objects))
	writers := util.CountingWriterAts(make([]*util.CountingWriterAt, len(objects)))

	for i, obj := range objects {
		f, err := os.OpenFile(filepath.Join(dir, path.Base(*obj.Key)), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return 0, err
		}

		writers[i] = &util.CountingWriterAt{WriterAt: f}
		bdObjects[i] = s3manager.BatchDownloadObject{
			Object: &s3.GetObjectInput{
				Bucket: aws.String(b.name),
				Key:    obj.Key,
			},
			Writer: writers[i],
			After: func() error {
				return f.Close()
			},
		}
	}

	err = b.downloader.DownloadWithIterator(ctx, &s3manager.DownloadObjectsIterator{Objects: bdObjects})
	return writers.Sum(), err
}

func (b *s3Bucket) Exists(key string) (bool, error) {
	_, err := b.svc.HeadObject(&s3.HeadObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(b.name),
	})
	if err != nil {
		if amzerr, ok := err.(awserr.Error); ok && (amzerr.Code() == s3.ErrCodeNoSuchKey || amzerr.Code() == "NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (b *s3Bucket) Delete(key string) error {
	_, err := b.svc.DeleteObject(&s3.DeleteObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(b.name),
	})
	return err
}

func (b *s3Bucket) DeleteIfExists(key string) error {
	exists, err := b.Exists(key)
	if err != nil {
		return err
	}
	if exists {
		return b.Delete(key)
	}
	return nil
}

func (b *s3Bucket) DeletePrefix(options *ListObjectsOptions) error {
	keys := make([]*string, 0)
	mtx := sync.Mutex{}

	if err := b.ListObjects(options, func(object *s3.Object) error {
		mtx.Lock()
		keys = append(keys, object.Key)
		mtx.Unlock()
		return nil
	}); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	objects := make([]*s3.ObjectIdentifier, 0)
	for idx, key := range keys {
		objects = append(objects, &s3.ObjectIdentifier{Key: key})
		if idx%maxDeleteObjectsKeys == maxDeleteObjectsKeys-1 || idx == len(keys)-1 {
			req, _ := b.makeDeleteObjectsRequest(objects)
			if err := req.Send(); err != nil {
				return err
			}
			objects = make([]*s3.ObjectIdentifier, 0)
		}
	}

	return nil
}

func (b *s3Bucket) makeDeleteObjectsRequest(objects []*s3.ObjectIdentifier) (*request.Request, *s3.DeleteObjectsOutput) {
	return b.svc.DeleteObjectsRequest(&s3.DeleteObjectsInput{
		Bucket: aws.String(b.name),
		Delete: &s3.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
}

func (b *s3Bucket) Size(key string) (int64, error) {
	if isPrefix(key) {
		return b.sizePrefix(key)
	} else {
		return b.sizeObject(key)
	}
}

func (b *s3Bucket) sizeObject(key string) (int64, error) {
	head, err := b.svc.HeadObject(&s3.HeadObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(b.name),
	})
	if err != nil {
		return 0, err
	}
	return *head.ContentLength, nil
}

func (b *s3Bucket) sizePrefix(key string) (int64, error) {
	var acc int64

	err := b.ListObjects(&ListObjectsOptions{RootPrefix: key}, func(object *s3.Object) error {
		atomic.AddInt64(&acc, *object.Size)
		return nil
	})

	return acc, err
}

type lister struct {
	svc *s3.S3
	visit func(*s3.Object) error
	maxDepth int
}

func (l *lister) list(input *s3.ListObjectsV2Input, depth int) error {
	//stop listing objects to process when we have reached the max user specified recursion depth
	if depth > l.maxDepth {
		return nil
	}
	objects, prefixes, err := list(l.svc, input)
	if err != nil {
		return err
	}
	var g errgroup.Group
	for _, object := range objects {
		obj := object
		g.Go(func() error { return l.visit(obj) })
	}
	for _, pfx := range prefixes {
		newInput := *input
		newInput.Prefix = pfx
		// tail recursion golang style with errgroup package
		g.Go(func() error { return l.list(&newInput, depth+1) })
	}
	return g.Wait()
}

func list(svc *s3.S3, input *s3.ListObjectsV2Input) ([]*s3.Object, []*string, error) {
	objects := make([]*s3.Object, 0)
	prefixes := make([]*string, 0)

	res, err := svc.ListObjectsV2(input)
	fmt.Println("list objects response")
	fmt.Println(res.Contents)
	if err != nil {
		return nil, nil, fmt.Errorf("error listing prefix %s: %s", *input.Prefix, err.Error())
	}
	objects = append(objects, res.Contents...)
	for _, commonPrefix := range res.CommonPrefixes {
		prefixes = append(prefixes, commonPrefix.Prefix)
	}
	input.ContinuationToken = res.NextContinuationToken

	return objects, prefixes, nil
}

func isPrefix(key string) bool {
	_, object := path.Split(key)
	return object == ""
}