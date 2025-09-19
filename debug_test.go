package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func main() {
	// Create normalizer with JQPaths
	normalizer, err := normalizers.NewIgnoreNormalizer([]v1alpha1.ResourceIgnoreDifferences{{
		Group:   "apps",
		Kind:    "Deployment",
		JQPaths: []string{`.metadata.annotations // {} | [keys[] | select(startswith("customprefix.")) | ["metadata", "annotations", .]]`},
	}}, make(map[string]v1alpha1.ResourceOverride), normalizers.IgnoreNormalizerOpts{})

	if err != nil {
		log.Fatal(err)
	}

	deployment := test.NewDeployment()
	deployment.SetAnnotations(map[string]string{
		"customprefix.annotation1": "value1",
		"customprefix.annotation2": "value2",
		"other.annotation":         "keep-this",
	})

	fmt.Println("Before normalization:")
	data, _ := json.MarshalIndent(deployment.Object, "", "  ")
	fmt.Println(string(data))

	// Apply normalization
	err = normalizer.Normalize(deployment)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nAfter normalization:")
	data, _ = json.MarshalIndent(deployment.Object, "", "  ")
	fmt.Println(string(data))

	fmt.Println("\nAnnotations:")
	fmt.Printf("%+v\n", deployment.GetAnnotations())
}
