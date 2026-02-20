package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
)

func newTestService(t *testing.T, objects ...runtime.Object) (*argoCDService, *dynfake.FakeDynamicClient) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.SchemeBuilder.AddToScheme(scheme))

	dynamicClient := dynfake.NewSimpleDynamicClient(scheme, objects...)
	k8sClient := fake.NewSimpleClientset()
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	svc, err := NewArgoCDService(k8sClient, dynamicClient, "default", mockRepoClient)
	require.NoError(t, err)
	t.Cleanup(svc.Close)
	return svc, dynamicClient
}

func TestGetAppProject(t *testing.T) {
	appProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-project",
			Namespace: "default",
		},
		Spec: v1alpha1.AppProjectSpec{
			Description: "test project",
		},
	}

	t.Run("returns AppProject when found", func(t *testing.T) {
		svc, _ := newTestService(t, appProject)
		result, err := svc.GetAppProject(context.Background(), "my-project", "default")
		require.NoError(t, err)
		assert.Equal(t, "my-project", result.GetName())
		assert.Equal(t, "default", result.GetNamespace())
	})

	t.Run("defaults to 'default' project when name is empty", func(t *testing.T) {
		defaultProject := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "default",
			},
		}
		svc, _ := newTestService(t, defaultProject)
		result, err := svc.GetAppProject(context.Background(), "", "default")
		require.NoError(t, err)
		assert.Equal(t, "default", result.GetName())
	})

	t.Run("returns error when AppProject not found", func(t *testing.T) {
		svc, _ := newTestService(t)
		result, err := svc.GetAppProject(context.Background(), "nonexistent", "default")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
