package worker

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"kroekerlabs.dev/chyme/services/internal/core")

type State struct {
	Stage       ProcessStage
	TaskMessage *core.TaskMessage
	Version     string
}

func (s *State) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(s)
}

func (s *State) Decode(r io.Reader) error {
	return json.NewDecoder(r).Decode(s)
}

type Persister interface {
	Persist(s *State) error
	Load() ([]*State, error)
}

const StateFilename = ".chstate.json"

type fsPersister struct {
	workDir string
}

func NewFSPersister(workDir string) Persister {
	return &fsPersister{workDir}
}

func (p *fsPersister) Persist(state *State) error {
	f, err := os.OpenFile(filepath.Join(state.TaskMessage.Task.Workspace.InternalDir, StateFilename), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return state.Encode(f)
}

func (p *fsPersister) Load() (states []*State, err error) {
	states = make([]*State, 0)
	err = filepath.Walk(p.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != StateFilename {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		state := &State{}
		if err := state.Decode(f); err != nil {
			return err
		}
		states = append(states, state)

		return nil
	})
	return
}