package main

import (
	"fmt"
	
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"golang.org/x/sync/errgroup"
)

/** s3 bucket specifics **/

type Bucket interface {
	ListObjects(options *ListObjectsOptions, visit func(object *s3.Object) error) error
}

type s3Bucket struct {
	name   string
	svc    *s3.S3
	downloader *s3manager.Downloader 
	uploader   *s3manager.Uploader
}

func NewS3Bucket(svc *s3.S3, name string) Bucket {
	downloader := s3manager.NewDownloaderWithClient(svc)
	uploader   := s3manager.NewUploaderWithClient(svc)

	return &s3Bucket{name, svc, downloader, uploader}
}

/** list objects of s3 bucket specifics **/

type ListObjectsOptions struct {
	Path  string
	Depth int
}

func (b *s3Bucket) ListObjects(options *ListObjectsOptions, visit func(object *s3.Object) error) error {
	depth := options.Depth

	//user specified recusion depth is the maximum depth; we start from 1 when calling lister.list
	lister := lister{b.svc, visit, depth}
	return lister.list(&s3.ListObjectsV2Input{
		Bucket:    aws.String(b.name),
		Prefix:    aws.String(options.Path),
		Delimiter: aws.String("/"),
	}, 1)
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
	if err != nil {
		return nil, nil, fmt.Errorf("error listing prefix %s: %s", *input.Prefix, err.Error())
	}
	objects = append(objects, res.Contents...)
	for _, commonPrefix := range res.CommonPrefixes {
		prefixes = append(prefixes, commonPrefix.Prefix)
	}
	// input.ContinuationToken = res.NextContinuationToken

	return objects, prefixes, nil
}
