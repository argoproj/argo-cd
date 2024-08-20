package io

import (
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type TempPaths interface {
	Add(key string, value string)
	GetPath(key string) (string, error)
	GetPathIfExists(key string) string
	GetPaths() map[string]string
}

// RandomizedTempPaths allows generating and memoizing random paths, each path being mapped to a specific key.
type RandomizedTempPaths struct {
	root  string
	paths map[string]string
	lock  sync.RWMutex
}

func NewRandomizedTempPaths(root string) *RandomizedTempPaths {
	return &RandomizedTempPaths{
		root:  root,
		paths: map[string]string{},
	}
}

func (p *RandomizedTempPaths) Add(key string, value string) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.paths[key] = value
}

// GetPath generates a path for the given key or returns previously generated one.
func (p *RandomizedTempPaths) GetPath(key string) (string, error) {
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

// GetPathIfExists gets a path for the given key if it exists. Otherwise, returns an empty string.
func (p *RandomizedTempPaths) GetPathIfExists(key string) string {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if val, ok := p.paths[key]; ok {
		return val
	}
	return ""
}

// GetPaths gets a copy of the map of paths.
func (p *RandomizedTempPaths) GetPaths() map[string]string {
	p.lock.RLock()
	defer p.lock.RUnlock()
	paths := map[string]string{}
	for k, v := range p.paths {
		paths[k] = v
	}
	return paths
}
