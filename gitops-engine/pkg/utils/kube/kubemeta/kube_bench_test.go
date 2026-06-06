package kubemeta

import (
	"fmt"
	"strings"
	"testing"
)

// largeObject synthesises a manifest the size real syncs diff: a Deployment
// whose pod template carries many env vars, plus a status blob. Only
// apiVersion/kind/metadata.{name,namespace} are read — everything else is bulk
// the decoder skips. nKB controls how much of it there is.
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
// members in one object, and because json.Marshal sorts map keys, "data" comes
// before the "kind"/"metadata" we want. Duplicate-name rejection would allocate
// proportionally to the member count here even though every one is skipped.
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

var benchInputs = []struct {
	name string
	data []byte
}{
	{"small", []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm","namespace":"default"}}`)},
	{"guestbook", []byte(guestbook)},
	{"large_8kb", largeObject(8)},
	{"large_64kb", largeObject(64)},
	{"wide_configmap_64kb", wideConfigMap(64)},
}

// BenchmarkNewKubeJson measures the extraction path:
//
//	go test -run='^$' -bench=BenchmarkNewKubeJson -benchmem ./pkg/utils/kube/kubemeta/
func BenchmarkNewKubeJson(b *testing.B) {
	for _, in := range benchInputs {
		b.Run(in.name, func(b *testing.B) {
			b.SetBytes(int64(len(in.data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				k, err := NewKubeJson(in.data)
				if err != nil {
					b.Fatal(err)
				}
				if k.GetKind() == "" {
					b.Fatal("empty kind")
				}
			}
		})
	}
}
