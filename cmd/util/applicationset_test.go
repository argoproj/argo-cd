package util

import (
	"testing"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
)

var appSet string = `apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  generators:
  - list:
      elements:
      - cluster: engineering-dev
        url: https://1.2.3.4
  template:
    metadata:
      name: '{{cluster}}-guestbook'
    spec:
      source:
        repoURL: https://github.com/infra-team/cluster-deployments.git
        targetRevision: HEAD
        path: guestbook/{{cluster}}
      destination:
        server: '{{url}}'
        namespace: guestbook
`

func TestReadAppSet(t *testing.T) {
	appsets := []*argoprojiov1alpha1.ApplicationSet{}
	err := readAppset([]byte(appSet), &appsets)
	if err != nil {
		t.Logf("Failed reading appset file")
	}
	assert.Equal(t, len(appsets), 1)
}
