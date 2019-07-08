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
	// return a key suitable for use for locking this object
	LockKey() string
	// clean-up any working directories, connect to repo
	Init() error
	// fetch data
	Fetch() error
	// checkout a specific directory, the revision maybe empty - in that case assume the latest version
	Checkout(path, revision string) error
	// convert an ambiguous revision (e.g. "master" or "HEAD") into a specific revision
	ResolveRevision(path, revision string) (string, error)
	// list files matching the path
	LsFiles(path string) ([]string, error)
	// return the revision for the checked out code
	Revision() (string, error)
	// return the revision meta-data for the checked out code
	RevisionMetadata(revision string) (*RevisionMetadata, error)
}
