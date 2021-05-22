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
	fmt.Println("predownload hook")
	return nil
}


func (m *MOV) PreExecute(ctx context.Context, task *core.Task) error {
	fmt.Println("preexecute hook")
	return nil
}

func (m *MOV) PreUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("preupload hook")
	return nil
}

func (m *MOV) PostUpload(ctx context.Context, task *core.Task) error {
	fmt.Println("postupload hook")
	return nil
}