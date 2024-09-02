package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	dynfake "k8s.io/client-go/dynamic/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/oidc"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
	testutil "github.com/argoproj/argo-cd/v2/util/test"
)

type FakeArgoCDServer struct {
	*ArgoCDServer
	TmpAssetsDir string
}

func fakeServer(t *testing.T) (*FakeArgoCDServer, func()) {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()
	redis, closer := test.NewInMemoryRedis()
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
	tmpAssetsDir := t.TempDir()
	dynamicClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	fakeClient := clientfake.NewClientBuilder().Build()

	port, err := test.GetFreePort()
	if err != nil {
		panic(err)
	}

	argoCDOpts := ArgoCDServerOpts{
		ListenPort:            port,
		Namespace:             test.FakeArgoCDNamespace,
		KubeClientset:         kubeclientset,
		AppClientset:          appClientSet,
		Insecure:              true,
		DisableAuth:           true,
		XFrameOptions:         "sameorigin",
		ContentSecurityPolicy: "frame-ancestors 'self';",
		Cache: servercache.NewCache(
			appstatecache.NewCache(
				cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
				1*time.Minute,
			),
			1*time.Minute,
			1*time.Minute,
			1*time.Minute,
		),
		RedisClient:             redis,
		RepoClientset:           mockRepoClient,
		StaticAssetsDir:         tmpAssetsDir,
		DynamicClientset:        dynamicClient,
		KubeControllerClientset: fakeClient,
	}
	srv := NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})
	fakeSrv := &FakeArgoCDServer{srv, tmpAssetsDir}
	return fakeSrv, closer
}

func TestEnforceProjectToken(t *testing.T) {
	projectName := "testProj"
	roleName := "testRole"
	subFormat := "proj:%s:%s"
	policyTemplate := "p, %s, applications, get, %s/%s, %s"
	defaultObject := "*"
	defaultEffect := "allow"
	defaultTestObject := fmt.Sprintf("%s/%s", projectName, "test")
	defaultIssuedAt := int64(1)
	defaultSub := fmt.Sprintf(subFormat, projectName, roleName)
	defaultPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)
	defaultId := "testId"

	role := v1alpha1.ProjectRole{Name: roleName, Policies: []string{defaultPolicy}, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: defaultIssuedAt}, {ID: defaultId}}}

	jwtTokenByRole := make(map[string]v1alpha1.JWTTokens)
	jwtTokenByRole[roleName] = v1alpha1.JWTTokens{Items: []v1alpha1.JWTToken{{IssuedAt: defaultIssuedAt}, {ID: defaultId}}}

	existingProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName, Namespace: test.FakeArgoCDNamespace},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{role},
		},
		Status: v1alpha1.AppProjectStatus{JWTTokensByRole: jwtTokenByRole},
	}
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	t.Run("TestEnforceProjectTokenSuccessful", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		cancel := test.StartInformer(s.projInformer)
		defer cancel()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
		assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
		assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenWithDiffCreateAtFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		diffCreateAt := defaultIssuedAt + 1
		claims := jwt.MapClaims{"sub": defaultSub, "iat": diffCreateAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenIncorrectSubFormatFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		invalidSub := "proj:test"
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultIssuedAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenNoTokenFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		nonExistentToken := "fake-token"
		invalidSub := fmt.Sprintf(subFormat, projectName, nonExistentToken)
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultIssuedAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenNotJWTTokenFailure", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0].JWTTokens = nil
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenExplicitDeny", func(t *testing.T) {
		denyApp := "testDenyApp"
		allowPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)
		denyPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, denyApp, "deny")
		role := v1alpha1.ProjectRole{Name: roleName, Policies: []string{allowPolicy, denyPolicy}, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: defaultIssuedAt}}}
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0] = role

		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		cancel := test.StartInformer(s.projInformer)
		defer cancel()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
		allowedObject := fmt.Sprintf("%s/%s", projectName, "test")
		denyObject := fmt.Sprintf("%s/%s", projectName, denyApp)
		assert.True(t, s.enf.Enforce(claims, "applications", "get", allowedObject))
		assert.False(t, s.enf.Enforce(claims, "applications", "get", denyObject))
	})

	t.Run("TestEnforceProjectTokenWithIdSuccessful", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		cancel := test.StartInformer(s.projInformer)
		defer cancel()
		claims := jwt.MapClaims{"sub": defaultSub, "jti": defaultId}
		assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
		assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenWithInvalidIdFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
		invalidId := "invalidId"
		claims := jwt.MapClaims{"sub": defaultSub, "jti": defaultId}
		res := s.enf.Enforce(claims, "applications", "get", invalidId)
		assert.False(t, res)
	})
}

func TestEnforceClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	rbacEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, test.NewFakeProjLister())
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)
	policy := `
g, org2:team2, role:admin
g, bob, role:admin
`
	_ = enf.SetUserPolicy(policy)
	allowed := []jwt.Claims{
		jwt.MapClaims{"groups": []string{"org1:team1", "org2:team2"}},
		jwt.RegisteredClaims{Subject: "admin"},
	}
	for _, c := range allowed {
		if !assert.True(t, enf.Enforce(c, "applications", "delete", "foo/obj")) {
			log.Errorf("%v: expected true, got false", c)
		}
	}

	disallowed := []jwt.Claims{
		jwt.MapClaims{"groups": []string{"org3:team3"}},
		jwt.RegisteredClaims{Subject: "nobody"},
	}
	for _, c := range disallowed {
		if !assert.False(t, enf.Enforce(c, "applications", "delete", "foo/obj")) {
			log.Errorf("%v: expected true, got false", c)
		}
	}
}

func TestDefaultRoleWithClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	rbacEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, test.NewFakeProjLister())
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)
	claims := jwt.MapClaims{"groups": []string{"org1:team1", "org2:team2"}}

	assert.False(t, enf.Enforce(claims, "applications", "get", "foo/bar"))
	// after setting the default role to be the read-only role, this should now pass
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.Enforce(claims, "applications", "get", "foo/bar"))
}

func TestEnforceNilClaims(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap())
	enf := rbac.NewEnforcer(kubeclientset, test.FakeArgoCDNamespace, common.ArgoCDConfigMapName, nil)
	_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	rbacEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, test.NewFakeProjLister())
	enf.SetClaimsEnforcerFunc(rbacEnf.EnforceClaims)
	assert.False(t, enf.Enforce(nil, "applications", "get", "foo/obj"))
	enf.SetDefaultRole("role:readonly")
	assert.True(t, enf.Enforce(nil, "applications", "get", "foo/obj"))
}

func TestInitializingExistingDefaultProject(t *testing.T) {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	defaultProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.DefaultAppProjectName, Namespace: test.FakeArgoCDNamespace},
		Spec:       v1alpha1.AppProjectSpec{},
	}
	appClientSet := apps.NewSimpleClientset(defaultProj)

	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
		RepoClientset: mockRepoClient,
	}

	argocd := NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})
	assert.NotNil(t, argocd)

	proj, err := appClientSet.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Get(context.Background(), v1alpha1.DefaultAppProjectName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, proj)
	assert.Equal(t, v1alpha1.DefaultAppProjectName, proj.Name)
}

func TestInitializingNotExistingDefaultProject(t *testing.T) {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
		RepoClientset: mockRepoClient,
	}

	argocd := NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})
	assert.NotNil(t, argocd)

	proj, err := appClientSet.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Get(context.Background(), v1alpha1.DefaultAppProjectName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, proj)
	assert.Equal(t, v1alpha1.DefaultAppProjectName, proj.Name)
}

func TestEnforceProjectGroups(t *testing.T) {
	projectName := "testProj"
	roleName := "testRole"
	subFormat := "proj:%s:%s"
	policyTemplate := "p, %s, applications, get, %s/%s, %s"
	groupName := "my-org:my-team"

	defaultObject := "*"
	defaultEffect := "allow"
	defaultTestObject := fmt.Sprintf("%s/%s", projectName, "test")
	defaultIssuedAt := int64(1)
	defaultSub := fmt.Sprintf(subFormat, projectName, roleName)
	defaultPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)

	existingProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      projectName,
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{
				{
					Name:     roleName,
					Policies: []string{defaultPolicy},
					Groups: []string{
						groupName,
					},
				},
			},
		},
	}
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret())
	s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
	cancel := test.StartInformer(s.projInformer)
	defer cancel()
	claims := jwt.MapClaims{
		"iat":    defaultIssuedAt,
		"groups": []string{groupName},
	}
	assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
	assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	assert.False(t, s.enf.Enforce(claims, "clusters", "get", "test"))

	// now remove the group and make sure it fails
	log.Println(existingProj.ProjectPoliciesString())
	existingProj.Spec.Roles[0].Groups = nil
	log.Println(existingProj.ProjectPoliciesString())
	_, _ = s.AppClientset.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Update(context.Background(), &existingProj, metav1.UpdateOptions{})
	time.Sleep(100 * time.Millisecond) // this lets the informer get synced
	assert.False(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
	assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	assert.False(t, s.enf.Enforce(claims, "clusters", "get", "test"))
}

func TestRevokedToken(t *testing.T) {
	projectName := "testProj"
	roleName := "testRole"
	subFormat := "proj:%s:%s"
	policyTemplate := "p, %s, applications, get, %s/%s, %s"
	defaultObject := "*"
	defaultEffect := "allow"
	defaultTestObject := fmt.Sprintf("%s/%s", projectName, "test")
	defaultIssuedAt := int64(1)
	defaultSub := fmt.Sprintf(subFormat, projectName, roleName)
	defaultPolicy := fmt.Sprintf(policyTemplate, defaultSub, projectName, defaultObject, defaultEffect)
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret())
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	jwtTokenByRole := make(map[string]v1alpha1.JWTTokens)
	jwtTokenByRole[roleName] = v1alpha1.JWTTokens{Items: []v1alpha1.JWTToken{{IssuedAt: defaultIssuedAt}}}

	existingProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      projectName,
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{
				{
					Name:     roleName,
					Policies: []string{defaultPolicy},
					JWTTokens: []v1alpha1.JWTToken{
						{
							IssuedAt: defaultIssuedAt,
						},
					},
				},
			},
		},
		Status: v1alpha1.AppProjectStatus{
			JWTTokensByRole: jwtTokenByRole,
		},
	}

	s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj), RepoClientset: mockRepoClient}, ApplicationSetOpts{})
	cancel := test.StartInformer(s.projInformer)
	defer cancel()
	claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
	assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
	assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
}

func TestCertsAreNotGeneratedInInsecureMode(t *testing.T) {
	s, closer := fakeServer(t)
	defer closer()
	assert.True(t, s.Insecure)
	assert.Nil(t, s.settings.Certificate)
}

func TestAuthenticate(t *testing.T) {
	type testData struct {
		test             string
		user             string
		errorMsg         string
		anonymousEnabled bool
	}
	tests := []testData{
		{
			test:             "TestNoSessionAnonymousDisabled",
			errorMsg:         "no session information",
			anonymousEnabled: false,
		},
		{
			test:             "TestSessionPresent",
			user:             "admin:login",
			anonymousEnabled: false,
		},
		{
			test:             "TestSessionNotPresentAnonymousEnabled",
			anonymousEnabled: true,
		},
	}

	for _, testData := range tests {
		t.Run(testData.test, func(t *testing.T) {
			cm := test.NewFakeConfigMap()
			if testData.anonymousEnabled {
				cm.Data["users.anonymous.enabled"] = "true"
			}
			secret := test.NewFakeSecret()
			kubeclientset := fake.NewSimpleClientset(cm, secret)
			appClientSet := apps.NewSimpleClientset()
			mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
			argoCDOpts := ArgoCDServerOpts{
				Namespace:     test.FakeArgoCDNamespace,
				KubeClientset: kubeclientset,
				AppClientset:  appClientSet,
				RepoClientset: mockRepoClient,
			}
			argocd := NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})
			ctx := context.Background()
			if testData.user != "" {
				token, err := argocd.sessionMgr.Create(testData.user, 0, "abc")
				require.NoError(t, err)
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(apiclient.MetaDataTokenKey, token))
			}

			_, err := argocd.Authenticate(ctx)
			if testData.errorMsg != "" {
				assert.Errorf(t, err, testData.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func dexMockHandler(t *testing.T, url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/api/dex/.well-known/openid-configuration":
			_, err := io.WriteString(w, fmt.Sprintf(`
{
  "issuer": "%[1]s/api/dex",
  "authorization_endpoint": "%[1]s/api/dex/auth",
  "token_endpoint": "%[1]s/api/dex/token",
  "jwks_uri": "%[1]s/api/dex/keys",
  "userinfo_endpoint": "%[1]s/api/dex/userinfo",
  "device_authorization_endpoint": "%[1]s/api/dex/device/code",
  "grant_types_supported": [
    "authorization_code",
    "refresh_token",
    "urn:ietf:params:oauth:grant-type:device_code"
  ],
  "response_types_supported": [
    "code"
  ],
  "subject_types_supported": [
    "public"
  ],
  "id_token_signing_alg_values_supported": [
    "RS256", "HS256"
  ],
  "code_challenge_methods_supported": [
    "S256",
    "plain"
  ],
  "scopes_supported": [
    "openid",
    "email",
    "groups",
    "profile",
    "offline_access"
  ],
  "token_endpoint_auth_methods_supported": [
    "client_secret_basic",
    "client_secret_post"
  ],
  "claims_supported": [
    "iss",
    "sub",
    "aud",
    "iat",
    "exp",
    "email",
    "email_verified",
    "locale",
    "name",
    "preferred_username",
    "at_hash"
  ]
}`, url))
			if err != nil {
				t.Fail()
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func getTestServer(t *testing.T, anonymousEnabled bool, withFakeSSO bool, useDexForSSO bool, additionalOIDCConfig settings_util.OIDCConfig) (argocd *ArgoCDServer, oidcURL string) {
	cm := test.NewFakeConfigMap()
	if anonymousEnabled {
		cm.Data["users.anonymous.enabled"] = "true"
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start with a placeholder. We need the server URL before setting up the real handler.
	}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dexMockHandler(t, ts.URL)(w, r)
	})
	oidcServer := ts
	if !useDexForSSO {
		oidcServer = testutil.GetOIDCTestServer(t)
	}
	if withFakeSSO {
		cm.Data["url"] = ts.URL
		if useDexForSSO {
			cm.Data["dex.config"] = `
connectors:
  # OIDC
  - type: OIDC
    id: oidc
    name: OIDC
    config:
    issuer: https://auth.example.gom
    clientID: test-client
    clientSecret: $dex.oidc.clientSecret`
		} else {
			// override required oidc config fields but keep other configs as passed in
			additionalOIDCConfig.Name = "Okta"
			additionalOIDCConfig.Issuer = oidcServer.URL
			additionalOIDCConfig.ClientID = "argo-cd"
			additionalOIDCConfig.ClientSecret = "$oidc.okta.clientSecret"
			oidcConfigString, err := yaml.Marshal(additionalOIDCConfig)
			require.NoError(t, err)
			cm.Data["oidc.config"] = string(oidcConfigString)
			// Avoid bothering with certs for local tests.
			cm.Data["oidc.tls.insecure.skip.verify"] = "true"
		}
	}
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}
	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
		RepoClientset: mockRepoClient,
	}
	if withFakeSSO && useDexForSSO {
		argoCDOpts.DexServerAddr = ts.URL
	}
	argocd = NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})
	var err error
	argocd.ssoClientApp, err = oidc.NewClientApp(argocd.settings, argocd.DexServerAddr, argocd.DexTLSConfig, argocd.BaseHRef, cache.NewInMemoryCache(24*time.Hour))
	require.NoError(t, err)
	return argocd, oidcServer.URL
}

func TestGetClaims(t *testing.T) {
	defaultExpiry := jwt.NewNumericDate(time.Now().Add(time.Hour * 24))
	defaultExpiryUnix := float64(defaultExpiry.Unix())

	type testData struct {
		test                  string
		claims                jwt.MapClaims
		expectedErrorContains string
		expectedClaims        jwt.MapClaims
		expectNewToken        bool
		additionalOIDCConfig  settings_util.OIDCConfig
	}
	tests := []testData{
		{
			test: "GetClaims",
			claims: jwt.MapClaims{
				"aud": "argo-cd",
				"exp": defaultExpiry,
				"sub": "randomUser",
			},
			expectedErrorContains: "",
			expectedClaims: jwt.MapClaims{
				"aud": "argo-cd",
				"exp": defaultExpiryUnix,
				"sub": "randomUser",
			},
			expectNewToken:       false,
			additionalOIDCConfig: settings_util.OIDCConfig{},
		},
		{
			// note: a passing test with user info groups can never be achieved since the user never logged in properly
			// therefore the oidcClient's cache contains no accessToken for the user info endpoint
			// and since the oidcClient cache is unexported (for good reasons) we can't mock this behaviour
			test: "GetClaimsWithUserInfoGroupsEnabled",
			claims: jwt.MapClaims{
				"aud": common.ArgoCDClientAppID,
				"exp": defaultExpiry,
				"sub": "randomUser",
			},
			expectedErrorContains: "invalid session",
			expectedClaims: jwt.MapClaims{
				"aud": common.ArgoCDClientAppID,
				"exp": defaultExpiryUnix,
				"sub": "randomUser",
			},
			expectNewToken: false,
			additionalOIDCConfig: settings_util.OIDCConfig{
				EnableUserInfoGroups:    true,
				UserInfoPath:            "/userinfo",
				UserInfoCacheExpiration: "5m",
			},
		},
	}

	for _, testData := range tests {
		testDataCopy := testData

		t.Run(testDataCopy.test, func(t *testing.T) {
			t.Parallel()

			// Must be declared here to avoid race.
			ctx := context.Background() //nolint:ineffassign,staticcheck

			argocd, oidcURL := getTestServer(t, false, true, false, testDataCopy.additionalOIDCConfig)

			// create new JWT and store it on the context to simulate an incoming request
			testDataCopy.claims["iss"] = oidcURL
			testDataCopy.expectedClaims["iss"] = oidcURL
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, testDataCopy.claims)
			key, err := jwt.ParseRSAPrivateKeyFromPEM(testutil.PrivateKey)
			require.NoError(t, err)
			tokenString, err := token.SignedString(key)
			require.NoError(t, err)
			ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(apiclient.MetaDataTokenKey, tokenString))

			gotClaims, newToken, err := argocd.getClaims(ctx)

			// Note: testutil.oidcMockHandler currently doesn't implement reissuing expired tokens
			// so newToken will always be empty
			if testDataCopy.expectNewToken {
				assert.NotEmpty(t, newToken)
			}
			if testDataCopy.expectedClaims == nil {
				assert.Nil(t, gotClaims)
			} else {
				assert.Equal(t, testDataCopy.expectedClaims, gotClaims)
			}
			if testDataCopy.expectedErrorContains != "" {
				assert.ErrorContains(t, err, testDataCopy.expectedErrorContains, "getClaims should have thrown an error and return an error")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticate_3rd_party_JWTs(t *testing.T) {
	// Marshaling single strings to strings is typical, so we test for this relatively common behavior.
	jwt.MarshalSingleStringAsArray = false

	type testData struct {
		test                  string
		anonymousEnabled      bool
		claims                jwt.RegisteredClaims
		expectedErrorContains string
		expectedClaims        interface{}
		useDex                bool
	}
	tests := []testData{
		// Dex
		{
			test:                  "anonymous disabled, no audience",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
		{
			test:                  "anonymous enabled, no audience",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{},
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "anonymous disabled, unexpired token, admin claim",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
		{
			test:                  "anonymous enabled, unexpired token, admin claim",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "anonymous disabled, expired token, admin claim",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now())},
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        jwt.RegisteredClaims{Issuer: "sso"},
		},
		{
			test:                  "anonymous enabled, expired token, admin claim",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now())},
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "anonymous disabled, unexpired token, admin claim, incorrect audience",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"incorrect-audience"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
		// External OIDC (not bundled Dex)
		{
			test:                  "external OIDC: anonymous disabled, no audience",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			useDex:                true,
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
		{
			test:                  "external OIDC: anonymous enabled, no audience",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{},
			useDex:                true,
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "external OIDC: anonymous disabled, unexpired token, admin claim",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			useDex:                true,
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
		{
			test:                  "external OIDC: anonymous enabled, unexpired token, admin claim",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			useDex:                true,
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "external OIDC: anonymous disabled, expired token, admin claim",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now())},
			useDex:                true,
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        jwt.RegisteredClaims{Issuer: "sso"},
		},
		{
			test:                  "external OIDC: anonymous enabled, expired token, admin claim",
			anonymousEnabled:      true,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{common.ArgoCDClientAppID}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now())},
			useDex:                true,
			expectedErrorContains: "",
			expectedClaims:        "",
		},
		{
			test:                  "external OIDC: anonymous disabled, unexpired token, admin claim, incorrect audience",
			anonymousEnabled:      false,
			claims:                jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"incorrect-audience"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))},
			useDex:                true,
			expectedErrorContains: common.TokenVerificationError,
			expectedClaims:        nil,
		},
	}

	for _, testData := range tests {
		testDataCopy := testData

		t.Run(testDataCopy.test, func(t *testing.T) {
			t.Parallel()

			// Must be declared here to avoid race.
			ctx := context.Background() //nolint:ineffassign,staticcheck

			argocd, oidcURL := getTestServer(t, testDataCopy.anonymousEnabled, true, testDataCopy.useDex, settings_util.OIDCConfig{})

			if testDataCopy.useDex {
				testDataCopy.claims.Issuer = fmt.Sprintf("%s/api/dex", oidcURL)
			} else {
				testDataCopy.claims.Issuer = oidcURL
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, testDataCopy.claims)
			tokenString, err := token.SignedString([]byte("key"))
			require.NoError(t, err)
			ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(apiclient.MetaDataTokenKey, tokenString))

			ctx, err = argocd.Authenticate(ctx)
			claims := ctx.Value("claims")
			if testDataCopy.expectedClaims == nil {
				assert.Nil(t, claims)
			} else {
				assert.Equal(t, testDataCopy.expectedClaims, claims)
			}
			if testDataCopy.expectedErrorContains != "" {
				assert.ErrorContains(t, err, testDataCopy.expectedErrorContains, "Authenticate should have thrown an error and blocked the request")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticate_no_request_metadata(t *testing.T) {
	type testData struct {
		test                  string
		anonymousEnabled      bool
		expectedErrorContains string
		expectedClaims        interface{}
	}
	tests := []testData{
		{
			test:                  "anonymous disabled",
			anonymousEnabled:      false,
			expectedErrorContains: "no session information",
			expectedClaims:        nil,
		},
		{
			test:                  "anonymous enabled",
			anonymousEnabled:      true,
			expectedErrorContains: "",
			expectedClaims:        "",
		},
	}

	for _, testData := range tests {
		testDataCopy := testData

		t.Run(testDataCopy.test, func(t *testing.T) {
			t.Parallel()

			argocd, _ := getTestServer(t, testDataCopy.anonymousEnabled, true, true, settings_util.OIDCConfig{})
			ctx := context.Background()

			ctx, err := argocd.Authenticate(ctx)
			claims := ctx.Value("claims")
			assert.Equal(t, testDataCopy.expectedClaims, claims)
			if testDataCopy.expectedErrorContains != "" {
				assert.ErrorContains(t, err, testDataCopy.expectedErrorContains, "Authenticate should have thrown an error and blocked the request")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticate_no_SSO(t *testing.T) {
	type testData struct {
		test                 string
		anonymousEnabled     bool
		expectedErrorMessage string
		expectedClaims       interface{}
	}
	tests := []testData{
		{
			test:                 "anonymous disabled",
			anonymousEnabled:     false,
			expectedErrorMessage: "SSO is not configured",
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled",
			anonymousEnabled:     true,
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
	}

	for _, testData := range tests {
		testDataCopy := testData

		t.Run(testDataCopy.test, func(t *testing.T) {
			t.Parallel()

			// Must be declared here to avoid race.
			ctx := context.Background() //nolint:ineffassign,staticcheck

			argocd, dexURL := getTestServer(t, testDataCopy.anonymousEnabled, false, true, settings_util.OIDCConfig{})
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Issuer: fmt.Sprintf("%s/api/dex", dexURL)})
			tokenString, err := token.SignedString([]byte("key"))
			require.NoError(t, err)
			ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(apiclient.MetaDataTokenKey, tokenString))

			ctx, err = argocd.Authenticate(ctx)
			claims := ctx.Value("claims")
			assert.Equal(t, testDataCopy.expectedClaims, claims)
			if testDataCopy.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, testDataCopy.expectedErrorMessage, "Authenticate should have thrown an error and blocked the request")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthenticate_bad_request_metadata(t *testing.T) {
	type testData struct {
		test                 string
		anonymousEnabled     bool
		metadata             metadata.MD
		expectedErrorMessage string
		expectedClaims       interface{}
	}
	tests := []testData{
		{
			test:                 "anonymous disabled, empty metadata",
			anonymousEnabled:     false,
			metadata:             metadata.MD{},
			expectedErrorMessage: "no session information",
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled, empty metadata",
			anonymousEnabled:     true,
			metadata:             metadata.MD{},
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
		{
			test:                 "anonymous disabled, empty tokens",
			anonymousEnabled:     false,
			metadata:             metadata.MD{apiclient.MetaDataTokenKey: []string{}},
			expectedErrorMessage: "no session information",
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled, empty tokens",
			anonymousEnabled:     true,
			metadata:             metadata.MD{apiclient.MetaDataTokenKey: []string{}},
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
		{
			test:                 "anonymous disabled, bad tokens",
			anonymousEnabled:     false,
			metadata:             metadata.Pairs(apiclient.MetaDataTokenKey, "bad"),
			expectedErrorMessage: "token contains an invalid number of segments",
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled, bad tokens",
			anonymousEnabled:     true,
			metadata:             metadata.Pairs(apiclient.MetaDataTokenKey, "bad"),
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
		{
			test:                 "anonymous disabled, bad auth header",
			anonymousEnabled:     false,
			metadata:             metadata.MD{"authorization": []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiJ9.TGGTTHuuGpEU8WgobXxkrBtW3NiR3dgw5LR-1DEW3BQ"}},
			expectedErrorMessage: common.TokenVerificationError,
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled, bad auth header",
			anonymousEnabled:     true,
			metadata:             metadata.MD{"authorization": []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiJ9.TGGTTHuuGpEU8WgobXxkrBtW3NiR3dgw5LR-1DEW3BQ"}},
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
		{
			test:                 "anonymous disabled, bad auth cookie",
			anonymousEnabled:     false,
			metadata:             metadata.MD{"grpcgateway-cookie": []string{"argocd.token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiJ9.TGGTTHuuGpEU8WgobXxkrBtW3NiR3dgw5LR-1DEW3BQ"}},
			expectedErrorMessage: common.TokenVerificationError,
			expectedClaims:       nil,
		},
		{
			test:                 "anonymous enabled, bad auth cookie",
			anonymousEnabled:     true,
			metadata:             metadata.MD{"grpcgateway-cookie": []string{"argocd.token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZG1pbiJ9.TGGTTHuuGpEU8WgobXxkrBtW3NiR3dgw5LR-1DEW3BQ"}},
			expectedErrorMessage: "",
			expectedClaims:       "",
		},
	}

	for _, testData := range tests {
		testDataCopy := testData

		t.Run(testDataCopy.test, func(t *testing.T) {
			t.Parallel()

			// Must be declared here to avoid race.
			ctx := context.Background() //nolint:ineffassign,staticcheck

			argocd, _ := getTestServer(t, testDataCopy.anonymousEnabled, true, true, settings_util.OIDCConfig{})
			ctx = metadata.NewIncomingContext(context.Background(), testDataCopy.metadata)

			ctx, err := argocd.Authenticate(ctx)
			claims := ctx.Value("claims")
			assert.Equal(t, testDataCopy.expectedClaims, claims)
			if testDataCopy.expectedErrorMessage != "" {
				assert.ErrorContains(t, err, testDataCopy.expectedErrorMessage, "Authenticate should have thrown an error and blocked the request")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_getToken(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	t.Run("Empty", func(t *testing.T) {
		assert.Empty(t, getToken(metadata.New(map[string]string{})))
	})
	t.Run("Token", func(t *testing.T) {
		assert.Equal(t, token, getToken(metadata.New(map[string]string{"token": token})))
	})
	t.Run("Authorisation", func(t *testing.T) {
		assert.Empty(t, getToken(metadata.New(map[string]string{"authorization": "Bearer invalid"})))
		assert.Equal(t, token, getToken(metadata.New(map[string]string{"authorization": "Bearer " + token})))
	})
	t.Run("Cookie", func(t *testing.T) {
		assert.Empty(t, getToken(metadata.New(map[string]string{"grpcgateway-cookie": "argocd.token=invalid"})))
		assert.Equal(t, token, getToken(metadata.New(map[string]string{"grpcgateway-cookie": "argocd.token=" + token})))
	})
}

func TestTranslateGrpcCookieHeader(t *testing.T) {
	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret()),
		AppClientset:  apps.NewSimpleClientset(),
		RepoClientset: &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}},
	}
	argocd := NewServer(context.Background(), argoCDOpts, ApplicationSetOpts{})

	t.Run("TokenIsNotEmpty", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		err := argocd.translateGrpcCookieHeader(context.Background(), recorder, &session.SessionResponse{
			Token: "xyz",
		})
		require.NoError(t, err)
		assert.Equal(t, "argocd.token=xyz; path=/; SameSite=lax; httpOnly; Secure", recorder.Result().Header.Get("Set-Cookie"))
		assert.Len(t, recorder.Result().Cookies(), 1)
	})

	t.Run("TokenIsLongerThan4093", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		err := argocd.translateGrpcCookieHeader(context.Background(), recorder, &session.SessionResponse{
			Token: "abc.xyz." + strings.Repeat("x", 4093),
		})
		require.NoError(t, err)
		assert.Regexp(t, "argocd.token=.*; path=/; SameSite=lax; httpOnly; Secure", recorder.Result().Header.Get("Set-Cookie"))
		assert.Len(t, recorder.Result().Cookies(), 2)
	})

	t.Run("TokenIsEmpty", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		err := argocd.translateGrpcCookieHeader(context.Background(), recorder, &session.SessionResponse{
			Token: "",
		})
		require.NoError(t, err)
		assert.Equal(t, "", recorder.Result().Header.Get("Set-Cookie"))
	})
}

func TestInitializeDefaultProject_ProjectDoesNotExist(t *testing.T) {
	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret()),
		AppClientset:  apps.NewSimpleClientset(),
		RepoClientset: &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}},
	}

	err := initializeDefaultProject(argoCDOpts)
	require.NoError(t, err)

	proj, err := argoCDOpts.AppClientset.ArgoprojV1alpha1().
		AppProjects(test.FakeArgoCDNamespace).Get(context.Background(), v1alpha1.DefaultAppProjectName, metav1.GetOptions{})

	require.NoError(t, err)

	assert.Equal(t, v1alpha1.AppProjectSpec{
		SourceRepos:              []string{"*"},
		Destinations:             []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
	}, proj.Spec)
}

func TestInitializeDefaultProject_ProjectAlreadyInitialized(t *testing.T) {
	existingDefaultProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.DefaultAppProjectName,
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:  []string{"some repo"},
			Destinations: []v1alpha1.ApplicationDestination{{Server: "some cluster", Namespace: "*"}},
		},
	}

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret()),
		AppClientset:  apps.NewSimpleClientset(&existingDefaultProject),
		RepoClientset: &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}},
	}

	err := initializeDefaultProject(argoCDOpts)
	require.NoError(t, err)

	proj, err := argoCDOpts.AppClientset.ArgoprojV1alpha1().
		AppProjects(test.FakeArgoCDNamespace).Get(context.Background(), v1alpha1.DefaultAppProjectName, metav1.GetOptions{})

	require.NoError(t, err)

	assert.Equal(t, proj.Spec, existingDefaultProject.Spec)
}

func TestOIDCConfigChangeDetection_SecretsChanged(t *testing.T) {
	// Given
	rawOIDCConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})
	require.NoError(t, err, "no error expected when marshalling OIDC config")

	originalSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "sharedargooauthsecret"}

	argoSettings := settings_util.ArgoCDSettings{OIDCConfigRAW: string(rawOIDCConfig), Secrets: originalSecrets}

	originalOIDCConfig := argoSettings.OIDCConfig()

	assert.Equal(t, originalOIDCConfig.ClientID, originalSecrets["k8ssecret:clientid"], "expected ClientID be replaced by secret value")
	assert.Equal(t, originalOIDCConfig.ClientSecret, originalSecrets["k8ssecret:clientsecret"], "expected ClientSecret be replaced by secret value")

	// When
	newSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "a!Better!Secret"}
	argoSettings.Secrets = newSecrets
	result := checkOIDCConfigChange(originalOIDCConfig, &argoSettings)

	// Then
	assert.True(t, result, "secrets have changed, expect interpolated OIDCConfig to change")
}

func TestOIDCConfigChangeDetection_ConfigChanged(t *testing.T) {
	// Given
	rawOIDCConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		Name:         "argocd",
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})

	require.NoError(t, err, "no error expected when marshalling OIDC config")

	originalSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "sharedargooauthsecret"}

	argoSettings := settings_util.ArgoCDSettings{OIDCConfigRAW: string(rawOIDCConfig), Secrets: originalSecrets}

	originalOIDCConfig := argoSettings.OIDCConfig()

	assert.Equal(t, originalOIDCConfig.ClientID, originalSecrets["k8ssecret:clientid"], "expected ClientID be replaced by secret value")
	assert.Equal(t, originalOIDCConfig.ClientSecret, originalSecrets["k8ssecret:clientsecret"], "expected ClientSecret be replaced by secret value")

	// When
	newRawOICDConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		Name:         "cat",
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})

	require.NoError(t, err, "no error expected when marshalling OIDC config")
	argoSettings.OIDCConfigRAW = string(newRawOICDConfig)
	result := checkOIDCConfigChange(originalOIDCConfig, &argoSettings)

	// Then
	assert.True(t, result, "no error expected since OICD config created")
}

func TestOIDCConfigChangeDetection_ConfigCreated(t *testing.T) {
	// Given
	argoSettings := settings_util.ArgoCDSettings{OIDCConfigRAW: ""}
	originalOIDCConfig := argoSettings.OIDCConfig()

	// When
	newRawOICDConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		Name:         "cat",
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})
	require.NoError(t, err, "no error expected when marshalling OIDC config")
	newSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "sharedargooauthsecret"}
	argoSettings.OIDCConfigRAW = string(newRawOICDConfig)
	argoSettings.Secrets = newSecrets
	result := checkOIDCConfigChange(originalOIDCConfig, &argoSettings)

	// Then
	assert.True(t, result, "no error expected since new OICD config created")
}

func TestOIDCConfigChangeDetection_ConfigDeleted(t *testing.T) {
	// Given
	rawOIDCConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})
	require.NoError(t, err, "no error expected when marshalling OIDC config")

	originalSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "sharedargooauthsecret"}

	argoSettings := settings_util.ArgoCDSettings{OIDCConfigRAW: string(rawOIDCConfig), Secrets: originalSecrets}

	originalOIDCConfig := argoSettings.OIDCConfig()

	assert.Equal(t, originalOIDCConfig.ClientID, originalSecrets["k8ssecret:clientid"], "expected ClientID be replaced by secret value")
	assert.Equal(t, originalOIDCConfig.ClientSecret, originalSecrets["k8ssecret:clientsecret"], "expected ClientSecret be replaced by secret value")

	// When
	argoSettings.OIDCConfigRAW = ""
	argoSettings.Secrets = make(map[string]string)
	result := checkOIDCConfigChange(originalOIDCConfig, &argoSettings)

	// Then
	assert.True(t, result, "no error expected since OICD config deleted")
}

func TestOIDCConfigChangeDetection_NoChange(t *testing.T) {
	// Given
	rawOIDCConfig, err := yaml.Marshal(&settings_util.OIDCConfig{
		ClientID:     "$k8ssecret:clientid",
		ClientSecret: "$k8ssecret:clientsecret",
	})
	require.NoError(t, err, "no error expected when marshalling OIDC config")

	originalSecrets := map[string]string{"k8ssecret:clientid": "argocd", "k8ssecret:clientsecret": "sharedargooauthsecret"}

	argoSettings := settings_util.ArgoCDSettings{OIDCConfigRAW: string(rawOIDCConfig), Secrets: originalSecrets}

	originalOIDCConfig := argoSettings.OIDCConfig()

	assert.Equal(t, originalOIDCConfig.ClientID, originalSecrets["k8ssecret:clientid"], "expected ClientID be replaced by secret value")
	assert.Equal(t, originalOIDCConfig.ClientSecret, originalSecrets["k8ssecret:clientsecret"], "expected ClientSecret be replaced by secret value")

	// When
	result := checkOIDCConfigChange(originalOIDCConfig, &argoSettings)

	// Then
	assert.False(t, result, "no error since no config change")
}

func TestIsMainJsBundle(t *testing.T) {
	testCases := []struct {
		name           string
		url            string
		isMainJsBundle bool
	}{
		{
			name:           "localhost with valid main bundle",
			url:            "https://localhost:8080/main.e4188e5adc97bbfc00c3.js",
			isMainJsBundle: true,
		},
		{
			name:           "localhost and deep path with valid main bundle",
			url:            "https://localhost:8080/some/argo-cd-instance/main.e4188e5adc97bbfc00c3.js",
			isMainJsBundle: true,
		},
		{
			name:           "font file",
			url:            "https://localhost:8080/assets/fonts/google-fonts/Heebo-Bols.woff2",
			isMainJsBundle: false,
		},
		{
			name:           "no dot after main",
			url:            "https://localhost:8080/main/e4188e5adc97bbfc00c3.js",
			isMainJsBundle: false,
		},
		{
			name:           "wrong extension character",
			url:            "https://localhost:8080/main.e4188e5adc97bbfc00c3/js",
			isMainJsBundle: false,
		},
		{
			name:           "wrong hash length",
			url:            "https://localhost:8080/main.e4188e5adc97bbfc00c3abcdefg.js",
			isMainJsBundle: false,
		},
	}
	for _, testCase := range testCases {
		testCaseCopy := testCase
		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()
			testUrl, _ := url.Parse(testCaseCopy.url)
			isMainJsBundle := isMainJsBundle(testUrl)
			assert.Equal(t, testCaseCopy.isMainJsBundle, isMainJsBundle)
		})
	}
}

func TestCacheControlHeaders(t *testing.T) {
	testCases := []struct {
		name                        string
		filename                    string
		createFile                  bool
		expectedStatus              int
		expectedCacheControlHeaders []string
	}{
		{
			name:                        "file exists",
			filename:                    "exists.html",
			createFile:                  true,
			expectedStatus:              200,
			expectedCacheControlHeaders: nil,
		},
		{
			name:                        "file does not exist",
			filename:                    "missing.html",
			createFile:                  false,
			expectedStatus:              404,
			expectedCacheControlHeaders: nil,
		},
		{
			name:                        "main js bundle exists",
			filename:                    "main.e4188e5adc97bbfc00c3.js",
			createFile:                  true,
			expectedStatus:              200,
			expectedCacheControlHeaders: []string{"public, max-age=31536000, immutable"},
		},
		{
			name:                        "main js bundle does not exists",
			filename:                    "main.e4188e5adc97bbfc00c0.js",
			createFile:                  false,
			expectedStatus:              404,
			expectedCacheControlHeaders: []string{"no-cache"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			argocd, closer := fakeServer(t)
			defer closer()

			handler := argocd.newStaticAssetsHandler()

			rr := httptest.NewRecorder()
			req := httptest.NewRequest("", fmt.Sprintf("/%s", testCase.filename), nil)

			fp := filepath.Join(argocd.TmpAssetsDir, testCase.filename)

			if testCase.createFile {
				tmpFile, err := os.Create(fp)
				require.NoError(t, err)
				err = tmpFile.Close()
				require.NoError(t, err)
			}

			handler(rr, req)

			assert.Equal(t, testCase.expectedStatus, rr.Code)

			cacheControl := rr.Result().Header["Cache-Control"]
			assert.Equal(t, testCase.expectedCacheControlHeaders, cacheControl)
		})
	}
}

func TestReplaceBaseHRef(t *testing.T) {
	testCases := []struct {
		name        string
		data        string
		expected    string
		replaceWith string
	}{
		{
			name: "non-root basepath",
			data: `<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>Argo CD</title>
    <base href="/">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel='icon' type='image/png' href='assets/favicon/favicon-32x32.png' sizes='32x32'/>
    <link rel='icon' type='image/png' href='assets/favicon/favicon-16x16.png' sizes='16x16'/>
    <link href="assets/fonts.css" rel="stylesheet">
</head>

<body>
    <noscript>
        <p>
        Your browser does not support JavaScript. Please enable JavaScript to view the site.
        Alternatively, Argo CD can be used with the <a href="https://argoproj.github.io/argo-cd/cli_installation/">Argo CD CLI</a>.
        </p>
    </noscript>
    <div id="app"></div>
</body>

</html>`,
			expected: `<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>Argo CD</title>
    <base href="/path1/path2/path3/">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel='icon' type='image/png' href='assets/favicon/favicon-32x32.png' sizes='32x32'/>
    <link rel='icon' type='image/png' href='assets/favicon/favicon-16x16.png' sizes='16x16'/>
    <link href="assets/fonts.css" rel="stylesheet">
</head>

<body>
    <noscript>
        <p>
        Your browser does not support JavaScript. Please enable JavaScript to view the site.
        Alternatively, Argo CD can be used with the <a href="https://argoproj.github.io/argo-cd/cli_installation/">Argo CD CLI</a>.
        </p>
    </noscript>
    <div id="app"></div>
</body>

</html>`,
			replaceWith: `<base href="/path1/path2/path3/">`,
		},
		{
			name: "root basepath",
			data: `<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>Argo CD</title>
    <base href="/any/path/test/">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel='icon' type='image/png' href='assets/favicon/favicon-32x32.png' sizes='32x32'/>
    <link rel='icon' type='image/png' href='assets/favicon/favicon-16x16.png' sizes='16x16'/>
    <link href="assets/fonts.css" rel="stylesheet">
</head>

<body>
    <noscript>
        <p>
        Your browser does not support JavaScript. Please enable JavaScript to view the site.
        Alternatively, Argo CD can be used with the <a href="https://argoproj.github.io/argo-cd/cli_installation/">Argo CD CLI</a>.
        </p>
    </noscript>
    <div id="app"></div>
</body>

</html>`,
			expected: `<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <title>Argo CD</title>
    <base href="/">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel='icon' type='image/png' href='assets/favicon/favicon-32x32.png' sizes='32x32'/>
    <link rel='icon' type='image/png' href='assets/favicon/favicon-16x16.png' sizes='16x16'/>
    <link href="assets/fonts.css" rel="stylesheet">
</head>

<body>
    <noscript>
        <p>
        Your browser does not support JavaScript. Please enable JavaScript to view the site.
        Alternatively, Argo CD can be used with the <a href="https://argoproj.github.io/argo-cd/cli_installation/">Argo CD CLI</a>.
        </p>
    </noscript>
    <div id="app"></div>
</body>

</html>`,
			replaceWith: `<base href="/">`,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := replaceBaseHRef(testCase.data, testCase.replaceWith)
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func Test_enforceContentTypes(t *testing.T) {
	getBaseHandler := func(t *testing.T, allow bool) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			assert.True(t, allow, "http handler was hit when it should have been blocked by content type enforcement")
			writer.WriteHeader(200)
		})
	}

	t.Parallel()

	t.Run("GET - not providing a content type, should still succeed", func(t *testing.T) {
		handler := enforceContentTypes(getBaseHandler(t, true), []string{"application/json"}).(http.HandlerFunc)
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		resp := w.Result()
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("POST", func(t *testing.T) {
		handler := enforceContentTypes(getBaseHandler(t, true), []string{"application/json"}).(http.HandlerFunc)
		req := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		resp := w.Result()
		assert.Equal(t, 415, resp.StatusCode, "didn't provide a content type, should have gotten an error")

		req = httptest.NewRequest("POST", "/", nil)
		req.Header = map[string][]string{"Content-Type": {"application/json"}}
		w = httptest.NewRecorder()
		handler(w, req)
		resp = w.Result()
		assert.Equal(t, 200, resp.StatusCode, "should have passed, since an allowed content type was provided")

		req = httptest.NewRequest("POST", "/", nil)
		req.Header = map[string][]string{"Content-Type": {"not-allowed"}}
		w = httptest.NewRecorder()
		handler(w, req)
		resp = w.Result()
		assert.Equal(t, 415, resp.StatusCode, "should not have passed, since a disallowed content type was provided")
	})
}
