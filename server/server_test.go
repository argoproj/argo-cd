package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/assets"
	"github.com/argoproj/argo-cd/util/rbac"
)

func fakeServer() *ArgoCDServer {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
		Insecure:      true,
		DisableAuth:   true,
	}
	return NewServer(context.Background(), argoCDOpts)
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

	role := v1alpha1.ProjectRole{Name: roleName, Policies: []string{defaultPolicy}, JWTTokens: []v1alpha1.JWTToken{{IssuedAt: defaultIssuedAt}}}
	existingProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projectName, Namespace: test.FakeArgoCDNamespace},
		Spec: v1alpha1.AppProjectSpec{
			Roles: []v1alpha1.ProjectRole{role},
		},
	}
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)

	t.Run("TestEnforceProjectTokenSuccessful", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		cancel := test.StartInformer(s.projInformer)
		defer cancel()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
		assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
		assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenWithDiffCreateAtFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		diffCreateAt := defaultIssuedAt + 1
		claims := jwt.MapClaims{"sub": defaultSub, "iat": diffCreateAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenIncorrectSubFormatFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		invalidSub := "proj:test"
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultIssuedAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenNoTokenFailure", func(t *testing.T) {
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
		nonExistentToken := "fake-token"
		invalidSub := fmt.Sprintf(subFormat, projectName, nonExistentToken)
		claims := jwt.MapClaims{"sub": invalidSub, "iat": defaultIssuedAt}
		assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	})

	t.Run("TestEnforceProjectTokenNotJWTTokenFailure", func(t *testing.T) {
		proj := existingProj.DeepCopy()
		proj.Spec.Roles[0].JWTTokens = nil
		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj)})
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

		s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(proj)})
		cancel := test.StartInformer(s.projInformer)
		defer cancel()
		claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
		allowedObject := fmt.Sprintf("%s/%s", projectName, "test")
		denyObject := fmt.Sprintf("%s/%s", projectName, denyApp)
		assert.True(t, s.enf.Enforce(claims, "applications", "get", allowedObject))
		assert.False(t, s.enf.Enforce(claims, "applications", "get", denyObject))
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
		jwt.StandardClaims{Subject: "admin"},
	}
	for _, c := range allowed {
		if !assert.True(t, enf.Enforce(c, "applications", "delete", "foo/obj")) {
			log.Errorf("%v: expected true, got false", c)
		}
	}

	disallowed := []jwt.Claims{
		jwt.MapClaims{"groups": []string{"org3:team3"}},
		jwt.StandardClaims{Subject: "nobody"},
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
		ObjectMeta: metav1.ObjectMeta{Name: common.DefaultAppProjectName, Namespace: test.FakeArgoCDNamespace},
		Spec:       v1alpha1.AppProjectSpec{},
	}
	appClientSet := apps.NewSimpleClientset(defaultProj)

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
	}

	argocd := NewServer(context.Background(), argoCDOpts)
	assert.NotNil(t, argocd)

	proj, err := appClientSet.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Get(common.DefaultAppProjectName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, proj)
	assert.Equal(t, proj.Name, common.DefaultAppProjectName)
}

func TestInitializingNotExistingDefaultProject(t *testing.T) {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()

	argoCDOpts := ArgoCDServerOpts{
		Namespace:     test.FakeArgoCDNamespace,
		KubeClientset: kubeclientset,
		AppClientset:  appClientSet,
	}

	argocd := NewServer(context.Background(), argoCDOpts)
	assert.NotNil(t, argocd)

	proj, err := appClientSet.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Get(common.DefaultAppProjectName, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.NotNil(t, proj)
	assert.Equal(t, proj.Name, common.DefaultAppProjectName)
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
	kubeclientset := fake.NewSimpleClientset(test.NewFakeConfigMap(), test.NewFakeSecret())
	s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
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
	_, _ = s.AppClientset.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Update(&existingProj)
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
	}

	s := NewServer(context.Background(), ArgoCDServerOpts{Namespace: test.FakeArgoCDNamespace, KubeClientset: kubeclientset, AppClientset: apps.NewSimpleClientset(&existingProj)})
	cancel := test.StartInformer(s.projInformer)
	defer cancel()
	claims := jwt.MapClaims{"sub": defaultSub, "iat": defaultIssuedAt}
	assert.True(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
	assert.True(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
	// Now revoke the token by deleting the token
	existingProj.Spec.Roles[0].JWTTokens = nil
	_, _ = s.AppClientset.ArgoprojV1alpha1().AppProjects(test.FakeArgoCDNamespace).Update(&existingProj)
	time.Sleep(200 * time.Millisecond) // this lets the informer get synced
	assert.False(t, s.enf.Enforce(claims, "projects", "get", existingProj.ObjectMeta.Name))
	assert.False(t, s.enf.Enforce(claims, "applications", "get", defaultTestObject))
}

func TestCertsAreNotGeneratedInInsecureMode(t *testing.T) {
	s := fakeServer()
	assert.True(t, s.Insecure)
	assert.Nil(t, s.settings.Certificate)
}

func TestUserAgent(t *testing.T) {
	s := fakeServer()
	cancelInformer := test.StartInformer(s.projInformer)
	defer cancelInformer()
	port, err := test.GetFreePort()
	assert.NoError(t, err)
	metricsPort, err := test.GetFreePort()
	assert.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx, port, metricsPort)
	defer func() { time.Sleep(3 * time.Second) }()

	err = test.WaitForPortListen(fmt.Sprintf("127.0.0.1:%d", port), 10*time.Second)
	assert.NoError(t, err)

	type testData struct {
		userAgent string
		errorMsg  string
	}
	currentVersionBytes, err := ioutil.ReadFile("../VERSION")
	assert.NoError(t, err)
	currentVersion := strings.TrimSpace(string(currentVersionBytes))
	var tests = []testData{
		{
			// Reject out-of-date user-agent
			userAgent: fmt.Sprintf("%s/0.10.0", common.ArgoCDUserAgentName),
			errorMsg:  "unsatisfied client version constraint",
		},
		{
			// Accept up-to-date user-agent
			userAgent: fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, currentVersion),
		},
		{
			// Accept up-to-date pre-release user-agent
			userAgent: fmt.Sprintf("%s/%s-rc1", common.ArgoCDUserAgentName, currentVersion),
		},
		{
			// Reject legacy client
			// NOTE: after we update the grpc-go client past 1.15.0, this test will break and should be deleted
			userAgent: " ", // need a space here since the apiclient will set the default user-agent if empty
			errorMsg:  "unsatisfied client version constraint",
		},
		{
			// Permit custom clients
			userAgent: "foo/1.2.3",
		},
	}

	for _, test := range tests {
		opts := apiclient.ClientOptions{
			ServerAddr: fmt.Sprintf("localhost:%d", port),
			PlainText:  true,
			UserAgent:  test.userAgent,
		}
		clnt, err := apiclient.NewClient(&opts)
		assert.NoError(t, err)
		conn, appClnt := clnt.NewApplicationClientOrDie()
		_, err = appClnt.List(ctx, &applicationpkg.ApplicationQuery{})
		if test.errorMsg != "" {
			assert.Error(t, err)
			assert.Regexp(t, test.errorMsg, err.Error())
		} else {
			assert.NoError(t, err)
		}
		_ = conn.Close()
	}
}

func TestAuthenticate(t *testing.T) {
	type testData struct {
		test             string
		user             string
		errorMsg         string
		anonymousEnabled bool
	}
	var tests = []testData{
		{
			test:             "TestNoSessionAnonymousDisabled",
			errorMsg:         "no session information",
			anonymousEnabled: false,
		},
		{
			test:             "TestSessionPresent",
			user:             "admin",
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
			argoCDOpts := ArgoCDServerOpts{
				Namespace:     test.FakeArgoCDNamespace,
				KubeClientset: kubeclientset,
				AppClientset:  appClientSet,
			}
			argocd := NewServer(context.Background(), argoCDOpts)
			ctx := context.Background()
			if testData.user != "" {
				token, err := argocd.sessionMgr.Create("admin", 0)
				assert.NoError(t, err)
				ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(apiclient.MetaDataTokenKey, token))
			}

			_, err := argocd.authenticate(ctx)
			if testData.errorMsg != "" {
				assert.Errorf(t, err, testData.errorMsg)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
