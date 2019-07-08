package depot

import (
	"time"
)

type RevisionMetadata struct {
	Author  string
	Date    time.Time
	Tags    []string
	Message string
}

// Client is a generic repo client interface
type Client interface {
	// test to see we can connect to the repo, returning an error if we cannot
	Test() error
	// return a unique key for the repo server to use for locking
	LockKey() string
	// clean-up any working directories, connect to repo
	Init() error
	// fetch data
	Fetch() error
	Checkout(path, revision string) error
	LsRemote(path, revision string) (string, error)
	LsFiles(path string) ([]string, error)
	CommitSHA() (string, error)
	RevisionMetadata(revision string) (*RevisionMetadata, error)
}
