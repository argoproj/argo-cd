package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// CreateOrUpdate overrides "sigs.k8s.io/controller-runtime" function
// in sigs.k8s.io/controller-runtime/pkg/controller/controllerutil/controllerutil.go
// to add equality for argov1alpha1.ApplicationDestination
// argov1alpha1.ApplicationDestination has a private variable, so the default
// implementation fails to compare it.
//
// CreateOrUpdate creates or updates the given object in the Kubernetes
// cluster. The object's desired state must be reconciled with the existing
// state inside the passed in callback MutateFn.
//
// The MutateFn is called regardless of creating or updating an object.
//
// It returns the executed operation and an error.
func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {

	key := client.ObjectKeyFromObject(obj)
	if err := c.Get(ctx, key, obj); err != nil {
		if !errors.IsNotFound(err) {
			return controllerutil.OperationResultNone, err
		}
		if err := mutate(f, key, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		if err := c.Create(ctx, obj); err != nil {
			return controllerutil.OperationResultNone, err
		}
		return controllerutil.OperationResultCreated, nil
	}

	existing := obj.DeepCopyObject()
	if err := mutate(f, key, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}

	equality := conversion.EqualitiesOrDie(
		func(a, b resource.Quantity) bool {
			// Ignore formatting, only care that numeric value stayed the same.
			// TODO: if we decide it's important, it should be safe to start comparing the format.
			//
			// Uninitialized quantities are equivalent to 0 quantities.
			return a.Cmp(b) == 0
		},
		func(a, b metav1.MicroTime) bool {
			return a.UTC() == b.UTC()
		},
		func(a, b metav1.Time) bool {
			return a.UTC() == b.UTC()
		},
		func(a, b labels.Selector) bool {
			return a.String() == b.String()
		},
		func(a, b fields.Selector) bool {
			return a.String() == b.String()
		},
		func(a, b argov1alpha1.ApplicationDestination) bool {
			return a.Namespace == b.Namespace && a.Name == b.Name && a.Server == b.Server
		},
	)

	if equality.DeepEqual(existing, obj) {
		return controllerutil.OperationResultNone, nil
	}

	if err := c.Update(ctx, obj); err != nil {
		return controllerutil.OperationResultNone, err
	}
	return controllerutil.OperationResultUpdated, nil
}

// mutate wraps a MutateFn and applies validation to its result
func mutate(f controllerutil.MutateFn, key client.ObjectKey, obj client.Object) error {
	if err := f(); err != nil {
		return err
	}
	if newKey := client.ObjectKeyFromObject(obj); key != newKey {
		return fmt.Errorf("MutateFn cannot mutate object name and/or object namespace")
	}
	return nil
}
