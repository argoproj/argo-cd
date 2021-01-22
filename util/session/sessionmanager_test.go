package session

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go/v4"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/password"
	"github.com/argoproj/argo-cd/util/settings"
)

func getKubeClient(pass string, enabled bool) *fake.Clientset {
	const defaultSecretKey = "Hello, world!"

	bcrypt, err := password.HashPassword(pass)
	errors.CheckError(err)

	return fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
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

func newSessionManager(settingsMgr *settings.SettingsManager, storage UserStateStorage) *SessionManager {
	mgr := NewSessionManager(settingsMgr, "", storage)
	mgr.verificationDelayNoiseEnabled = false
	return mgr
}

func TestSessionManager(t *testing.T) {
	const (
		defaultSubject = "admin"
	)
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("pass", true), "argocd")
	mgr := newSessionManager(settingsMgr, NewInMemoryUserStateStorage())

	token, err := mgr.Create(defaultSubject, 0, "")
	if err != nil {
		t.Errorf("Could not create token: %v", err)
	}

	claims, err := mgr.Parse(token)
	if err != nil {
		t.Errorf("Could not parse token: %v", err)
	}

	mapClaims := *(claims.(*jwt.MapClaims))
	subject := mapClaims["sub"].(string)
	if subject != "admin" {
		t.Errorf("Token claim subject \"%s\" does not match expected subject \"%s\".", subject, defaultSubject)
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

			mgr := newSessionManager(settingsMgr, NewInMemoryUserStateStorage())

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
	storage := NewInMemoryUserStateStorage()

	mgr := newSessionManager(settingsMgr, storage)

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
	mgr := newSessionManager(settingsMgr, NewInMemoryUserStateStorage())
	err := mgr.VerifyUsernamePassword(username, "password")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf(usernameTooLongError, maxUsernameLength))
}

func TestMaxCacheSize(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), getKubeClient("password", true), "argocd")
	mgr := newSessionManager(settingsMgr, NewInMemoryUserStateStorage())

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
	mgr := newSessionManager(settingsMgr, NewInMemoryUserStateStorage())

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
