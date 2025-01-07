package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// nolint:unparam
func getSecret(client kubernetes.Interface, ns, name string) (*apiv1.Secret, error) {
	s, err := client.CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func Test_CreateOrUpdateSecretField(t *testing.T) {
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test",
			Labels: map[string]string{
				"label1": "bar",
				"label2": "baz",
			},
			Annotations: map[string]string{
				"annotation1": "bar",
				"annotation2": "baz",
			},
		},
		Data: map[string][]byte{
			"password": []byte("foobar"),
		},
	}

	labels := map[string]string{
		"label3": "foo",
	}
	annotations := map[string]string{
		"annotation3": "foo",
	}

	client := fake.NewSimpleClientset(secret)

	t.Run("Change field in existing secret", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretField("test", "test-secret", "password", "barfoo")
		require.NoError(t, err)
		s, err := getSecret(client, "test", "test-secret")
		require.NoError(t, err)

		// password field should be updated
		assert.Equal(t, "barfoo", string(s.Data["password"]))

		// Labels and annotations should be untouched
		assert.Len(t, s.Labels, 2)
		assert.Len(t, s.Annotations, 2)
	})

	t.Run("Change field in non-existing secret", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretField("test", "nonexist-secret", "password", "foobaz")
		require.NoError(t, err)
		s, err := getSecret(client, "test", "nonexist-secret")
		require.NoError(t, err)

		// password field should be requested value
		assert.Equal(t, "foobaz", string(s.Data["password"]))

		// Labels and annotations should be untouched
		assert.Empty(t, s.Labels)
		assert.Empty(t, s.Annotations)
	})

	t.Run("Change field in existing secret with labels", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO()).WithAnnotations(annotations).WithLabels(labels)
		err := ku.CreateOrUpdateSecretField("test", "test-secret", "password", "barfoo")
		require.NoError(t, err)
		s, err := getSecret(client, "test", "test-secret")
		require.NoError(t, err)

		// password field should be updated
		assert.Equal(t, "barfoo", string(s.Data["password"]))

		// Labels and annotations should be untouched
		assert.Len(t, s.Labels, 2)
		assert.Len(t, s.Annotations, 2)
	})

	t.Run("Change field in existing secret with labels", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO()).WithAnnotations(annotations).WithLabels(labels)
		err := ku.CreateOrUpdateSecretField("test", "nonexisting-secret", "password", "barfoo")
		require.NoError(t, err)
		s, err := getSecret(client, "test", "nonexisting-secret")
		require.NoError(t, err)

		// password field should be updated
		assert.Equal(t, "barfoo", string(s.Data["password"]))

		// Labels and annotations should be applied
		assert.Len(t, s.Labels, 1)
		assert.Len(t, s.Annotations, 1)
		assert.Contains(t, s.Labels, "label3")
		assert.Contains(t, s.Annotations, "annotation3")
	})
}

func Test_CreateOrUpdateSecretData(t *testing.T) {
	secret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test",
		},
		Data: map[string][]byte{
			"something": []byte("something"),
			"password":  []byte("foobar"),
			"foobar":    []byte("barfoo"),
		},
	}

	data1 := map[string][]byte{
		"password": []byte("barfoo"),
	}

	data2 := map[string][]byte{
		"password": []byte("foobarbaz"),
	}

	client := fake.NewSimpleClientset(secret)

	t.Run("Change data in existing secret with merge", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretData("test", "test-secret", data1, true)
		require.NoError(t, err)
		s, err := getSecret(client, "test", "test-secret")
		require.NoError(t, err)
		require.Contains(t, s.Data, "something")
		require.Contains(t, s.Data, "password")
		require.Equal(t, "barfoo", string(s.Data["password"]))
	})

	t.Run("Change data in non-existing secret with merge", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretData("test", "nonexist-secret", data1, true)
		require.NoError(t, err)
		s, err := getSecret(client, "test", "nonexist-secret")
		require.NoError(t, err)
		require.Len(t, s.Data, 1)
		require.Equal(t, "barfoo", string(s.Data["password"]))
	})

	t.Run("Change data in existing secret without merge", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretData("test", "test-secret", data2, false)
		require.NoError(t, err)
		s, err := getSecret(client, "test", "test-secret")
		require.NoError(t, err)
		require.Contains(t, s.Data, "password")
		require.NotContains(t, s.Data, "something")
		require.NotContains(t, s.Data, "foobar")
		require.Equal(t, "foobarbaz", string(s.Data["password"]))
	})

	t.Run("Change data in non-existing secret without merge", func(t *testing.T) {
		ku := NewKubeUtil(client, context.TODO())
		err := ku.CreateOrUpdateSecretData("test", "nonexist-secret", data2, false)
		require.NoError(t, err)
		s, err := getSecret(client, "test", "nonexist-secret")
		require.NoError(t, err)
		require.Len(t, s.Data, 1)
		require.Equal(t, "foobarbaz", string(s.Data["password"]))
	})
}
