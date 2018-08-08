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
	enforcer := rbac.NewEnforcer(fake.NewSimpleClientset(), "default", common.ArgoCDRBACConfigMapName, nil)
	enforcer.SetBuiltinPolicy(test.BuiltinPolicy)
	enforcer.SetDefaultRole("role:admin")
	enforcer.SetClaimsEnforcerFunc(func(rvals ...interface{}) bool {
		return true
	})
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

	policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

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
		tokenName := "testToken"
		tokenResponse, err := projectServer.CreateToken(context.Background(), &ProjectTokenCreateRequest{Project: projWithoutToken.Name, Token: tokenName, SecondsBeforeExpiry: 1})
		assert.Nil(t, err)
		claims, err := sessionMgr.Parse(tokenResponse.Token)
		assert.Nil(t, err)

		mapClaims, err := jwtUtil.MapClaims(claims)
		subject, ok := mapClaims["sub"].(string)
		assert.True(t, ok)
		expectedSubject := fmt.Sprintf(JwtTokenSubFormat, projWithoutToken.Name, tokenName)
		assert.Equal(t, expectedSubject, subject)
		assert.Nil(t, err)
	})

	t.Run("TestDeleteTokenSuccesfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(&settings.ArgoCDSettings{})
		projWithToken := existingProj.DeepCopy()
		tokenName := "testToken"

		token := v1alpha1.ProjectRole{Name: tokenName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), sessionMgr)
		_, err := projectServer.DeleteToken(context.Background(), &ProjectTokenDeleteRequest{Project: projWithToken.Name, Token: tokenName})
		assert.Nil(t, err)
		projWithoutToken, err := projectServer.Get(context.Background(), &ProjectQuery{Name: projWithToken.Name})
		assert.Len(t, projWithoutToken.Spec.Roles, 0)
	})

	t.Run("TestCreateDuplicateTokenFailure", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(&settings.ArgoCDSettings{})
		projWithToken := existingProj.DeepCopy()
		tokenName := "testToken"
		token := v1alpha1.ProjectRole{Name: tokenName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, util.NewKeyLock(), sessionMgr)
		_, err := projectServer.CreateToken(context.Background(), &ProjectTokenCreateRequest{Project: projWithToken.Name, Token: tokenName})
		expectedError := fmt.Sprintf("rpc error: code = AlreadyExists desc = '%s' token already exist for project '%s'", tokenName, projWithToken.Name)
		assert.EqualError(t, err, expectedError)
	})

	t.Run("TestCreateRolePolicySuccessfully", func(t *testing.T) {
		action := "create"
		object := "testObject"
		roleName := "testRole"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Nil(t, err)
		t.Log(projWithRole.Spec.Roles[0].Policies[0])
		expectedPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, role.Name, action, projWithRole.Name, object)
		assert.Equal(t, projWithRole.Spec.Roles[0].Policies[0], expectedPolicy)
	})

	t.Run("TestValidatePolicyDuplicatePolicyFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		roleName := "testRole"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object)
		role.Policies = append(role.Policies, policy)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = AlreadyExists desc = policy '%s' already exists for role '%s'", policy, roleName)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidateProjectAccessToSeparateProjectObjectFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		roleName := "testRole"
		otherProject := "other-project"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, otherProject, object)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect policy format for '%s' as policies can't grant access to other roles or projects", policy)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidateProjectIncorrectProjectInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		roleName := "testRole"
		otherProject := "other-project"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		invalidPolicy := fmt.Sprintf(policyTemplate, otherProject, roleName, action, projWithRole.Name, object)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect policy format for '%s' as policy can't grant access to other projects", invalidPolicy)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidateProjectIncorrectTokenInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testObject"
		roleName := "testRole"
		policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"
		otherToken := "other-token"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JwtToken: &v1alpha1.JwtToken{CreatedAt: 1}}
		invalidPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, otherToken, action, projWithRole.Name, object)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, util.NewKeyLock(), nil)
		request := &ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = incorrect policy format for '%s' as policy can't grant access to other roles", invalidPolicy)
		assert.EqualError(t, err, expectedErr)
	})
}
