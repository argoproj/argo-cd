package project

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util"
	jwtUtil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

func TestProjectServer(t *testing.T) {
	enforcer := rbac.NewEnforcer(fake.NewSimpleClientset(), nil, "default", common.ArgoCDRBACConfigMapName, nil)
	enforcer.SetBuiltinPolicy(test.BuiltinPolicy)
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

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock(), nil)

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

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock(), nil)

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

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock(), nil)

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

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock(), nil)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{}

		_, err := projectServer.Update(context.Background(), &ProjectUpdateRequest{Project: updatedProj})

		assert.NotNil(t, err)
		assert.Equal(t, codes.InvalidArgument, grpc.Code(err))
	})

	t.Run("TestDeleteProjectSuccessful", func(t *testing.T) {
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj), enforcer, util.NewKeyLock(), nil)

		_, err := projectServer.Delete(context.Background(), &ProjectQuery{Name: "test"})

		assert.Nil(t, err)
	})

	t.Run("TestDeleteProjectReferencedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test"},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, util.NewKeyLock(), nil)

		_, err := projectServer.Delete(context.Background(), &ProjectQuery{Name: "test"})

		assert.NotNil(t, err)
		assert.Equal(t, codes.InvalidArgument, grpc.Code(err))
	})

	t.Run("TestCreateTokenSuccesfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(&settings.ArgoCDSettings{})
		projWithoutToken := existingProj.DeepCopy()
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithoutToken), enforcer, util.NewKeyLock(), sessionMgr)
		tokenName := "test"
		tokenResponse, err := projectServer.CreateToken(context.Background(), &ProjectTokenCreateRequest{Project: projWithoutToken.Name, Token: tokenName, SecondsBeforeExpiry: 1})
		assert.Nil(t, err)
		claims, err := sessionMgr.Parse(tokenResponse.Token)
		assert.Nil(t, err)

		mapClaims, err := jwtUtil.MapClaims(claims)
		subject, ok := mapClaims["sub"].(string)
		assert.True(t, ok)
		assert.Equal(t, "proj:test:test", subject)
		assert.Nil(t, err)
	})

	t.Run("TestDeleteTokenSuccesfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(&settings.ArgoCDSettings{})
		projWithToken := existingProj.DeepCopy()
		tokenName := "test"
		token := v1alpha1.ProjectToken{Name: tokenName}
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), sessionMgr)
		_, err := projectServer.DeleteToken(context.Background(), &ProjectTokenDeleteRequest{Project: projWithToken.Name, Token: tokenName})
		assert.Nil(t, err)
		assert.Len(t, projWithToken.Spec.Tokens, 1)
	})

	t.Run("TestCreateDuplicateTokenFailure", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(&settings.ArgoCDSettings{})
		projWithToken := existingProj.DeepCopy()
		tokenName := "test"
		token := v1alpha1.ProjectToken{Name: tokenName}
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), sessionMgr)
		_, err := projectServer.CreateToken(context.Background(), &ProjectTokenCreateRequest{Project: projWithToken.Name, Token: tokenName})
		assert.EqualError(t, err, "rpc error: code = AlreadyExists desc = 'test' token already exist for project 'test'")
	})

	t.Run("TestCreateTokenPolicySuccessfully", func(t *testing.T) {
		action := "create"
		object := "testObject"
		tokenName := "test"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithToken := existingProj.DeepCopy()
		token := v1alpha1.ProjectToken{Name: tokenName}
		policy := fmt.Sprintf(policyTemplate, projWithToken.Name, tokenName, action, projWithToken.Name, object)
		token.Policies = append(token.Policies, policy)
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithToken}
		_, err := projectServer.Update(context.Background(), request)
		assert.Nil(t, err)
		t.Log(projWithToken.Spec.Tokens[0].Policies[0])
		expectedPolicy := fmt.Sprintf(policyTemplate, projWithToken.Name, token.Name, action, projWithToken.Name, object)
		assert.Equal(t, projWithToken.Spec.Tokens[0].Policies[0], expectedPolicy)
	})

	t.Run("TestCreateTokenPolicyDuplicatePolicyFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		tokenName := "test"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithToken := existingProj.DeepCopy()
		token := v1alpha1.ProjectToken{Name: tokenName}
		policy := fmt.Sprintf(policyTemplate, projWithToken.Name, tokenName, action, projWithToken.Name, object)
		token.Policies = append(token.Policies, policy)
		token.Policies = append(token.Policies, policy)
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithToken}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = AlreadyExists desc = token policy '%s' already exists for token '%s'", policy, tokenName)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidityProjectAccessToSeparateProjectObjectFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		tokenName := "test"
		otherProject := "other-project"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithToken := existingProj.DeepCopy()
		token := v1alpha1.ProjectToken{Name: tokenName}
		policy := fmt.Sprintf(policyTemplate, projWithToken.Name, tokenName, action, otherProject, object)
		token.Policies = append(token.Policies, policy)
		token.Policies = append(token.Policies, policy)
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithToken}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect token policy format for '%s' as token policies can't grant access to other tokens or projects", policy)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidityProjectIncorrectProjectInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		tokenName := "test"
		otherProject := "other-project"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithToken := existingProj.DeepCopy()
		token := v1alpha1.ProjectToken{Name: tokenName}
		invalidPolicy := fmt.Sprintf(policyTemplate, otherProject, tokenName, action, projWithToken.Name, object)
		token.Policies = append(token.Policies, invalidPolicy)
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithToken}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect policy format for '%s' as policy can't grant access to other projects", invalidPolicy)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidityProjectIncorrectTokenInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		tokenName := "test"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"
		otherToken := "other-token"

		projWithToken := existingProj.DeepCopy()
		token := v1alpha1.ProjectToken{Name: tokenName}
		invalidPolicy := fmt.Sprintf(policyTemplate, projWithToken.Name, otherToken, action, projWithToken.Name, object)
		token.Policies = append(token.Policies, invalidPolicy)
		token.Policies = append(token.Policies, invalidPolicy)
		projWithToken.Spec.Tokens = append(projWithToken.Spec.Tokens, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithToken}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect policy format for '%s' as policy can't grant access to other tokens", invalidPolicy)
		assert.EqualError(t, err, expectedErr)
	})
}
