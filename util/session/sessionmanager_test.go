package session

import (
	"context"
	"encoding/pem"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/password"
	"github.com/argoproj/argo-cd/v2/util/settings"
	utiltest "github.com/argoproj/argo-cd/v2/util/test"
)

func getProjLister(objects ...runtime.Object) v1alpha1.AppProjectNamespaceLister {
	return test.NewFakeProjListerFromInterface(apps.NewSimpleClientset(objects...).ArgoprojV1alpha1().AppProjects("argocd"))
}

func getKubeClient(pass string, enabled bool, capabilities ...settings.AccountCapability) *fake.Clientset {
	const defaultSecretKey = "Hello, world!"

	bcrypt, err := password.HashPassword(pass)
	errors.CheckError(err)
	if len(capabilities) == 0 {
		capabilities = []settings.AccountCapability{settings.AccountCapabilityLogin, settings.AccountCapabilityApiKey}
	}
	var capabilitiesStr []string
	for i := range capabilities {
		capabilitiesStr = append(capabilitiesStr, string(capabilities[i]))
	}

	return fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"admin":         strings.Join(capabilitiesStr, ","),
			"admin.enabled": strconv.FormatBool(enabled),
		},
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: "argocd",
		},
		Data: map[string][]byte{
			"admin.password":   []byte(bcrypt),
			"server.secretkey": []byte(defaultSecretKey),
		},
	})
}

func newSessionManager(settingsMgr *settings.SettingsManager, projectLister v1alpha1.AppProjectNamespaceLister, storage UserStateStorage) *SessionManager {
	mgr := NewSessionManager(settingsMgr, projectLister, "", nil, storage)
	mgr.verificationDelayNoiseEnabled = false
	return mgr
}

func TestSessionManager_AdminToken(t *testing.T) {
	redisClient, closer := test.NewInMemoryRedis()
	defer closer()

	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(redisClient))

	token, err := mgr.Create("admin:login", 0, "123")
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	claims, newToken, err := mgr.Parse(token)
	require.NoError(t, err)
	assert.Empty(t, newToken)

	mapClaims := *(claims.(*jwt.MapClaims))
	subject := mapClaims["sub"].(string)
	if subject != "admin" {
		t.Errorf("Token claim subject \"%s\" does not match expected subject \"%s\".", subject, "admin")
	}
}

func TestSessionManager_AdminToken_ExpiringSoon(t *testing.T) {
	redisClient, closer := test.NewInMemoryRedis()
	defer closer()

	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(redisClient))

	token, err := mgr.Create("admin:login", int64(autoRegenerateTokenDuration.Seconds()-1), "123")
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	// verify new token is generated is login token is expiring soon
	_, newToken, err := mgr.Parse(token)
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)

	// verify that new token is valid and for the same user
	claims, _, err := mgr.Parse(newToken)
	require.NoError(t, err)
	mapClaims := *(claims.(*jwt.MapClaims))
	subject := mapClaims["sub"].(string)
	assert.Equal(t, "admin", subject)
}

func TestSessionManager_AdminToken_Revoked(t *testing.T) {
	redisClient, closer := test.NewInMemoryRedis()
	defer closer()

	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true), "argocd")
	storage := NewUserStateStorage(redisClient)

	mgr := newSessionManager(settingsMgr, getProjLister(), storage)

	token, err := mgr.Create("admin:login", 0, "123")
	require.NoError(t, err)

	err = storage.RevokeToken(context.Background(), "123", time.Hour)
	require.NoError(t, err)

	_, _, err = mgr.Parse(token)
	require.Error(t, err)
	assert.Equal(t, "token is revoked, please re-login", err.Error())
}

func TestSessionManager_AdminToken_Deactivated(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", false), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	token, err := mgr.Create("admin:login", 0, "abc")
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	_, _, err = mgr.Parse(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account admin is disabled")
}

func TestSessionManager_AdminToken_LoginCapabilityDisabled(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true, settings.AccountCapabilityLogin), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	token, err := mgr.Create("admin", 0, "abc")
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	_, _, err = mgr.Parse(token)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account admin does not have 'apiKey' capability")
}

func TestSessionManager_ProjectToken(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true), "argocd")

	t.Run("Valid Token", func(t *testing.T) {
		proj := appv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: appv1.AppProjectSpec{Roles: []appv1.ProjectRole{{Name: "test"}}},
			Status: appv1.AppProjectStatus{JWTTokensByRole: map[string]appv1.JWTTokens{
				"test": {
					Items: []appv1.JWTToken{{ID: "abc", IssuedAt: time.Now().Unix(), ExpiresAt: 0}},
				},
			}},
		}
		mgr := newSessionManager(settingsMgr, getProjLister(&proj), NewUserStateStorage(nil))

		jwtToken, err := mgr.Create("proj:default:test", 100, "abc")
		require.NoError(t, err)

		_, _, err = mgr.Parse(jwtToken)
		require.NoError(t, err)
	})

	t.Run("Token Revoked", func(t *testing.T) {
		proj := appv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: appv1.AppProjectSpec{Roles: []appv1.ProjectRole{{Name: "test"}}},
		}

		mgr := newSessionManager(settingsMgr, getProjLister(&proj), NewUserStateStorage(nil))

		jwtToken, err := mgr.Create("proj:default:test", 10, "")
		require.NoError(t, err)

		_, _, err = mgr.Parse(jwtToken)
		require.Error(t, err)

		assert.Contains(t, err.Error(), "does not exist in project 'default'")
	})
}

type claimsMock struct {
	err error
}

func (cm *claimsMock) Valid() error {
	return cm.err
}

type tokenVerifierMock struct {
	claims *claimsMock
	err    error
}

func (tm *tokenVerifierMock) VerifyToken(token string) (jwt.Claims, string, error) {
	if tm.claims == nil {
		return nil, "", tm.err
	}
	return tm.claims, "", tm.err
}

func strPointer(str string) *string {
	return &str
}

func TestSessionManager_WithAuthMiddleware(t *testing.T) {
	handlerFunc := func() func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			t.Helper()
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/text")
			_, err := w.Write([]byte("Ok"))
			if err != nil {
				t.Fatalf("error writing response: %s", err)
			}
		}
	}
	type testCase struct {
		name                 string
		authDisabled         bool
		cookieHeader         bool
		verifiedClaims       *claimsMock
		verifyTokenErr       error
		expectedStatusCode   int
		expectedResponseBody *string
	}

	cases := []testCase{
		{
			name:                 "will authenticate successfully",
			authDisabled:         false,
			cookieHeader:         true,
			verifiedClaims:       &claimsMock{},
			verifyTokenErr:       nil,
			expectedStatusCode:   200,
			expectedResponseBody: strPointer("Ok"),
		},
		{
			name:                 "will be noop if auth is disabled",
			authDisabled:         true,
			cookieHeader:         false,
			verifiedClaims:       nil,
			verifyTokenErr:       nil,
			expectedStatusCode:   200,
			expectedResponseBody: strPointer("Ok"),
		},
		{
			name:                 "will return 400 if no cookie header",
			authDisabled:         false,
			cookieHeader:         false,
			verifiedClaims:       &claimsMock{},
			verifyTokenErr:       nil,
			expectedStatusCode:   400,
			expectedResponseBody: nil,
		},
		{
			name:                 "will return 401 verify token fails",
			authDisabled:         false,
			cookieHeader:         true,
			verifiedClaims:       &claimsMock{},
			verifyTokenErr:       stderrors.New("token error"),
			expectedStatusCode:   401,
			expectedResponseBody: nil,
		},
		{
			name:                 "will return 200 if claims are nil",
			authDisabled:         false,
			cookieHeader:         true,
			verifiedClaims:       nil,
			verifyTokenErr:       nil,
			expectedStatusCode:   200,
			expectedResponseBody: strPointer("Ok"),
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// given
			mux := http.NewServeMux()
			mux.HandleFunc("/", handlerFunc())
			tm := &tokenVerifierMock{
				claims: tc.verifiedClaims,
				err:    tc.verifyTokenErr,
			}
			ts := httptest.NewServer(WithAuthMiddleware(tc.authDisabled, tm, mux))
			defer ts.Close()
			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatalf("error creating request: %s", err)
			}
			if tc.cookieHeader {
				req.Header.Add("Cookie", "argocd.token=123456")
			}

			// when
			resp, err := http.DefaultClient.Do(req)

			// then
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)
			if tc.expectedResponseBody != nil {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				actual := strings.TrimSuffix(string(body), "\n")
				assert.Contains(t, actual, *tc.expectedResponseBody)
			}
		})
	}
}

var loggedOutContext = context.Background()

// nolint:staticcheck
var loggedInContext = context.WithValue(context.Background(), "claims", &jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "bar", "groups": []string{"baz"}})

func TestIss(t *testing.T) {
	assert.Empty(t, Iss(loggedOutContext))
	assert.Equal(t, "qux", Iss(loggedInContext))
}

func TestLoggedIn(t *testing.T) {
	assert.False(t, LoggedIn(loggedOutContext))
	assert.True(t, LoggedIn(loggedInContext))
}

func TestUsername(t *testing.T) {
	assert.Empty(t, Username(loggedOutContext))
	assert.Equal(t, "bar", Username(loggedInContext))
}

func TestSub(t *testing.T) {
	assert.Empty(t, Sub(loggedOutContext))
	assert.Equal(t, "foo", Sub(loggedInContext))
}

func TestGroups(t *testing.T) {
	assert.Empty(t, Groups(loggedOutContext, []string{"groups"}))
	assert.Equal(t, []string{"baz"}, Groups(loggedInContext, []string{"groups"}))
}

func TestVerifyUsernamePassword(t *testing.T) {
	const password = "password"

	for _, tc := range []struct {
		name     string
		disabled bool
		userName string
		password string
		expected error
	}{
		{
			name:     "Success if userName and password is correct",
			disabled: false,
			userName: common.ArgoCDAdminUsername,
			password: password,
			expected: nil,
		},
		{
			name:     "Return error if password is empty",
			disabled: false,
			userName: common.ArgoCDAdminUsername,
			password: "",
			expected: status.Errorf(codes.Unauthenticated, blankPasswordError),
		},
		{
			name:     "Return error if password is not correct",
			disabled: false,
			userName: common.ArgoCDAdminUsername,
			password: "foo",
			expected: status.Errorf(codes.Unauthenticated, invalidLoginError),
		},
		{
			name:     "Return error if disableAdmin is true",
			disabled: true,
			userName: common.ArgoCDAdminUsername,
			password: password,
			expected: status.Errorf(codes.Unauthenticated, accountDisabled, "admin"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient(password, !tc.disabled), "argocd")

			mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

			err := mgr.VerifyUsernamePassword(tc.userName, tc.password)

			if tc.expected == nil {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expected.Error())
			}
		})
	}
}

func TestCacheValueGetters(t *testing.T) {
	t.Run("Default values", func(t *testing.T) {
		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)
	})

	t.Run("Valid environment overrides", func(t *testing.T) {
		t.Setenv(envLoginMaxFailCount, "5")
		t.Setenv(envLoginMaxCacheSize, "5")

		mlf := getMaxLoginFailures()
		assert.Equal(t, 5, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, 5, mcs)
	})

	t.Run("Invalid environment overrides", func(t *testing.T) {
		t.Setenv(envLoginMaxFailCount, "invalid")
		t.Setenv(envLoginMaxCacheSize, "invalid")

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)
	})

	t.Run("Less than allowed in environment overrides", func(t *testing.T) {
		t.Setenv(envLoginMaxFailCount, "-1")
		t.Setenv(envLoginMaxCacheSize, "-1")

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)
	})

	t.Run("Greater than allowed in environment overrides", func(t *testing.T) {
		t.Setenv(envLoginMaxFailCount, fmt.Sprintf("%d", math.MaxInt32+1))
		t.Setenv(envLoginMaxCacheSize, fmt.Sprintf("%d", math.MaxInt32+1))

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)
	})
}

func TestLoginRateLimiter(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	storage := NewUserStateStorage(nil)

	mgr := newSessionManager(settingsMgr, getProjLister(), storage)

	t.Run("Test login delay valid user", func(t *testing.T) {
		for i := 0; i < getMaxLoginFailures(); i++ {
			err := mgr.VerifyUsernamePassword("admin", "wrong")
			require.Error(t, err)
		}

		// The 11th time should fail even if password is right
		{
			err := mgr.VerifyUsernamePassword("admin", "password")
			require.Error(t, err)
		}

		storage.attempts = map[string]LoginAttempts{}
		// Failed counter should have been reset, should validate immediately
		{
			err := mgr.VerifyUsernamePassword("admin", "password")
			require.NoError(t, err)
		}
	})

	t.Run("Test login delay invalid user", func(t *testing.T) {
		for i := 0; i < getMaxLoginFailures(); i++ {
			err := mgr.VerifyUsernamePassword("invalid", "wrong")
			require.Error(t, err)
		}

		err := mgr.VerifyUsernamePassword("invalid", "wrong")
		require.Error(t, err)
	})
}

func TestMaxUsernameLength(t *testing.T) {
	username := ""
	for i := 0; i < maxUsernameLength+1; i++ {
		username += "a"
	}
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))
	err := mgr.VerifyUsernamePassword(username, "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf(usernameTooLongError, maxUsernameLength))
}

func TestMaxCacheSize(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	invalidUsers := []string{"invalid1", "invalid2", "invalid3", "invalid4", "invalid5", "invalid6", "invalid7"}
	// Temporarily decrease max cache size
	t.Setenv(envLoginMaxCacheSize, "5")

	for _, user := range invalidUsers {
		err := mgr.VerifyUsernamePassword(user, "password")
		require.Error(t, err)
	}

	assert.Len(t, mgr.GetLoginFailures(), 5)
}

func TestFailedAttemptsExpiry(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	invalidUsers := []string{"invalid1", "invalid2", "invalid3", "invalid4", "invalid5", "invalid6", "invalid7"}

	t.Setenv(envLoginFailureWindowSeconds, "1")

	for _, user := range invalidUsers {
		err := mgr.VerifyUsernamePassword(user, "password")
		require.Error(t, err)
	}

	time.Sleep(2 * time.Second)

	err := mgr.VerifyUsernamePassword("invalid8", "password")
	require.Error(t, err)
	assert.Len(t, mgr.GetLoginFailures(), 1)
}

func getKubeClientWithConfig(config map[string]string, secretConfig map[string][]byte) *fake.Clientset {
	mergedSecretConfig := map[string][]byte{
		"server.secretkey": []byte("Hello, world!"),
	}
	for key, value := range secretConfig {
		mergedSecretConfig[key] = value
	}

	return fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: config,
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: "argocd",
		},
		Data: mergedSecretConfig,
	})
}

func TestSessionManager_VerifyToken(t *testing.T) {
	oidcTestServer := utiltest.GetOIDCTestServer(t)
	t.Cleanup(oidcTestServer.Close)

	dexTestServer := utiltest.GetDexTestServer(t)
	t.Cleanup(dexTestServer.Close)

	t.Run("RS512 is supported", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, nil), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false
		// Use test server's client to avoid TLS issues.
		mgr.client = oidcTestServer.Client()

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		assert.NotContains(t, err.Error(), "oidc: id token signed with unsupported algorithm")
	})

	t.Run("oidcConfig.rootCA is respected", func(t *testing.T) {
		cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: oidcTestServer.TLS.Certificates[0].Certificate[0]})

		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
rootCA: |
  %s
`, oidcTestServer.URL, strings.ReplaceAll(string(cert), "\n", "\n  ")),
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, nil), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		// If the root CA is being respected, we won't get this error. The error message is environment-dependent, so
		// we check for either of the error messages associated with a failed cert check.
		assert.NotContains(t, err.Error(), "certificate is not trusted")
		assert.NotContains(t, err.Error(), "certificate signed by unknown authority")
	})

	t.Run("OIDC provider is Dex, TLS is configured", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": dexTestServer.URL,
			"dex.config": `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), dexTestServer.URL, nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = fmt.Sprintf("%s/api/dex", dexTestServer.URL)
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is external, TLS is configured", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is Dex, TLS is configured", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": dexTestServer.URL,
			"dex.config": `connectors:
- type: github
  name: GitHub
  config:
    clientID: aabbccddeeff00112233
    clientSecret: aabbccddeeff00112233`,
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), dexTestServer.URL, nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = fmt.Sprintf("%s/api/dex", dexTestServer.URL)
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is external, TLS is configured, OIDCTLSInsecureSkipVerify is true", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true",
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		assert.NotContains(t, err.Error(), "certificate is not trusted")
		assert.NotContains(t, err.Error(), "certificate signed by unknown authority")
	})

	t.Run("OIDC provider is external, TLS is not configured, OIDCTLSInsecureSkipVerify is true", func(t *testing.T) {
		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true",
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig, nil), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		// This is the error thrown when the test server's certificate _is_ being verified.
		assert.NotContains(t, err.Error(), "certificate is not trusted")
		assert.NotContains(t, err.Error(), "certificate signed by unknown authority")
	})

	t.Run("OIDC provider is external, audience is not specified", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
	})

	t.Run("OIDC provider is external, audience is not specified, absent audience is allowed", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
skipAudienceCheckWhenTokenHasNoAudience: true`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.NoError(t, err)
	})

	t.Run("OIDC provider is external, audience is not specified but is required", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
skipAudienceCheckWhenTokenHasNoAudience: false`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is external, audience is client ID, no allowed list specified", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"xxx"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.NoError(t, err)
	})

	t.Run("OIDC provider is external, audience is in allowed list", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
allowedAudiences:
- something`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"something"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.NoError(t, err)
	})

	t.Run("OIDC provider is external, audience is not in allowed list", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
allowedAudiences:
- something-else`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"something"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is external, audience is not client ID, and there is no allow list", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"something"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	t.Run("OIDC provider is external, audience is specified, but allow list is empty", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
allowedAudiences: []`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"something"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})

	// Make sure the logic works to allow any of the allowed audiences, not just the first one.
	t.Run("OIDC provider is external, audience is specified, actual audience isn't the first allowed audience", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
allowedAudiences: ["aud-a", "aud-b"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"aud-b"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.NoError(t, err)
	})

	t.Run("OIDC provider is external, audience is not specified, token is signed with the wrong key", func(t *testing.T) {
		config := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
			"oidc.tls.insecure.skip.verify": "true", // This isn't what we're testing.
		}

		// This is not actually used in the test. The test only calls the OIDC test server. But a valid cert/key pair
		// must be set to test VerifyToken's behavior when Argo CD is configured with TLS enabled.
		secretConfig := map[string][]byte{
			"tls.crt": utiltest.Cert,
			"tls.key": utiltest.PrivateKey,
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(config, secretConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false

		claims := jwt.RegisteredClaims{Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey2)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		require.Error(t, err)
		assert.ErrorIs(t, err, common.TokenVerificationErr)
	})
}

func Test_PickFailureAttemptWhenOverflowed(t *testing.T) {
	t.Run("Not pick admin user from the queue", func(t *testing.T) {
		failures := map[string]LoginAttempts{
			"admin": {
				FailCount: 1,
			},
			"test2": {
				FailCount: 1,
			},
		}

		// inside pickRandomNonAdminLoginFailure, it uses random, so we need to test it multiple times
		for i := 0; i < 1000; i++ {
			user := pickRandomNonAdminLoginFailure(failures, "test")
			assert.Equal(t, "test2", *user)
		}
	})

	t.Run("Not pick admin user and current user from the queue", func(t *testing.T) {
		failures := map[string]LoginAttempts{
			"test": {
				FailCount: 1,
			},
			"admin": {
				FailCount: 1,
			},
			"test2": {
				FailCount: 1,
			},
		}

		// inside pickRandomNonAdminLoginFailure, it uses random, so we need to test it multiple times
		for i := 0; i < 1000; i++ {
			user := pickRandomNonAdminLoginFailure(failures, "test")
			assert.Equal(t, "test2", *user)
		}
	})
}
