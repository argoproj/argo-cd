package io

import (
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// TempPaths allows generating and memoizing random paths, each path being mapped to a specific key.
type TempPaths struct {
	root  string
	paths map[string]string
	lock  sync.Mutex
}

func NewTempPaths(root string) *TempPaths {
	return &TempPaths{
		root:  root,
		paths: map[string]string{},
	}
}

func (p *TempPaths) Add(key string, value string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.paths[key] = value
}

// GetPath generates a path for the given key or returns previously generated one.
func (p *TempPaths) GetPath(key string) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if val, ok := p.paths[key]; ok {
		return val, nil
	}
	uniqueId, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	repoPath := filepath.Join(p.root, uniqueId.String())
	p.paths[key] = repoPath
	return repoPath, nil
}
