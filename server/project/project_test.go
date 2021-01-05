package project

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/argoproj/pkg/sync"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"

	"github.com/dgrijalva/jwt-go/v4"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	informer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/assets"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"github.com/argoproj/argo-cd/util/settings"
)

const testNamespace = "default"

func TestProjectServer(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-cm",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}, &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeclientset, testNamespace)
	enforcer := newEnforcer(kubeclientset)
	existingProj := v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: testNamespace},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{
				{Namespace: "ns1", Server: "https://server1"},
				{Namespace: "ns2", Server: "https://server2"},
			},
			SourceRepos: []string{"https://github.com/argoproj/argo-cd.git"},
		},
	}
	existingApp := v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{Namespace: "ns3", Server: "https://server3"}},
	}

	policyTemplate := "p, proj:%s:%s, applications, %s, %s/%s, %s"

	ctx := context.Background()
	fakeAppsClientset := apps.NewSimpleClientset()
	factory := informer.NewFilteredSharedInformerFactory(fakeAppsClientset, 0, "", func(options *metav1.ListOptions) {})
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	go projInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), projInformer.HasSynced) {
		panic("Timed out waiting forfff caches to sync")
	}

	t.Run("TestNormalizeProj", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		roleName := "roleName"
		role1 := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		projectWithRole.Spec.Roles = append(projectWithRole.Spec.Roles, role1)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, nil, projInformer, settingsMgr)
		err := projectServer.NormalizeProjs()
		assert.NoError(t, err)

		appList, err := projectServer.appclientset.ArgoprojV1alpha1().AppProjects(projectWithRole.Namespace).List(context.Background(), v1.ListOptions{})
		assert.NoError(t, err)
		assert.Equal(t, appList.Items[0].Status.JWTTokensByRole[roleName].Items[0].IssuedAt, int64(1))
		assert.ElementsMatch(t, appList.Items[0].Status.JWTTokensByRole[roleName].Items, appList.Items[0].Spec.Roles[0].JWTTokens)

	})

	t.Run("TestClusterUpdateDenied", func(t *testing.T) {

		enforcer.SetDefaultRole("role:projects")
		_ = enforcer.SetBuiltinPolicy("p, role:projects, projects, update, *, allow")
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.Destinations = nil

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Equal(t, status.Error(codes.PermissionDenied, "permission denied: clusters, update, https://server1"), err)
	})

	t.Run("TestReposUpdateDenied", func(t *testing.T) {

		enforcer.SetDefaultRole("role:projects")
		_ = enforcer.SetBuiltinPolicy("p, role:projects, projects, update, *, allow")
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = nil

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Equal(t, status.Error(codes.PermissionDenied, "permission denied: repositories, update, https://github.com/argoproj/argo-cd.git"), err)
	})

	t.Run("TestClusterResourceWhitelistUpdateDenied", func(t *testing.T) {

		enforcer.SetDefaultRole("role:projects")
		_ = enforcer.SetBuiltinPolicy("p, role:projects, projects, update, *, allow")
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.ClusterResourceWhitelist = []metav1.GroupKind{{}}

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Equal(t, status.Error(codes.PermissionDenied, "permission denied: clusters, update, https://server1"), err)
	})

	t.Run("TestNamespaceResourceBlacklistUpdateDenied", func(t *testing.T) {

		enforcer.SetDefaultRole("role:projects")
		_ = enforcer.SetBuiltinPolicy("p, role:projects, projects, update, *, allow")
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.NamespaceResourceBlacklist = []metav1.GroupKind{{}}

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Equal(t, status.Error(codes.PermissionDenied, "permission denied: clusters, update, https://server1"), err)
	})

	enforcer = newEnforcer(kubeclientset)

	t.Run("TestRemoveDestinationSuccessful", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{Namespace: "ns3", Server: "https://server3"}},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.Destinations = updatedProj.Spec.Destinations[1:]

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Nil(t, err)
	})

	t.Run("TestRemoveDestinationUsedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{Namespace: "ns1", Server: "https://server1"}},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.Destinations = updatedProj.Spec.Destinations[1:]

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.NotNil(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
	})

	t.Run("TestRemoveSourceSuccessful", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test"},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{}

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.Nil(t, err)
	})

	t.Run("TestRemoveSourceUsedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Source: v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argo-cd.git"}},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := existingProj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{}

		_, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.NotNil(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
	})

	t.Run("TestRemoveSourceUsedByAppSuccessfulIfPermittedByAnotherSrc", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.SourceRepos = []string{"https://github.com/argoproj/argo-cd.git", "https://github.com/argoproj/*"}
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test", Source: v1alpha1.ApplicationSource{RepoURL: "https://github.com/argoproj/argo-cd.git"}},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(proj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := proj.DeepCopy()
		updatedProj.Spec.SourceRepos = []string{"https://github.com/argoproj/*"}

		res, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.NoError(t, err)
		assert.ElementsMatch(t, res.Spec.SourceRepos, updatedProj.Spec.SourceRepos)
	})

	t.Run("TestRemoveDestinationUsedByAppSuccessfulIfPermittedByAnotherDestination", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.Destinations = []v1alpha1.ApplicationDestination{
			{Namespace: "org1-team1", Server: "https://server1"},
			{Namespace: "org1-*", Server: "https://server1"},
		}
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec: v1alpha1.ApplicationSpec{Project: "test", Destination: v1alpha1.ApplicationDestination{
				Server:    "https://server1",
				Namespace: "org1-team1",
			}},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(proj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		updatedProj := proj.DeepCopy()
		updatedProj.Spec.Destinations = []v1alpha1.ApplicationDestination{
			{Namespace: "org1-*", Server: "https://server1"},
		}

		res, err := projectServer.Update(context.Background(), &project.ProjectUpdateRequest{Project: updatedProj})

		assert.NoError(t, err)
		assert.ElementsMatch(t, res.Spec.Destinations, updatedProj.Spec.Destinations)
	})

	t.Run("TestDeleteProjectSuccessful", func(t *testing.T) {
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		_, err := projectServer.Delete(context.Background(), &project.ProjectQuery{Name: "test"})

		assert.Nil(t, err)
	})

	t.Run("TestDeleteDefaultProjectFailure", func(t *testing.T) {
		defaultProj := v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{Name: "default", Namespace: "default"},
			Spec:       v1alpha1.AppProjectSpec{},
		}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&defaultProj), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		_, err := projectServer.Delete(context.Background(), &project.ProjectQuery{Name: defaultProj.Name})
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
	})

	t.Run("TestDeleteProjectReferencedByApp", func(t *testing.T) {
		existingApp := v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "default"},
			Spec:       v1alpha1.ApplicationSpec{Project: "test"},
		}

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(&existingProj, &existingApp), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)

		_, err := projectServer.Delete(context.Background(), &project.ProjectQuery{Name: "test"})

		assert.NotNil(t, err)
		statusCode, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, statusCode.Code())
	})

	// configure a user named "admin" which is denied by default
	enforcer = newEnforcer(kubeclientset)
	_ = enforcer.SetBuiltinPolicy(`p, *, *, *, *, deny`)
	enforcer.SetClaimsEnforcerFunc(nil)
	// nolint:staticcheck
	ctx = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"my-group"}})
	policyEnf := rbacpolicy.NewRBACPolicyEnforcer(enforcer, nil)
	policyEnf.SetScopes([]string{"groups"})

	tokenName := "testToken"
	id := "testId"

	t.Run("TestCreateTokenDenied", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		projectWithRole.Spec.Roles = []v1alpha1.ProjectRole{{Name: tokenName}}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.CreateToken(ctx, &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1})
		assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = permission denied: projects, update, test")
	})

	t.Run("TestCreateTokenSuccessfullyUsingGroup", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		projectWithRole.Spec.Roles = []v1alpha1.ProjectRole{{Name: tokenName, Groups: []string{"my-group"}}}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.CreateToken(ctx, &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1})
		assert.NoError(t, err)
	})

	_ = enforcer.SetBuiltinPolicy(`p, role:admin, projects, update, *, allow`)

	t.Run("TestCreateTokenSuccessfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		projectWithRole.Spec.Roles = []v1alpha1.ProjectRole{{Name: tokenName}}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		tokenResponse, err := projectServer.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1})
		assert.NoError(t, err)
		claims, err := sessionMgr.Parse(tokenResponse.Token)
		assert.NoError(t, err)

		mapClaims, err := jwtutil.MapClaims(claims)
		subject, ok := mapClaims["sub"].(string)
		assert.True(t, ok)
		expectedSubject := fmt.Sprintf(JWTTokenSubFormat, projectWithRole.Name, tokenName)
		assert.Equal(t, expectedSubject, subject)
		assert.NoError(t, err)
	})

	t.Run("TestCreateTokenWithIDSuccessfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		projectWithRole.Spec.Roles = []v1alpha1.ProjectRole{{Name: tokenName}}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		tokenResponse, err := projectServer.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1, Id: id})
		assert.NoError(t, err)
		claims, err := sessionMgr.Parse(tokenResponse.Token)
		assert.NoError(t, err)

		mapClaims, err := jwtutil.MapClaims(claims)
		subject, ok := mapClaims["sub"].(string)
		assert.True(t, ok)
		expectedSubject := fmt.Sprintf(JWTTokenSubFormat, projectWithRole.Name, tokenName)
		assert.Equal(t, expectedSubject, subject)
		assert.NoError(t, err)
	})

	t.Run("TestCreateTokenWithSameIdDeny", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithRole := existingProj.DeepCopy()
		projectWithRole.Spec.Roles = []v1alpha1.ProjectRole{{Name: tokenName}}
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithRole), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		tokenResponse, err := projectServer.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1, Id: id})

		assert.NoError(t, err)
		claims, err := sessionMgr.Parse(tokenResponse.Token)
		assert.NoError(t, err)

		mapClaims, err := jwtutil.MapClaims(claims)
		subject, ok := mapClaims["sub"].(string)
		assert.True(t, ok)
		expectedSubject := fmt.Sprintf(JWTTokenSubFormat, projectWithRole.Name, tokenName)
		assert.Equal(t, expectedSubject, subject)
		assert.NoError(t, err)

		_, err1 := projectServer.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projectWithRole.Name, Role: tokenName, ExpiresIn: 1, Id: id})
		expectedErr := fmt.Sprintf("rpc error: code = InvalidArgument desc = rpc error: code = InvalidArgument desc = Token id '%s' has been used. ", id)
		assert.EqualError(t, err1, expectedErr)
	})

	_ = enforcer.SetBuiltinPolicy(`p, *, *, *, *, deny`)

	t.Run("TestDeleteTokenDenied", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projWithToken := existingProj.DeepCopy()
		issuedAt := int64(1)
		secondIssuedAt := issuedAt + 1
		token := v1alpha1.ProjectRole{Name: tokenName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: issuedAt}, {IssuedAt: secondIssuedAt}}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.DeleteToken(ctx, &project.ProjectTokenDeleteRequest{Project: projWithToken.Name, Role: tokenName, Iat: issuedAt})
		assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = permission denied: projects, update, test")
	})

	t.Run("TestDeleteTokenSuccessfullyWithGroup", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projWithToken := existingProj.DeepCopy()
		issuedAt := int64(1)
		secondIssuedAt := issuedAt + 1
		token := v1alpha1.ProjectRole{Name: tokenName, Groups: []string{"my-group"}, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: issuedAt}, {IssuedAt: secondIssuedAt}}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.DeleteToken(ctx, &project.ProjectTokenDeleteRequest{Project: projWithToken.Name, Role: tokenName, Iat: issuedAt})
		assert.NoError(t, err)
	})

	_ = enforcer.SetBuiltinPolicy(`p, role:admin, projects, get, *, allow
p, role:admin, projects, update, *, allow`)

	t.Run("TestDeleteTokenSuccessfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projWithToken := existingProj.DeepCopy()
		issuedAt := int64(1)
		secondIssuedAt := issuedAt + 1
		token := v1alpha1.ProjectRole{Name: tokenName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: issuedAt}, {IssuedAt: secondIssuedAt}}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.DeleteToken(ctx, &project.ProjectTokenDeleteRequest{Project: projWithToken.Name, Role: tokenName, Iat: issuedAt})
		assert.NoError(t, err)
		projWithoutToken, err := projectServer.Get(context.Background(), &project.ProjectQuery{Name: projWithToken.Name})
		assert.NoError(t, err)
		assert.Len(t, projWithoutToken.Spec.Roles, 1)
		assert.Len(t, projWithoutToken.Spec.Roles[0].JWTTokens, 1)
		assert.Equal(t, projWithoutToken.Spec.Roles[0].JWTTokens[0].IssuedAt, secondIssuedAt)
	})

	_ = enforcer.SetBuiltinPolicy(`p, role:admin, projects, get, *, allow
p, role:admin, projects, update, *, allow`)

	t.Run("TestDeleteTokenByIdSuccessfully", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projWithToken := existingProj.DeepCopy()
		issuedAt := int64(1)
		secondIssuedAt := issuedAt + 1
		id := "testId"
		uniqueId, _ := uuid.NewRandom()
		secondId := uniqueId.String()
		token := v1alpha1.ProjectRole{Name: tokenName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: issuedAt, ID: id}, {IssuedAt: secondIssuedAt, ID: secondId}}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.DeleteToken(ctx, &project.ProjectTokenDeleteRequest{Project: projWithToken.Name, Role: tokenName, Iat: secondIssuedAt, Id: id})
		assert.NoError(t, err)
		projWithoutToken, err := projectServer.Get(context.Background(), &project.ProjectQuery{Name: projWithToken.Name})
		assert.NoError(t, err)
		assert.Len(t, projWithoutToken.Spec.Roles, 1)
		assert.Len(t, projWithoutToken.Spec.Roles[0].JWTTokens, 1)
		assert.Equal(t, projWithoutToken.Spec.Roles[0].JWTTokens[0].IssuedAt, secondIssuedAt)
	})

	enforcer = newEnforcer(kubeclientset)

	t.Run("TestCreateTwoTokensInRoleSuccess", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projWithToken := existingProj.DeepCopy()
		tokenName := "testToken"
		token := v1alpha1.ProjectRole{Name: tokenName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		projWithToken.Spec.Roles = append(projWithToken.Spec.Roles, token)
		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithToken), enforcer, sync.NewKeyLock(), sessionMgr, policyEnf, projInformer, settingsMgr)
		_, err := projectServer.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projWithToken.Name, Role: tokenName})
		assert.Nil(t, err)
		projWithTwoTokens, err := projectServer.Get(context.Background(), &project.ProjectQuery{Name: projWithToken.Name})
		assert.Nil(t, err)
		assert.Len(t, projWithTwoTokens.Spec.Roles, 1)
		assert.Len(t, projWithTwoTokens.Spec.Roles[0].JWTTokens, 2)
	})

	t.Run("TestAddWildcardSource", func(t *testing.T) {

		proj := existingProj.DeepCopy()
		wildSourceRepo := "*"
		proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, wildSourceRepo)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(proj), enforcer, sync.NewKeyLock(), nil, policyEnf, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: proj}
		updatedProj, err := projectServer.Update(context.Background(), request)
		assert.Nil(t, err)
		assert.Equal(t, wildSourceRepo, updatedProj.Spec.SourceRepos[1])
	})

	t.Run("TestCreateRolePolicySuccessfully", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, policyEnf, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Nil(t, err)
		t.Log(projWithRole.Spec.Roles[0].Policies[0])
		expectedPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, role.Name, action, projWithRole.Name, object, effect)
		assert.Equal(t, projWithRole.Spec.Roles[0].Policies[0], expectedPolicy)
	})

	t.Run("TestValidatePolicyDuplicatePolicyFailure", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, policy)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		expectedErr := fmt.Sprintf("rpc error: code = AlreadyExists desc = policy '%s' already exists for role '%s'", policy, roleName)
		assert.EqualError(t, err, expectedErr)
	})

	t.Run("TestValidateProjectAccessToSeparateProjectObjectFailure", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		otherProject := "other-project"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		policy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, otherProject, object, effect)
		role.Policies = append(role.Policies, policy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Contains(t, err.Error(), "object must be of form 'test/*' or 'test/<APPNAME>'")
	})

	t.Run("TestValidateProjectIncorrectProjectInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		otherProject := "other-project"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		invalidPolicy := fmt.Sprintf(policyTemplate, otherProject, roleName, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Contains(t, err.Error(), "policy subject must be: 'proj:test:testRole'")
	})

	t.Run("TestValidateProjectIncorrectTokenInRoleFailure", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		otherToken := "other-token"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		invalidPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, otherToken, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Contains(t, err.Error(), "policy subject must be: 'proj:test:testRole'")
	})

	t.Run("TestValidateProjectInvalidEffectFailure", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		effect := "testEffect"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		invalidPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		_, err := projectServer.Update(context.Background(), request)
		assert.Contains(t, err.Error(), "effect must be: 'allow' or 'deny'")
	})

	t.Run("TestNormalizeProjectRolePolicies", func(t *testing.T) {
		action := "create"
		object := "testApplication"
		roleName := "testRole"
		effect := "allow"

		projWithRole := existingProj.DeepCopy()
		role := v1alpha1.ProjectRole{Name: roleName, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: 1}}}
		noSpacesPolicyTemplate := strings.Replace(policyTemplate, " ", "", -1)
		invalidPolicy := fmt.Sprintf(noSpacesPolicyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object, effect)
		role.Policies = append(role.Policies, invalidPolicy)
		projWithRole.Spec.Roles = append(projWithRole.Spec.Roles, role)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projWithRole), enforcer, sync.NewKeyLock(), nil, nil, projInformer, settingsMgr)
		request := &project.ProjectUpdateRequest{Project: projWithRole}
		updateProj, err := projectServer.Update(context.Background(), request)
		assert.Nil(t, err)
		expectedPolicy := fmt.Sprintf(policyTemplate, projWithRole.Name, roleName, action, projWithRole.Name, object, effect)
		assert.Equal(t, expectedPolicy, updateProj.Spec.Roles[0].Policies[0])
	})

	t.Run("TestSyncWindowsActive", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithSyncWindows := existingProj.DeepCopy()
		projectWithSyncWindows.Spec.SyncWindows = v1alpha1.SyncWindows{}
		win := &v1alpha1.SyncWindow{Kind: "allow", Schedule: "* * * * *", Duration: "1h"}
		projectWithSyncWindows.Spec.SyncWindows = append(projectWithSyncWindows.Spec.SyncWindows, win)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithSyncWindows), enforcer, sync.NewKeyLock(), sessionMgr, nil, projInformer, settingsMgr)
		res, err := projectServer.GetSyncWindowsState(ctx, &project.SyncWindowsQuery{Name: projectWithSyncWindows.Name})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(res.Windows))
	})

	t.Run("TestGetSyncWindowsStateCannotGetProjectDetails", func(t *testing.T) {
		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithSyncWindows := existingProj.DeepCopy()
		projectWithSyncWindows.Spec.SyncWindows = v1alpha1.SyncWindows{}
		win := &v1alpha1.SyncWindow{Kind: "allow", Schedule: "* * * * *", Duration: "1h"}
		projectWithSyncWindows.Spec.SyncWindows = append(projectWithSyncWindows.Spec.SyncWindows, win)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithSyncWindows), enforcer, sync.NewKeyLock(), sessionMgr, nil, projInformer, settingsMgr)
		res, err := projectServer.GetSyncWindowsState(ctx, &project.SyncWindowsQuery{Name: "incorrect"})
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, res)
	})

	t.Run("TestGetSyncWindowsStateDenied", func(t *testing.T) {
		enforcer = newEnforcer(kubeclientset)
		_ = enforcer.SetBuiltinPolicy(`p, *, *, *, *, deny`)
		enforcer.SetClaimsEnforcerFunc(nil)
		// nolint:staticcheck
		ctx := context.WithValue(context.Background(), "claims", &jwt.MapClaims{"groups": []string{"my-group"}})

		sessionMgr := session.NewSessionManager(settingsMgr, "", session.NewInMemoryUserStateStorage())
		projectWithSyncWindows := existingProj.DeepCopy()
		win := &v1alpha1.SyncWindow{Kind: "allow", Schedule: "* * * * *", Duration: "1h"}
		projectWithSyncWindows.Spec.SyncWindows = append(projectWithSyncWindows.Spec.SyncWindows, win)

		projectServer := NewServer("default", fake.NewSimpleClientset(), apps.NewSimpleClientset(projectWithSyncWindows), enforcer, sync.NewKeyLock(), sessionMgr, nil, projInformer, settingsMgr)
		_, err := projectServer.GetSyncWindowsState(ctx, &project.SyncWindowsQuery{Name: projectWithSyncWindows.Name})
		assert.EqualError(t, err, "rpc error: code = PermissionDenied desc = permission denied: projects, get, test")
	})

}

func newEnforcer(kubeclientset *fake.Clientset) *rbac.Enforcer {
	enforcer := rbac.NewEnforcer(kubeclientset, testNamespace, common.ArgoCDRBACConfigMapName, nil)
	_ = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	enforcer.SetDefaultRole("role:admin")
	enforcer.SetClaimsEnforcerFunc(func(claims jwt.Claims, rvals ...interface{}) bool {
		return true
	})
	return enforcer
}
