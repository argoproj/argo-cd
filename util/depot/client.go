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
	// check we can connect to the repo
	Test() error
	Root() string
	Init() error
	Fetch() error
	Checkout(path, revision string) error
	LsRemote(path, revision string) (string, error)
	LsFiles(path string) ([]string, error)
	CommitSHA() (string, error)
	RevisionMetadata(revision string) (*RevisionMetadata, error)
}
