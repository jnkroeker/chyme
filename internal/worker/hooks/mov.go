package hooks

import (
	"context"
	"fmt"

	"kroekerlabs.dev/chyme/services/internal/core"
)

type MOV struct {
	Base
	TaskLoader     core.TaskLoader
	ResourceLoader core.ResourceLoader
}

func (m *MOV) PreDownload(ctx context.Context, task *core.Task) error {
	fmt.Println("MOV predownload hook")
	return nil
}

func (m *MOV) PreExecute(ctx context.Context, task *core.Task) error {
	fmt.Println("MOV preexecute hook")
	return nil
}

func (m *MOV) PreUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("MOV preupload hook")
	return nil
}

func (m *MOV) PostUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("MOV postupload hook")
	return nil
}
