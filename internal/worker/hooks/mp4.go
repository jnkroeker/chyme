package hooks

import (
	"context"
	"fmt"

	"kroekerlabs.dev/chyme/services/internal/core"
)

type MP4 struct {
	Base
	TaskLoader     core.TaskLoader
	ResourceLoader core.ResourceLoader
}

func (m *MP4) PreDownload(ctx context.Context, task *core.Task) error {
	fmt.Println("MP4 predownload hook")
	return nil
}

func (m *MP4) PreExecute(ctx context.Context, task *core.Task) error {
	fmt.Println("MP4 preexecute hook")
	return nil
}

func (m *MP4) PreUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("MP4 preupload hook")
	return nil
}

func (m *MP4) PostUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("MP4 postupload hook")
	return nil
}
