package io

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// TempPaths allows generating and memorizing random paths for a given URL.
type TempPaths struct {
	paths map[string]string
	lock  sync.Mutex
}

func NewTempPaths() *TempPaths {
	return &TempPaths{
		paths: make(map[string]string),
	}
}

// GetPath generates a path for the given URL or returns previously generated one.
func (p *TempPaths) GetPath(url string) (string, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if val, ok := p.paths[url]; ok {
		return val, nil
	}
	uniqueId, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	repoPath := filepath.Join(os.TempDir(), uniqueId.String())
	p.paths[url] = repoPath
	return repoPath, nil
}
