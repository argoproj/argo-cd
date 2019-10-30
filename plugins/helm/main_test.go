package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover(t *testing.T) {
	assert.ElementsMatch(t, []string{"minio", "redis", "wordpress"}, runDiscover("../../util/helm/testdata"))
}
