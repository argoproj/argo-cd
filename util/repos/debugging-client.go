package repos

import (
	log "github.com/sirupsen/logrus"
)

type debug struct {
	delegate Client
}

func (d debug) WorkDir() string {
	return d.delegate.WorkDir()
}

func (d debug) Test() error {
	log.Debugf("testing")
	err := d.delegate.Test()
	log.WithFields(log.Fields{"err": err}).Debug("tested")
	return err
}

func (d debug) Checkout(path, revision string) (string, error) {
	log.WithFields(log.Fields{"path": path, "revision": revision}).Debug("checking out")
	resolvedRevision, err := d.delegate.Checkout(path, revision)
	log.WithFields(log.Fields{"resolvedRevision": resolvedRevision, "err": err}).Debug("resolved revision")
	return resolvedRevision, err
}

func (d debug) ResolveRevision(path, revision string) (string, error) {
	log.WithFields(log.Fields{"path": path, "revision": revision}).Debug("resolving revision")
	resolvedRevision, err := d.delegate.ResolveRevision(path, revision)
	log.WithFields(log.Fields{"resolvedRevision": resolvedRevision, "err": err}).Debug("resolved revision")
	return resolvedRevision, err
}

func (d debug) LsFiles(glob string) ([]string, error) {
	log.WithFields(log.Fields{"glob": glob}).Debug("listing files")
	paths, err := d.delegate.LsFiles(glob)
	log.WithFields(log.Fields{"paths": paths, "err": err}).Debug("listed files")
	return paths, err
}

func newDebug(client Client, err error) (Client, error) {
	return debug{client}, err
}
