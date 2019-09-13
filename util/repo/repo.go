package repo

import (
	"time"
)

type RevisionMetadata struct {
	Author  string
	Date    time.Time
	Tags    []string
	Message string
}

// Repo is a generic repo client interface
type Repo interface {
	// return a key suitable for use for locking this object
	LockKey() string
	// init
	Init() error
	// list apps for an ambiguous revision,
	ListApps(resolvedRevision string) (apps map[string]string, err error)
	// convert an app's ambiguous revision (e.g. "", "master" or "HEAD") into a specific revision (e.g. "231345034boc" or "5.8.0")
	ResolveAppRevision(app, revision string) (resolvedRevision string, err error)
	// checkout an app
	GetApp(app, resolvedRevision string) (path string, err error)
	// return the revision meta-data for the checked out code
	RevisionMetadata(app, resolvedRevision string) (*RevisionMetadata, error)
}
