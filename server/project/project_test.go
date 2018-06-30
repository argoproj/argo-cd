package project

import (
	"context"
	"testing"

	"github.com/gobuffalo/packr"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/rbac"
)

var (
	box           = packr.NewBox("../../util/rbac")
	builtinPolicy string
)

func TestProjectServer(t *testing.T) {
	enforcer := rbac.NewEnforcer(fake.NewSimpleClientset(), "default", common.ArgoCDRBACConfigMapName, nil)
	builtinPolicy, err := box.MustString("builtin-policy.csv")
	assert.Nil(t, err)
	enforcer.SetBuiltinPolicy(builtinPolicy)
	enforcer.SetDefaultRole("role:admin")
	existingProj := v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{
				{Namespace: "ns1", Server: "https://server1"},
				{Namespace: "ns2", Server: "https://server2"},
			},
			SourceRepos: []string{"https://github.com/argoproj/argo-cd.git"},
		},
	}

	t.Run("TestRemoveDestinationSuccessful", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{Namespace: "ns3", Server: "https://server3"}},
		}

		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock())

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.Destinations = updatedProj.Spec.Destinations[1:]

		_, err := projectServer.Update(context.Background(), &ProjectUpdateRequest{Project: updatedProj})

		assert.Nil(t, err)
	})

	t.Run("TestRemoveDestinationUsedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{Namespace: "ns1", Server: "https://server1"}},
		}

		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock())

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.Destinations = updatedProj.Spec.Destinations[1:]

		_, err := projectServer.Update(context.Background(), &ProjectUpdateRequest{Project: updatedProj})

		assert.NotNil(t, err)
		assert.Equal(t, codes.InvalidArgument, grpc.Code(err))
	})

	t.Run("TestRemoveSourceSuccessful", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test"},
		}

		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock())

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{}

		_, err := projectServer.Update(context.Background(), &ProjectUpdateRequest{Project: updatedProj})

		assert.Nil(t, err)
	})

	t.Run("TestRemoveSourceUsedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Source: v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argo-cd.git"}},
		}

		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock())

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{}

		_, err := projectServer.Update(context.Background(), &ProjectUpdateRequest{Project: updatedProj})

		assert.NotNil(t, err)
		assert.Equal(t, codes.InvalidArgument, grpc.Code(err))
	})

	t.Run("TestDeleteProjectSuccessful", func(t *testing.T) {
		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj), enforcer, util.NewKeyLock())

		_, err := projectServer.Delete(context.Background(), &ProjectQuery{Name: "test"})

		assert.Nil(t, err)
	})

	t.Run("TestDeleteProjectReferencedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test"},
		}

		projectServer := NewServer("default", apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock())

		_, err := projectServer.Delete(context.Background(), &ProjectQuery{Name: "test"})

		assert.NotNil(t, err)
		assert.Equal(t, codes.InvalidArgument, grpc.Code(err))
	})
}
