// Package legacyjson benchmarks the unstructured decode kubemeta replaces,
// against whichever encoding/json implementation the toolchain was built with.
// It lives outside kubemeta because that package imports encoding/json/v2,
// which does not exist when the jsonv2 experiment is off — so only a package
// free of that import can be built both ways.
//
// The jsonv2 experiment is on by default from Go 1.27, which makes
// encoding/json's Unmarshal v1 semantics over the v2 engine. That engine alone
// is worth ~30%, so kubemeta's in-package BenchmarkUnstructuredJson is the
// honest baseline for the partial decode. This one shows the pre-v2 floor:
//
//	B=./pkg/utils/kube/kubemeta/internal/legacyjson/
//	GOEXPERIMENT=nojsonv2 go test -run='^$' -bench=. -benchmem -count=6 $B | sed /^pkg:/d > old.txt
//	go test -run='^$' -bench=BenchmarkUnstructuredJson -benchmem -count=6 ./pkg/utils/kube/kubemeta/ | sed /^pkg:/d > new.txt
//	benchstat old.txt new.txt
//
// Dropping the pkg: line is what lets benchstat pair rows across the two
// packages; it treats pkg as a config field and tables them separately.
package legacyjson

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubemeta/internal/benchfixtures"
)

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
