package db

import (
	"context"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	fuzzTestNamespace = "default"
)

func FuzzCreateRepoCertificate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		f := fuzz.NewConsumer(data)
		repocertlist := &v1alpha1.RepositoryCertificateList{}
		err := f.GenerateStruct(repocertlist)
		if err != nil {
			return
		}
		upsert, err := f.GetBool()
		if err != nil {
			return
		}
		clientset := getCertClientset()
		db := NewDB(fuzzTestNamespace, settings.NewSettingsManager(context.Background(), clientset, fuzzTestNamespace), clientset)
		_, _ = db.CreateRepoCertificate(context.Background(), repocertlist, upsert)
	})
}
