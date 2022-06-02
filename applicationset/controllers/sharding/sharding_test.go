package sharding

import (
	"errors"
	"github.com/argoproj/argo-cd/v2/common"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"testing"
)

func TestInferShardFromHostname(t *testing.T) {
	type args struct {
		hostnameDetector func() (string, error)
	}
	tests := []struct {
		name          string
		args          args
		expectedShard int
		expectingErr  bool
	}{
		{
			name: "Should return error when detector returns an error",
			args: args{hostnameDetector: func() (string, error) {
				return "", errors.New("fake-error")
			}},
			expectedShard: 0,
			expectingErr:  true,
		},
		{
			name: "should return err when hostname does contain -",
			args: args{hostnameDetector: func() (string, error) {
				return "fakehostname", nil
			}},
			expectedShard: 0,
			expectingErr:  true,
		},
		{
			name: "Should return error when hostname does not end with -<number>",
			args: args{hostnameDetector: func() (string, error) {
				return "fake-hostname", nil
			}},
			expectedShard: 0,
			expectingErr:  true,
		},
		{
			name: "Should return shard number successfully",
			args: args{hostnameDetector: func() (string, error) {
				return "fake-hostname-12", nil
			}},
			expectedShard: 12,
			expectingErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InferShardFromHostname(tt.args.hostnameDetector)
			assert.Equal(t, tt.expectingErr, err != nil)
			assert.Equalf(t, tt.expectedShard, got, "InferShardFromHostname(%v)", tt.args.hostnameDetector)
		})
	}
}

func TestInferShard(t *testing.T) {
	type args struct {
		hostnameDetector func() (string, error)
	}
	tests := []struct {
		name          string
		envVars       map[string]string
		args          args
		expectedShard int
		expectingErr  bool
	}{
		{
			name: "should detect shard number from env successfully",
			envVars: map[string]string{
				common.EnvApplicationSetControllerShard: "6",
			},
			args: args{hostnameDetector: func() (string, error) {
				return "fake-hostname-12", nil
			}},
			expectedShard: 6,
			expectingErr:  false,
		},
		{
			name: "should fallback to hostname based detection when the given shard number is less than zero",
			envVars: map[string]string{
				common.EnvApplicationSetControllerShard: "-6",
			},
			args: args{hostnameDetector: func() (string, error) {
				return "fake-hostname-12", nil
			}},
			expectedShard: 12,
			expectingErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			got, err := InferShard(tt.args.hostnameDetector)
			assert.Equal(t, tt.expectingErr, err != nil)
			assert.Equalf(t, tt.expectedShard, got, "InferShard(%v)", tt.args.hostnameDetector)
		})
	}
}

func TestGenerateApplicationSetFilterForStatefulSetShouldReturnNoFilterIfNoReplicaSpecified(t *testing.T) {
	// Given
	mockHostnameDetector := func() (string, error) {
		return "fake-hostname-12", nil
	}

	// When
	filter, err := GenerateApplicationSetFilterForStatefulSet(mockHostnameDetector)

	// Then
	assert.NoError(t, err)
	assert.True(t, reflect.ValueOf(noFilter).Pointer() == reflect.ValueOf(filter).Pointer())
}

func TestGenerateApplicationSetFilterForStatefulSetShouldReturnErrorWhenCouldNotInferShard(t *testing.T) {
	// Given
	t.Setenv(common.EnvApplicationSetControllerReplicas, "10")
	mockHostnameDetector := func() (string, error) {
		return "invalidhostname", nil
	}

	// When
	filter, err := GenerateApplicationSetFilterForStatefulSet(mockHostnameDetector)

	// Then
	assert.Error(t, err)
	assert.Nil(t, filter)
}

func TestGenerateApplicationSetFilterForStatefulSetShouldReturnErrorWhenInferredShardGreaterThanReplica(t *testing.T) {
	// Given
	t.Setenv(common.EnvApplicationSetControllerReplicas, "10")
	t.Setenv(common.EnvApplicationSetControllerShard, "11")
	mockHostnameDetector := func() (string, error) {
		return "invalidhostname", nil
	}

	// When
	filter, err := GenerateApplicationSetFilterForStatefulSet(mockHostnameDetector)

	// Then
	assert.Error(t, err)
	assert.Nil(t, filter)
}

func TestGenerateApplicationSetFilterForStatefulSetShouldReturnFilterSuccessfully(t *testing.T) {
	// Given
	t.Setenv(common.EnvApplicationSetControllerReplicas, "10")
	mockHostnameDetector := func() (string, error) {
		return "hostname-8", nil
	}

	// When
	filter, err := GenerateApplicationSetFilterForStatefulSet(mockHostnameDetector)

	// Then
	assert.NoError(t, err)
	assert.NotNil(t, filter)

	firstAppset := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("5"),
		},
	}
	assert.True(t, filter(firstAppset))

	secondAppset := &argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("8"),
		},
	}
	assert.False(t, filter(secondAppset))
}
func TestGetShardByID(t *testing.T) {
	assert.Equal(t, 0, getShardByID("", 10))
	assert.Equal(t, 8, getShardByID("5", 10))
}
