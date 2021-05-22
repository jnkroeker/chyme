package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"kroekerlabs.dev/chyme/services/pkg/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

type ResourceLoader interface {
	Scheme() string
	CheckCapacityPosix(resource *Resource, path string, scaleFactor uint64) (bool, error)
	Download(ctx context.Context, resource *Resource, path string) (int64, error)
	Upload(ctx context.Context, resource *Resource, path string, metadata map[string]*string, remove bool) (int64, error)
	Exists(ctx context.Context, resource *Resource) (bool, error)
	Tag(resource *Resource, tags map[string]string) error
}

type LoaderRegistry map[string]ResourceLoader 

type resourceLoader struct {
	registry LoaderRegistry
}

// Creates a new ResourceLoader that delegates execution to the registered loader
func NewResourceLoader(registry LoaderRegistry) ResourceLoader {
	registry["phony"] = &phonyResourceLoader{}
	return &resourceLoader{registry}
}

func (l *resourceLoader) Scheme() string {
	return ""
}

func (l *resourceLoader) Download(ctx context.Context, resource *Resource, path string) (int64, error) {
	loader, err := l.resolve(resource)
	if err != nil {
		return 0, err
	}
	return loader.Download(ctx, resource, path)
}

func (l *resourceLoader) Upload(ctx context.Context, resource *Resource, path string, metadata map[string]*string, remove bool) (int64, error) {
	loader, err := l.resolve(resource)
	if err != nil {
		return 0, err 
	}
	return loader.Upload(ctx, resource, path, metadata, remove)
}

func (l *resourceLoader) CheckCapacityPosix(resource *Resource, path string, scaleFactor uint64) (bool, error) {
	loader, err := l.resolve(resource)
	if err != nil {
		return false, err
	}
	return loader.CheckCapacityPosix(resource, path, scaleFactor)
}

func (l *resourceLoader) Exists(ctx context.Context, resource *Resource) (bool, error) {
	loader, err := l.resolve(resource)
	if err != nil {
		return false, err 
	}
	return loader.Exists(ctx, resource)
}

func (l *resourceLoader) Tag(resource *Resource, tags map[string]string) error {
	loader, err := l.resolve(resource)
	if err != nil {
		return err
	}
	return loader.Tag(resource, tags)
}

func (l *resourceLoader) resolve(resource *Resource) (ResourceLoader, error) {
	if resource.Phony {
		return l.registry["phony"], nil 
	}
	loader, ok := l.registry[resource.Url.Scheme]
	if !ok {
		return nil, fmt.Errorf("no loader for scheme %s", resource.Url.Scheme)
	}
	return loader, nil
}

type phonyResourceLoader struct{}

func (*phonyResourceLoader) Scheme() string {
	return "phony"
}

func (*phonyResourceLoader) CheckCapacityPosix(resource *Resource, path string, scaleFactor uint64) (bool, error) {
	return true, nil
}

func (*phonyResourceLoader) Download(ctx context.Context, resource *Resource, path string) (int64, error) {
	return 0, nil
}

func (*phonyResourceLoader) Upload(ctx context.Context, resource *Resource, path string, metadata map[string]*string, remove bool) (int64, error) {
	return 0, nil
}

func (l *phonyResourceLoader) Exists(ctx context.Context, resource *Resource) (bool, error) {
	return false, nil
}

func (l *phonyResourceLoader) Tag(resource *Resource, tags map[string]string) error {
	return nil
}

type s3ResourceLoader struct {
	svc             *s3.S3
	defaultMetadata map[string]*string
}

func NewS3ResourceLoader(svc *s3.S3, defaultMetadata map[string]*string) ResourceLoader {
	return &s3ResourceLoader{svc, defaultMetadata}
}

func (l *s3ResourceLoader) Scheme() string {
	return "s3"
}

// Returns true if there is enough space to download the object, false otherwise.
func (l *s3ResourceLoader) CheckCapacityPosix(resource *Resource, path string, scaleFactor uint64) (bool, error) {
	bucket := aws.NewS3Bucket(l.svc, resource.Url.Host)
	objectSz, err := bucket.Size(resource.Url.Path)
	if err != nil {
		return false, err
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return false, err
	}

	return uint64(objectSz)*scaleFactor < stat.Bavail*uint64(stat.Bsize), nil
}

func (l *s3ResourceLoader) Download(ctx context.Context, resource *Resource, filePath string) (int64, error) {
	_, object := path.Split(resource.Url.Path)
	isPfx := object == ""
	isDir, err := isDirectory(filePath)
	if err != nil {
		return 0, err
	}

	bucket := aws.NewS3Bucket(l.svc, resource.Url.Host)

	// If the resource is a prefix and the download location is a directory, sync the prefix into the directory.
	if isPfx && isDir {
		// Sync
		return bucket.DownloadPrefix(ctx, resource.Url.Path, filePath, 1)
	}

	// If the resource is an object and the download location is a file, use the existing file.
	if !(isPfx || isDir) {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		return bucket.Download(ctx, resource.Url.Path, f)
	}

	// If the resource is a prefix and the download location is a file, archive the prefix into the file.
	if isPfx && !isDir {
		ext := filepath.Ext(filePath)
		if ext != ".tar" {
			return 0, fmt.Errorf("unsupported archive format %s", ext)
		}
		return 0, errors.New("prefix archival not implemented")
	}

	// If the resource is an object and the download location is a directory, create a new file in the directory using
	// the name of the object.
	if !isPfx && isDir {
		f, err := os.OpenFile(filepath.Join(filePath, object), os.O_WRONLY|os.O_CREATE, 0700)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		return bucket.Download(ctx, resource.Url.Path, f)
	}

	panic("unreachable")
}

const MetadataObjectName = "tw-metadata"

func (l *s3ResourceLoader) Upload(ctx context.Context, resource *Resource, filePath string, metadata map[string]*string, remove bool) (int64, error) {
	_, object := path.Split(resource.Url.Path)
	isPfx := object == ""
	isDir, err := isDirectory(filePath)
	if err != nil {
		return 0, err
	}

	bucket := aws.NewS3Bucket(l.svc, resource.Url.Host)
	// bucket.DefaultMetadata(l.defaultMetadata)

	// If the resource is a prefix and the upload location is a directory, sync the directory into the prefix.
	if isPfx && isDir {
		trimmed := trimTrailingSlash(resource.Url.Path)
		if remove {
			if err := bucket.DeletePrefix(&aws.ListObjectsOptions{RootPrefix: trimmed}); err != nil {
				return 0, err
			}
		}

		sz, err := bucket.UploadDirectory(ctx, filePath, trimmed)
		if err != nil {
			return 0, err
		}

		// Write a zero-byte object with the metadata if metadata exists.
		if len(metadata) > 0 {
			_, err = bucket.Upload(context.Background(), path.Join(trimmed, MetadataObjectName), &bytes.Buffer{}, metadata)
		}

		return sz, err
	}

	// If the resource is an object and the upload location is a file, use the object name.
	if !(isPfx || isDir) {
		if remove {
			if err := bucket.DeleteIfExists(resource.Url.Path); err != nil {
				return 0, err
			}
		}

		f, err := os.Open(filePath)
		if err != nil {
			return 0, err
		}
		defer f.Close()

		return bucket.Upload(ctx, resource.Url.Path, f, metadata)
	}

	// If the resource is a prefix and the upload location is a file, upload a new object with the filename into the
	// prefix.
	if isPfx && !isDir {
		_, filename := filepath.Split(filePath)
		key := path.Join(resource.Url.Path, filename)

		if remove {
			if err := bucket.DeleteIfExists(key); err != nil {
				return 0, err
			}
		}

		f, err := os.Open(filePath)
		if err != nil {
			return 0, err
		}
		defer f.Close()

		return bucket.Upload(ctx, key, f, metadata)
	}

	// If the resource is an object and the upload location is a directory, archive the upload directory into the
	// object.
	if !isPfx && isDir {
		ext := filepath.Ext(object)
		if ext != ".tar" {
			return 0, fmt.Errorf("unsupported archive format %s", ext)
		}

		return 0, errors.New("directory archival not implemented")
	}

	panic("unreachable")
}

func (l *s3ResourceLoader) Exists(_ context.Context, resource *Resource) (bool, error) {
	return aws.NewS3Bucket(l.svc, resource.Url.Host).Exists(resource.Url.Path)
}

func (l *s3ResourceLoader) Tag(resource *Resource, tags map[string]string) error {
	return nil
	// return aws.NewS3Bucket(l.svc, resource.Url.Host).TagObject(resource.Url.Path, tags)
}

func isDirectory(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.Mode().IsDir(), nil
}

func trimTrailingSlash(str string) string {
	last := len(str) - 1
	if rune(str[last]) == '/' {
		str = str[:last]
	}
	return str
}
