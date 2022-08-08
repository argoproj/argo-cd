package io

import log "github.com/sirupsen/logrus"

var (
	NopCloser = NewCloser(func() error {
		return nil
	})
)

type Closer interface {
	Close() error
}

type inlineCloser struct {
	close func() error
}

func (c *inlineCloser) Close() error {
	return c.close()
}

func NewCloser(close func() error) Closer {
	return &inlineCloser{close: close}
}

// Close is a convenience function to close a object that has a Close() method, ignoring any errors
// Used to satisfy errcheck lint
func Close(c Closer) {
	if err := c.Close(); err != nil {
		log.Warnf("failed to close %v: %v", c, err)
	}
}
