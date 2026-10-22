// Package benchfixtures holds the manifests the kubemeta benchmarks decode.
// It is a separate package, and imports no encoding/json/v2, so the legacyjson
// benchmark can build with GOEXPERIMENT=nojsonv2 off the same inputs.
package benchfixtures

import (
	"fmt"
	"strings"
)

// Guestbook keeps nested objects/arrays/escapes so the skip path is exercised.
const Guestbook = `{
  "apiVersion": "argoproj.io/v1alpha1",
  "kind": "Application",
  "metadata": {
    "name": "guestbook",
    "namespace": "argocd",
    "finalizers": ["resources-finalizer.argocd.argoproj.io"],
    "labels": {"name": "guestbook"}
  },
  "spec": {
    "project": "default",
    "source": {
      "repoURL": "https://github.com/argoproj/argocd-example-apps.git",
      "path": "guestbook",
      "helm": {
        "values": "ingress:\n  enabled: true\n  tls:\n    - secretName: mydomain-tls\n",
        "parameters": [{"name": "a\\.b/c", "value": "x", "forceString": true}]
      }
    },
    "destination": {"server": "https://kubernetes.default.svc", "namespace": "guestbook"}
  }
}`

// largeObject synthesises a manifest the size real syncs diff: a Deployment
// whose pod template carries many env vars. Everything outside the four fields
// read is bulk the decoder skips; nKB controls how much of it there is.
func largeObject(nKB int) []byte {
	var env strings.Builder
	for i := 0; env.Len() < nKB*1024; i++ {
		fmt.Fprintf(&env, `{"name":"VAR_%d","value":"some-reasonably-long-environment-value-%d"},`, i, i)
	}
	return []byte(fmt.Sprintf(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {"name": "my-app", "namespace": "production", "labels": {"app": "my-app"}},
  "spec": {"replicas": 3, "template": {"spec": {"containers": [{"name": "app", "env": [%s {"name":"LAST","value":"x"}]}]}}},
  "status": {"observedGeneration": 7, "replicas": 3, "updatedReplicas": 3, "readyReplicas": 3, "availableReplicas": 3}
}`, env.String()))
}

// wideConfigMap is the adversarial shape for a skipping decoder: thousands of
// members in one object, and json.Marshal sorts keys, so "data" precedes the
// "kind"/"metadata" we want. Duplicate-name rejection would allocate per member
// here even though every one is skipped.
func wideConfigMap(nKB int) []byte {
	var data strings.Builder
	for i := 0; data.Len() < nKB*1024; i++ {
		if i > 0 {
			data.WriteString(",")
		}
		fmt.Fprintf(&data, `"key-%d":"value-%d-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`, i, i)
	}
	return []byte(fmt.Sprintf(`{"apiVersion":"v1","data":{%s},"kind":"ConfigMap","metadata":{"name":"cm","namespace":"default"}}`, data.String()))
}

// Input is one benchmark manifest.
type Input struct {
	Name string
	Data []byte
}

// Inputs is the shared corpus, smallest first.
var Inputs = []Input{
	{"small", []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","namespace":"default"}}`)},
	{"guestbook", []byte(Guestbook)},
	{"large_8kb", largeObject(8)},
	{"large_64kb", largeObject(64)},
	{"wide_configmap_64kb", wideConfigMap(64)},
}
