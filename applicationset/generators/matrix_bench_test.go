package generators

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// benchClusterSecrets returns count cluster secrets shaped like a registered cluster, with a bearer
// token and CA data so the cost of copying them out of the cache is realistic.
func benchClusterSecrets(namespace string, count int) []client.Object {
	config := fmt.Sprintf(`{"bearerToken":%q,"tlsClientConfig":{"insecure":false,"caData":%q}}`,
		strings.Repeat("t", 800), strings.Repeat("c", 1400))

	clusters := make([]client.Object, count)
	for i := range clusters {
		name := fmt.Sprintf("cluster-%03d", i)
		clusters[i] = &corev1.Secret{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"environment":                    "production",
			},
			Data: map[string][]byte{
				"name":   []byte(name),
				"server": []byte(fmt.Sprintf("https://%s.example.com", name)),
				"config": []byte(config),
			},
			Type: corev1.SecretType("Opaque"),
		}
	}
	return clusters
}

func benchListElements(count int) []apiextensionsv1.JSON {
	elements := make([]apiextensionsv1.JSON, count)
	for i := range elements {
		raw, err := json.Marshal(map[string]string{"path": fmt.Sprintf("apps/app-%03d", i)})
		if err != nil {
			panic(err)
		}
		elements[i] = apiextensionsv1.JSON{Raw: raw}
	}
	return elements
}

// BenchmarkMatrixGenerate_ListByClusters measures a matrix whose second generator does not reference
// the first, the shape that lists every cluster secret once per outer parameter set.
func BenchmarkMatrixGenerate_ListByClusters(b *testing.B) {
	const namespace = "namespace"

	fakeClient := fake.NewClientBuilder().WithObjects(benchClusterSecrets(namespace, 500)...).Build()
	matrixGenerator := NewMatrixGenerator(map[string]Generator{
		"List":     &ListGenerator{},
		"Clusters": NewClusterGenerator(fakeClient, namespace),
	})

	appSetGenerator := &v1alpha1.ApplicationSetGenerator{
		Matrix: &v1alpha1.MatrixGenerator{
			Generators: []v1alpha1.ApplicationSetNestedGenerator{
				{List: &v1alpha1.ListGenerator{Elements: benchListElements(200)}},
				{Clusters: &v1alpha1.ClusterGenerator{}},
			},
		},
	}
	appSet := &v1alpha1.ApplicationSet{
		Name:      "set",
		Namespace: namespace,
		Spec:      v1alpha1.ApplicationSetSpec{GoTemplate: true},
	}

	b.ReportAllocs()
	for b.Loop() {
		params, err := matrixGenerator.GenerateParams(appSetGenerator, appSet, fakeClient)
		if err != nil {
			b.Fatal(err)
		}
		if len(params) != 200*501 {
			b.Fatalf("unexpected parameter count %d", len(params))
		}
	}
}

// BenchmarkMatrixGenerate_ListByClustersPerRow measures a matrix whose second generator selects on
// the first, so it is generated once per outer parameter set and none of the work is reusable.
func BenchmarkMatrixGenerate_ListByClustersPerRow(b *testing.B) {
	const namespace = "namespace"

	fakeClient := fake.NewClientBuilder().WithObjects(benchClusterSecrets(namespace, 500)...).Build()
	matrixGenerator := NewMatrixGenerator(map[string]Generator{
		"List":     &ListGenerator{},
		"Clusters": NewClusterGenerator(fakeClient, namespace),
	})

	elements := make([]apiextensionsv1.JSON, 50)
	for i := range elements {
		elements[i] = apiextensionsv1.JSON{Raw: []byte(`{"env":"production"}`)}
	}

	appSetGenerator := &v1alpha1.ApplicationSetGenerator{
		Matrix: &v1alpha1.MatrixGenerator{
			Generators: []v1alpha1.ApplicationSetNestedGenerator{
				{List: &v1alpha1.ListGenerator{Elements: elements}},
				{Clusters: &v1alpha1.ClusterGenerator{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{"environment": "{{.env}}"}},
				}},
			},
		},
	}
	appSet := &v1alpha1.ApplicationSet{
		Name:      "set",
		Namespace: namespace,
		Spec:      v1alpha1.ApplicationSetSpec{GoTemplate: true},
	}

	b.ReportAllocs()
	for b.Loop() {
		params, err := matrixGenerator.GenerateParams(appSetGenerator, appSet, fakeClient)
		if err != nil {
			b.Fatal(err)
		}
		if len(params) != 50*500 {
			b.Fatalf("unexpected parameter count %d", len(params))
		}
	}
}
