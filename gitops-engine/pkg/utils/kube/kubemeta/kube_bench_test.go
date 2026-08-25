package kubemeta

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubemeta/internal/benchfixtures"
)

// BenchmarkNewKubeJson measures the extraction path:
//
//	go test -run='^$' -bench=BenchmarkNewKubeJson -benchmem ./pkg/utils/kube/kubemeta/
func BenchmarkNewKubeJson(b *testing.B) {
	for _, in := range benchfixtures.Inputs {
		b.Run(in.Name, func(b *testing.B) {
			b.SetBytes(int64(len(in.Data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				k, err := NewKubeJson(in.Data)
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

// BenchmarkUnstructuredJson is the baseline NewKubeJson replaces: decoding the
// whole document into an unstructured.Unstructured to read the same four
// fields. Under Go's jsonv2 experiment (on by default since 1.27) this is the
// v1 API over the v2 engine; internal/legacyjson runs the same body against the
// pre-v2 engine.
func BenchmarkUnstructuredJson(b *testing.B) {
	for _, in := range benchfixtures.Inputs {
		b.Run(in.Name, func(b *testing.B) {
			b.SetBytes(int64(len(in.Data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				u := &unstructured.Unstructured{}
				if err := json.Unmarshal(in.Data, u); err != nil {
					b.Fatal(err)
				}
				if u.GetKind() == "" {
					b.Fatal("empty kind")
				}
				_ = kube.NewResourceKey(u.GroupVersionKind().Group, u.GetKind(), u.GetNamespace(), u.GetName())
			}
		})
	}
}
