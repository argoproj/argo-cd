package session

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.NotEmpty(t, newToken)

	// verify that new token is valid and for the same user
	claims, _, err := mgr.Parse(newToken)
	assert.NoError(t, err)
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
		assert.NoError(t, err)
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
				assert.Nil(t, err)
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
		os.Setenv(envLoginMaxFailCount, "5")
		os.Setenv(envLoginMaxCacheSize, "5")

		mlf := getMaxLoginFailures()
		assert.Equal(t, 5, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, 5, mcs)

		os.Setenv(envLoginMaxFailCount, "")
		os.Setenv(envLoginMaxCacheSize, "")
	})

	t.Run("Invalid environment overrides", func(t *testing.T) {
		os.Setenv(envLoginMaxFailCount, "invalid")
		os.Setenv(envLoginMaxCacheSize, "invalid")

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)

		os.Setenv(envLoginMaxFailCount, "")
		os.Setenv(envLoginMaxCacheSize, "")
	})

	t.Run("Less than allowed in environment overrides", func(t *testing.T) {
		os.Setenv(envLoginMaxFailCount, "-1")
		os.Setenv(envLoginMaxCacheSize, "-1")

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)

		os.Setenv(envLoginMaxFailCount, "")
		os.Setenv(envLoginMaxCacheSize, "")
	})

	t.Run("Greater than allowed in environment overrides", func(t *testing.T) {
		os.Setenv(envLoginMaxFailCount, fmt.Sprintf("%d", math.MaxInt32+1))
		os.Setenv(envLoginMaxCacheSize, fmt.Sprintf("%d", math.MaxInt32+1))

		mlf := getMaxLoginFailures()
		assert.Equal(t, defaultMaxLoginFailures, mlf)

		mcs := getMaximumCacheSize()
		assert.Equal(t, defaultMaxCacheSize, mcs)

		os.Setenv(envLoginMaxFailCount, "")
		os.Setenv(envLoginMaxCacheSize, "")
	})

}

func TestLoginRateLimiter(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	storage := NewUserStateStorage(nil)

	mgr := newSessionManager(settingsMgr, getProjLister(), storage)

	t.Run("Test login delay valid user", func(t *testing.T) {
		for i := 0; i < getMaxLoginFailures(); i++ {
			err := mgr.VerifyUsernamePassword("admin", "wrong")
			assert.Error(t, err)
		}

		// The 11th time should fail even if password is right
		{
			err := mgr.VerifyUsernamePassword("admin", "password")
			assert.Error(t, err)
		}

		storage.attempts = map[string]LoginAttempts{}
		// Failed counter should have been reset, should validate immediately
		{
			err := mgr.VerifyUsernamePassword("admin", "password")
			assert.NoError(t, err)
		}
	})

	t.Run("Test login delay invalid user", func(t *testing.T) {
		for i := 0; i < getMaxLoginFailures(); i++ {
			err := mgr.VerifyUsernamePassword("invalid", "wrong")
			assert.Error(t, err)
		}

		err := mgr.VerifyUsernamePassword("invalid", "wrong")
		assert.Error(t, err)
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf(usernameTooLongError, maxUsernameLength))
}

func TestMaxCacheSize(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	invalidUsers := []string{"invalid1", "invalid2", "invalid3", "invalid4", "invalid5", "invalid6", "invalid7"}
	// Temporarily decrease max cache size
	os.Setenv(envLoginMaxCacheSize, "5")

	for _, user := range invalidUsers {
		err := mgr.VerifyUsernamePassword(user, "password")
		assert.Error(t, err)
	}

	assert.Len(t, mgr.GetLoginFailures(), 5)
}

func TestFailedAttemptsExpiry(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, getProjLister(), NewUserStateStorage(nil))

	invalidUsers := []string{"invalid1", "invalid2", "invalid3", "invalid4", "invalid5", "invalid6", "invalid7"}

	os.Setenv(envLoginFailureWindowSeconds, "1")

	for _, user := range invalidUsers {
		err := mgr.VerifyUsernamePassword(user, "password")
		assert.Error(t, err)
	}

	time.Sleep(2 * time.Second)

	err := mgr.VerifyUsernamePassword("invalid8", "password")
	assert.Error(t, err)
	assert.Len(t, mgr.GetLoginFailures(), 1)

	os.Setenv(envLoginFailureWindowSeconds, "")
}

func oidcMockHandler(t *testing.T, url string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.RequestURI {
		case "/.well-known/openid-configuration":
			_, err := io.WriteString(w, fmt.Sprintf(`
{
  "issuer": "%[1]s",
  "authorization_endpoint": "%[1]s/auth",
  "token_endpoint": "%[1]s/token",
  "jwks_uri": "%[1]s/keys",
  "userinfo_endpoint": "%[1]s/userinfo",
  "device_authorization_endpoint": "%[1]s/device/code",
  "grant_types_supported": ["authorization_code"],
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS512"],
  "code_challenge_methods_supported": ["S256", "plain"],
  "scopes_supported": ["openid"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "aud", "exp"]
}`, url))
			require.NoError(t, err)
		default:
			w.WriteHeader(404)
		}
	}
}

func getOIDCTestServer(t *testing.T) *httptest.Server {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start with a placeholder. We need the server URL before setting up the real handler.
	}))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oidcMockHandler(t, ts.URL)(w, r)
	})
	return ts
}

func getKubeClientWithConfig(config map[string]string) *fake.Clientset {
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
		Data: map[string][]byte{
			"server.secretkey": []byte("Hello, world!"),
		},
	})
}

// privateKey is an RSA key used only for tests.
var privateKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEA2KHkp2fe0ReHqAt9BimWEec2ryWZIyg9jvB3BdP3mzFf0bOt
WlHm1FAETFxH4h5jYASUaaWEwRNNyGlT1GhTp+jOMC4xhOSb5/SnI2dt2EITkudQ
FKsSFUdJAndqOzkjrP2pL4fi4b7JWhuLDO36ufAP4l2m3tnAseGSSTIccWvzLFFU
s3wsHOHxOcJGCP1Z7rizxl6mTKYL/Z+GHqN17OJslDf901uPXsUeDCYL2iigGPhD
Ao6k8POsfbpqLG7poCDTK50FLnS5qEocjxt+J4ZjBEWTU/DOFXWYstzfbhm8OZPQ
pSSEiBCxpg+zjtkQfCyZxXB5RQ84CY78fXOI9QIDAQABAoIBAG8jL0FLIp62qZvm
uO9ualUo/37/lP7aaCpq50UQJ9lwjS3yNh8+IWQO4QWj2iUBXg4mi1Vf2ymKk78b
eixgkXp1D0Lcj/8ToYBwnUami04FKDGXhhf0Y8SS27vuM4vKlqjrQd7modkangYi
V0X82UKHDD8fuLpfkGIxzXDLypfMzjMuVpSntnWaf2YX3VR/0/66yEp9GejftF2k
wqhGoWM6r68pN5XuCqWd5PRluSoDy/o4BAFMhYCSfp9PjgZE8aoeWHgYzlZ3gUyn
r+HaDDNWbibhobXk/9h8lwAJ6KCZ5RZ+HFfh0HuwIxmocT9OCFgy/S0g1p+o3m9K
VNd5AMkCgYEA5fbS5UK7FBzuLoLgr1hktmbLJhpt8y8IPHNABHcUdE+O4/1xTQNf
pMUwkKjGG1MtrGjLOIoMGURKKn8lR1GMZueOTSKY0+mAWUGvSzl6vwtJwvJruT8M
otEO03o0tPnRKGxbFjqxkp2b6iqJ8MxCRZ3lSidc4mdi7PHzv9lwgvsCgYEA8Siq
7weCri9N6y+tIdORAXgRzcW54BmJyqB147c72RvbMacb6rN28KXpM3qnRXyp3Llb
yh81TW3FH10GqrjATws7BK8lP9kkAw0Z/7kNiS1NgH3pUbO+5H2kAa/6QW35nzRe
Jw2lyfYGWqYO4hYXH14ML1kjgS1hgd3XHOQ64M8CgYAKcjDYSzS2UC4dnMJaFLjW
dErsGy09a7iDDnUs/r/GHMsP3jZkWi/hCzgOiiwdl6SufUAl/FdaWnjH/2iRGco3
7nLPXC/3CFdVNp+g2iaSQRADtAFis9N+HeL/hkCYq/RtUqa8lsP0NgacF3yWnKCy
Ct8chDc67ZlXzBHXeCgdOwKBgHHGFPbWXUHeUW1+vbiyvrupsQSanznp8oclMtkv
Dk48hSokw9fzuU6Jh77gw9/Vk7HtxS9Tj+squZA1bDrJFPl1u+9WzkUUJZhG6xgp
bwhj1iejv5rrKUlVOTYOlwudXeJNa4oTNz9UEeVcaLMjZt9GmIsSC90a0uDZD26z
AlAjAoGAEoqm2DcNN7SrH6aVFzj1EVOrNsHYiXj/yefspeiEmf27PSAslP+uF820
SDpz4h+Bov5qTKkzcxuu1QWtA4M0K8Iy6IYLwb83DZEm1OsAf4i0pODz21PY/I+O
VHzjB10oYgaInHZgMUdyb6F571UdiYSB6a/IlZ3ngj5touy3VIM=
-----END RSA PRIVATE KEY-----`)

func TestSessionManager_VerifyToken(t *testing.T) {
	t.Run("HS256 is supported", func(t *testing.T) {
		oidcTestServer := getOIDCTestServer(t)
		defer oidcTestServer.Close()
		dexConfig := map[string]string{
			"url": "",
			"oidc.config": fmt.Sprintf(`
name: Test
issuer: %s
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`, oidcTestServer.URL),
		}

		settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClientWithConfig(dexConfig), "argocd")
		mgr := NewSessionManager(settingsMgr, getProjLister(), "", nil, NewUserStateStorage(nil))
		mgr.verificationDelayNoiseEnabled = false
		// Use test server's client to avoid TLS issues.
		mgr.client = oidcTestServer.Client()

		claims := jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"test-client"}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		claims.Issuer = oidcTestServer.URL
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(privateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)

		_, _, err = mgr.VerifyToken(tokenString)
		assert.NotContains(t, err.Error(), "oidc: id token signed with unsupported algorithm")
	})
}
