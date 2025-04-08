package upgrade

import (
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type mockClientConfig struct {
	mock.Mock
}

func (m *mockClientConfig) RawConfig() (api.Config, error) {
	panic("unused")
}

func (m *mockClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	panic("unused")
}

func (m *mockClientConfig) ClientConfig() (*rest.Config, error) {
	args := m.Called()
	return args.Get(0).(*rest.Config), args.Error(1)
}

func (m *mockClientConfig) Namespace() (string, bool, error) {
	args := m.Called()
	return args.String(0), args.Bool(1), args.Error(2)
}

type mockClientSet struct {
	kubernetes.Interface
}

type mockCheck struct {
	mock.Mock
}

func (m *mockCheck) performChecks(u *Upgrade) ([]CheckResult, error) {
	args := m.Called(u)
	return args.Get(0).([]CheckResult), args.Error(1)
}
