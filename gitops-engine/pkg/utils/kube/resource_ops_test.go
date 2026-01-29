package kube

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

func TestExecuteCreateWithRecovery(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm",
				"namespace": "default",
			},
		},
	}

	k := &kubectlResourceOperations{
		config: &rest.Config{},
		log:    textlogger.NewLogger(textlogger.NewConfig()),
		tracer: tracing.NopTracer{},
	}

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		executor := func() error {
			return nil
		}
		err := k.executeCreateWithRecovery(ctx, obj, executor, nil, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("AlreadyExists_AndExistsInCluster_ReturnsSuccess", func(t *testing.T) {
		mockDyn := new(mockDynamicInterface)
		mockNS := new(mockNamespaceableResourceInterface)
		mockRes := new(mockResourceInterface)

		mockDyn.On("Resource", mock.Anything).Return(mockNS)
		mockNS.On("Namespace", "default").Return(mockRes)
		mockRes.On("Get", mock.Anything, "test-cm", mock.Anything, mock.Anything).Return(obj, nil)

		newDyn := func(config *rest.Config) (dynamic.Interface, error) {
			return mockDyn, nil
		}
		newDisco := func(config *rest.Config) (discovery.DiscoveryInterface, error) {
			return nil, nil
		}
		serverRes := func(disco discovery.DiscoveryInterface, gvk schema.GroupVersionKind, verb string) (*metav1.APIResource, error) {
			return &metav1.APIResource{Name: "configmaps", Namespaced: true}, nil
		}

		executor := func() error {
			return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, "test-cm")
		}

		err := k.executeCreateWithRecovery(ctx, obj, executor, newDyn, newDisco, serverRes)
		assert.NoError(t, err)
		mockRes.AssertExpectations(t)
	})

	t.Run("AlreadyExists_ButDoesNotExistInCluster_ReturnsOriginalError", func(t *testing.T) {
		mockDyn := new(mockDynamicInterface)
		mockNS := new(mockNamespaceableResourceInterface)
		mockRes := new(mockResourceInterface)

		mockDyn.On("Resource", mock.Anything).Return(mockNS)
		mockNS.On("Namespace", "default").Return(mockRes)
		mockRes.On("Get", mock.Anything, "test-cm", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))

		newDyn := func(config *rest.Config) (dynamic.Interface, error) {
			return mockDyn, nil
		}
		newDisco := func(config *rest.Config) (discovery.DiscoveryInterface, error) {
			return nil, nil
		}
		serverRes := func(disco discovery.DiscoveryInterface, gvk schema.GroupVersionKind, verb string) (*metav1.APIResource, error) {
			return &metav1.APIResource{Name: "configmaps", Namespaced: true}, nil
		}

		origErr := apierrors.NewAlreadyExists(schema.GroupResource{Resource: "configmaps"}, "test-cm")
		executor := func() error {
			return origErr
		}

		err := k.executeCreateWithRecovery(ctx, obj, executor, newDyn, newDisco, serverRes)
		assert.Equal(t, origErr, err)
	})

	t.Run("OtherError_ReturnsError", func(t *testing.T) {
		otherErr := errors.New("some other error")
		executor := func() error {
			return otherErr
		}

		err := k.executeCreateWithRecovery(ctx, obj, executor, nil, nil, nil)
		assert.Equal(t, otherErr, err)
	})
}

type mockDynamicInterface struct {
	mock.Mock
	dynamic.Interface
}

func (m *mockDynamicInterface) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	args := m.Called(resource)
	return args.Get(0).(dynamic.NamespaceableResourceInterface)
}

type mockNamespaceableResourceInterface struct {
	mock.Mock
	dynamic.NamespaceableResourceInterface
}

func (m *mockNamespaceableResourceInterface) Namespace(ns string) dynamic.ResourceInterface {
	args := m.Called(ns)
	return args.Get(0).(dynamic.ResourceInterface)
}

type mockResourceInterface struct {
	mock.Mock
	dynamic.ResourceInterface
}

func (m *mockResourceInterface) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, name, options, subresources)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

