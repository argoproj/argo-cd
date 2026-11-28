package upgrade

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func setupUpgrade() *Upgrade {
	return &Upgrade{
		namespace:      "test-ns",
		currentVersion: Version{Major: 2, Minor: 14, Patch: 8, SemVer: "2.14.8", Number: 2.14},
		upgradeVersion: Version{Major: 3, Minor: 1, Patch: 0, SemVer: "3.1.0", Number: 3.1},
	}
}

func TestGetCurrentVersion(t *testing.T) {
	u := setupUpgrade()
	expectedMajor := 2
	expectedMinor := 14
	expectedPatch := 8
	expectedSemVer := "2.14.8"
	expectedNumber := 2.14
	assert.Equal(t, expectedMajor, u.GetCurrentVersion().Major, "Expected major version to match")
	assert.Equal(t, expectedMinor, u.GetCurrentVersion().Minor, "Expected minor version to match")
	assert.Equal(t, expectedPatch, u.GetCurrentVersion().Patch, "Expected patch version to match")
	assert.Equal(t, expectedSemVer, u.GetCurrentVersion().SemVer, "Expected semver to match")
	assert.InDelta(t, expectedNumber, u.GetCurrentVersion().Number, 0.0001)
}

func TestGetUpgradeVersion(t *testing.T) {
	u := setupUpgrade()
	expectedMajor := 3
	expectedMinor := 1
	expectedPatch := 0
	expectedSemVer := "3.1.0"
	expectedNumber := 3.1
	assert.Equal(t, expectedMajor, u.GetUpgradeVersion().Major, "Expected major version to match")
	assert.Equal(t, expectedMinor, u.GetUpgradeVersion().Minor, "Expected minor version to match")
	assert.Equal(t, expectedPatch, u.GetUpgradeVersion().Patch, "Expected patch version to match")
	assert.Equal(t, expectedSemVer, u.GetUpgradeVersion().SemVer, "Expected semver to match")
	assert.InDelta(t, expectedNumber, u.GetUpgradeVersion().Number, 0.0001)
}

func TestSetCurrentVersion_ValidTag(t *testing.T) {
	u := setupUpgrade()
	tag := "v1.4.5"

	err := u.SetCurrentVersion(tag)

	require.NoError(t, err, "Expected no error for valid version tag")
	assert.Equal(t, 1, u.currentVersion.Major, "Expected major version 1")
	assert.Equal(t, 4, u.currentVersion.Minor, "Expected minor version 4")
	assert.Equal(t, 5, u.currentVersion.Patch, "Expected patch version 5")
	assert.Equal(t, "1.4.5", u.currentVersion.SemVer, "Expected semver to match")
	assert.InDelta(t, 1.4, u.currentVersion.Number, 0.0001)
}

func TestSetCurrentVersion_InvalidTag(t *testing.T) {
	u := setupUpgrade()
	invalidTag := "invalid"

	err := u.SetCurrentVersion(invalidTag)

	require.Error(t, err, "Expected error for invalid version tag")
	assert.Equal(t, Version{}, u.currentVersion, "Expected current version to be empty on error")
}

func TestSetUpgradeVersion_ValidTag(t *testing.T) {
	u := setupUpgrade()
	tag := "v3.2.1"

	err := u.SetUpgradeVersion(tag)

	require.NoError(t, err, "Expected no error for valid version tag")
	assert.Equal(t, 3, u.upgradeVersion.Major, "Expected major version 3")
	assert.Equal(t, 2, u.upgradeVersion.Minor, "Expected minor version 2")
	assert.Equal(t, 1, u.upgradeVersion.Patch, "Expected patch version 1")
	assert.Equal(t, "3.2.1", u.upgradeVersion.SemVer, "Expected semver to match")
	assert.InDelta(t, 3.2, u.upgradeVersion.Number, 0.0001)
}

func TestSetUpgradeVersion_InvalidTag(t *testing.T) {
	u := &Upgrade{}
	invalidTag := "not-a-version"

	err := u.SetUpgradeVersion(invalidTag)

	require.Error(t, err, "Expected error for invalid version tag")
	assert.Equal(t, Version{}, u.upgradeVersion, "Expected upgrade version to be empty on error")
}

func TestGetClientSet(t *testing.T) {
	u := setupUpgrade()
	expectedClientSet := fake.NewClientset()
	u.clientSet = expectedClientSet
	actualClientSet := u.getClientSet()
	assert.Equal(t, expectedClientSet, actualClientSet, "Expected getClientSet to return the clientSet")
}

func TestSetClientSet(t *testing.T) {
	u := setupUpgrade()
	expectedClientSet := fake.NewClientset()
	u.setClientSet(expectedClientSet)
	actualClientSet := u.getClientSet()
	assert.Equal(t, expectedClientSet, actualClientSet, "setClientSet should assign the correct clientSet")
}

func TestGetNamespace(t *testing.T) {
	u := setupUpgrade()
	expectedNamespace := "orange"
	u.namespace = expectedNamespace
	actualNamespace := u.getNamespace()
	assert.Equal(t, expectedNamespace, actualNamespace, "getNamespace should return the correct namespace")
}

func TestSetNamespace(t *testing.T) {
	u := setupUpgrade()
	expectedNamespace := "test-ns"
	u.setNamespace(expectedNamespace)
	assert.Equal(t, expectedNamespace, u.namespace, "setNamespace should assign the correct namespace")
}

func TestGetConfigMap(t *testing.T) {
	u := setupUpgrade()
	name := "test-cm"
	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: u.namespace,
		},
		Data: map[string]string{"key": "value"},
	}

	u.clientSet = fake.NewClientset(expected)
	result, err := u.getConfigMap(name)

	require.NoError(t, err)
	assert.Equal(t, expected.Name, result.Name)
	assert.Equal(t, expected.Data, result.Data)

	_, err = u.getConfigMap("bad-name")
	require.Error(t, err)
}

func TestConfigMapExists(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pear"},
	}

	u.clientSet = fake.NewClientset(cm)

	expected := u.configMapExists("test-cm")
	assert.True(t, expected)
	expected = u.configMapExists("bad-name")
	assert.False(t, expected)
}

func TestConfigMapKeyExists_True(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pear"},
	}
	result := u.configMapKeyExists(cm, "fruit")
	assert.True(t, result)
}

func TestConfigMapKeyExists_False(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{},
	}
	result := u.configMapKeyExists(cm, "fruit")
	assert.False(t, result)
}

func TestConfigMapValueContains_True(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pears"},
	}
	result := u.configMapValueContains(cm, "fruit", "ear")
	assert.True(t, result)
}

func TestConfigMapValueContains_False(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pear"},
	}
	result := u.configMapValueContains(cm, "fruit", "apple")
	assert.False(t, result)
}

func TestConfigMapValueEqual_True(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pear"},
	}
	result := u.configMapValueEqual(cm, "fruit", "pear")
	assert.True(t, result)
}

func TestConfigMapValueEqual_False(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "pear"},
	}
	result := u.configMapValueEqual(cm, "fruit", "pea")
	assert.False(t, result)
}

func TestConfigMapValueRegex_Matches(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "apple pear apple"},
	}
	results := u.configMapValueRegex(cm, "fruit", `(apple)`)
	assert.Len(t, results, 2)
	assert.Equal(t, "apple", results[0])
	assert.Equal(t, "apple", results[1])
}

func TestConfigMapValueRegex_NoMatches(t *testing.T) {
	u := setupUpgrade()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: u.namespace,
		},
		Data: map[string]string{"fruit": "apple pear apple"},
	}
	results := u.configMapValueRegex(cm, "fruit", `(grape)`)
	assert.Empty(t, results)
}

func TestGetVersionElements_ValidInputs(t *testing.T) {
	tests := []struct {
		input          string
		expectedMajor  int
		expectedMinor  int
		expectedPatch  int
		expectedSemVer string
		expectedNumber float64
	}{
		{"v3.1.0", 3, 1, 0, "3.1.0", 3.1},
		{"3.2.5", 3, 2, 5, "3.2.5", 3.2},
		{"v10.20.30", 10, 20, 30, "10.20.30", 10.20},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			version, err := getVersionElements(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMajor, version.Major)
			assert.Equal(t, tt.expectedMinor, version.Minor)
			assert.Equal(t, tt.expectedPatch, version.Patch)
			assert.Equal(t, tt.expectedSemVer, version.SemVer)
			assert.InDelta(t, tt.expectedNumber, version.Number, 0.0001)
		})
	}
}

func TestGetVersionElements_InvalidInputs(t *testing.T) {
	invalidInputs := []string{
		"v3", "v3.1", "v3.1.x", "version3.1.0", "v.1.2", "", "3..1.0",
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			_, err := getVersionElements(input)
			require.Error(t, err)
		})
	}
}

func TestGetKubeConfigWithClient_Success(t *testing.T) {
	u := setupUpgrade()
	originalNewForConfig := NewForConfig
	defer func() { NewForConfig = originalNewForConfig }()
	mockCfg := &rest.Config{}
	mockNamespace := "test-namespace"
	mockKubeClient := &mockClientSet{}
	NewForConfig = func(_ *rest.Config) (kubernetes.Interface, error) {
		return mockKubeClient, nil
	}
	mockClientCfg := new(mockClientConfig)
	mockClientCfg.On("ClientConfig").Return(mockCfg, nil)
	mockClientCfg.On("Namespace").Return(mockNamespace, true, nil)

	err := u.getKubeConfigWithClient(mockClientCfg)

	require.NoError(t, err)
	assert.Equal(t, mockNamespace, u.getNamespace())
	assert.Equal(t, mockKubeClient, u.getClientSet())

	mockClientCfg.AssertExpectations(t)
}

func TestGetKubeConfigWithClient_ClientConfigError(t *testing.T) {
	u := setupUpgrade()
	mockClientCfg := new(mockClientConfig)
	mockClientCfg.On("ClientConfig").Return((*rest.Config)(nil), errors.New("config error"))

	err := u.getKubeConfigWithClient(mockClientCfg)

	require.EqualError(t, err, "config error")
	mockClientCfg.AssertExpectations(t)
}

func TestGetKubeConfigWithClient_NamespaceError(t *testing.T) {
	u := setupUpgrade()
	mockCfg := &rest.Config{}
	mockClientCfg := new(mockClientConfig)
	mockClientCfg.On("ClientConfig").Return(mockCfg, nil)
	mockClientCfg.On("Namespace").Return("", false, errors.New("namespace error"))

	err := u.getKubeConfigWithClient(mockClientCfg)

	require.EqualError(t, err, "namespace error")
	mockClientCfg.AssertExpectations(t)
}

func TestGetKubeConfigWithClient_NewForConfigError(t *testing.T) {
	u := setupUpgrade()
	originalNewForConfig := NewForConfig
	defer func() { NewForConfig = originalNewForConfig }()
	mockCfg := &rest.Config{}
	NewForConfig = func(_ *rest.Config) (kubernetes.Interface, error) {
		return nil, errors.New("clientset error")
	}
	mockClientCfg := new(mockClientConfig)
	mockClientCfg.On("ClientConfig").Return(mockCfg, nil)
	mockClientCfg.On("Namespace").Return("test", true, nil)

	err := u.getKubeConfigWithClient(mockClientCfg)

	require.EqualError(t, err, "clientset error")
	mockClientCfg.AssertExpectations(t)
}

func TestRun_Success(t *testing.T) {
	u := setupUpgrade()
	mockConfig := new(mockClientConfig)
	mockRestConfig := &rest.Config{}
	mockClient := fake.NewClientset()
	mockConfig.On("ClientConfig").Return(mockRestConfig, nil)
	mockConfig.On("Namespace").Return("default", false, nil)
	originalNewForConfig := NewForConfig
	NewForConfig = func(_ *rest.Config) (kubernetes.Interface, error) {
		return mockClient, nil
	}
	defer func() { NewForConfig = originalNewForConfig }()
	originalGetCheck := getCheck
	getCheck = func(_ string) (Check, error) {
		mockCheck := new(mockCheck)
		mockCheck.On("performChecks", mock.Anything).Return([]CheckResult{
			{
				title:       "Test Check",
				description: "This is a test check",
				rules: []Rule{
					{
						title:   "Rule 1",
						actions: []string{"Action 1"},
						result:  checkPass,
					},
				},
			},
		}, nil)
		return mockCheck, nil
	}
	defer func() { getCheck = originalGetCheck }()

	err := u.SetCurrentVersion("v2.0.0")
	require.NoError(t, err)
	err = u.SetUpgradeVersion("v3.0.0")
	require.NoError(t, err)

	err = Run(u, mockConfig)
	require.NoError(t, err)

	mockConfig.AssertExpectations(t)
}

func TestRun_GetCheckFails(t *testing.T) {
	u := setupUpgrade()
	mockConfig := new(mockClientConfig)
	mockRestConfig := &rest.Config{}
	mockClient := fake.NewClientset()
	mockConfig.On("ClientConfig").Return(mockRestConfig, nil)
	mockConfig.On("Namespace").Return("default", false, nil)
	originalNewForConfig := NewForConfig
	NewForConfig = func(_ *rest.Config) (kubernetes.Interface, error) {
		return mockClient, nil
	}
	defer func() { NewForConfig = originalNewForConfig }()
	originalGetCheck := getCheck
	getCheck = func(_ string) (Check, error) {
		return nil, errors.New("failed to get check")
	}
	defer func() { getCheck = originalGetCheck }()

	_ = u.SetCurrentVersion("v99.99.99")
	_ = u.SetUpgradeVersion("v3.0.0")

	err := Run(u, mockConfig)
	require.Error(t, err)
	require.EqualError(t, err, "failed to get check")
}
