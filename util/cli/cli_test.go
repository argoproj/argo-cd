package cli

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

var (
	k3sLocation string = "/etc/rancher/k3s/k3s.yaml"
	k8sLocation string = "/home/.kube/config"
)

func TestCustomKubeConfigLocation(t *testing.T) {
	testCases := []struct {
		name             string
		coreMode         bool
		newLocation      string
		expectedLocation string
	}{
		{
			name:             "kubeconfig is specified in non core mode",
			coreMode:         false,
			newLocation:      k8sLocation,
			expectedLocation: "",
		},
		{
			name:             "kubeconfig is not specified in non core mode",
			coreMode:         false,
			newLocation:      "",
			expectedLocation: "",
		},
		{
			name:             "kubeconfig is not specified in core mode",
			coreMode:         true,
			newLocation:      "",
			expectedLocation: "",
		},
		{
			name:             "kubeconfig is specified in core mode",
			coreMode:         true,
			newLocation:      k3sLocation,
			expectedLocation: k3sLocation,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var fakeFlags *pflag.FlagSet = pflag.NewFlagSet("tmp", pflag.ContinueOnError)

			clientConfig := AddKubectlFlagsToSetInCore(fakeFlags, testCase.coreMode, testCase.newLocation)
			assert.Equal(t, testCase.expectedLocation, clientConfig.ConfigAccess().GetExplicitFile())

		})
	}
}
