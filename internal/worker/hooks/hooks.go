// package hooks provides an interface to hook into worker processing.
package hooks

import (
	"context"

	"kroekerlabs.dev/chyme/services/internal/core"
)

type Interface interface {
	PreDownload(ctx context.Context, task *core.Task) error
	PreExecute(ctx context.Context, task *core.Task) error
	PreUpload(ctx context.Context, task *core.Task) error
	PostUpload(ctx context.Context, task *core.Task) error
}

type Registry map[string]Interface 

type Base struct {}

func (Base) PreDownload(ctx context.Context, task *core.Task) error {
	return nil
}

func (Base) PreExecute(ctx context.Context, task *core.Task) error {
	return nil
}

func (Base) PreUpload(ctx context.Context, task *core.Task) error {
	return nil
}

func (Base) PostUpload(ctx context.Context, task *core.Task) error {
	return nil
}