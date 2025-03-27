package util

import (
	"testing"

	"github.com/stretchr/testify/assert"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var appSet = `apiVersion: argoproj.io/v1alpha1
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
	var appSets []*argoprojiov1alpha1.ApplicationSet
	err := readAppset([]byte(appSet), &appSets)
	if err != nil {
		t.Logf("Failed reading appset file")
	}
	assert.Len(t, appSets, 1)
}
