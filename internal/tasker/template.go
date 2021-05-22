package tasker

import (
	"kroekerlabs.dev/chyme/services/internal/core"
)

type Template struct {
	Name string
	Create func(resource *core.Resource) *core.Task 
}

type Templater interface {
	Create(resource *core.Resource) []*core.Task 
	Reload() error 
}

type inMemTemplater struct {
	templates []*Template 
	version   string
}

func NewInMemTemplater(templates []*Template, version string) Templater {
	return &inMemTemplater{templates, version}
}

func (t *inMemTemplater) Create(resource *core.Resource) []*core.Task {
	tasks := make([]*core.Task, 0)
	for _, template := range t.templates {
		if task := template.Create(resource); task != nil {
			task.Version = t.version
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (t *inMemTemplater) Reload() error {
	return nil
}