package db

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/settings"
)

// GPG config map with a single key and good mapping
var gpgCMEmpty = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
}

// GPG config map with a single key and good mapping
var gpgCMSingleGoodPubkey = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
	Data: map[string]string{
		"4AEE18F83AFDEB23": test.MustLoadFileToString("../gpg/testdata/github.asc"),
	},
}

// GPG config map with two keys and good mapping
var gpgCMMultiGoodPubkey = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
	Data: map[string]string{
		"FDC79815400D88A9": test.MustLoadFileToString("../gpg/testdata/johndoe.asc"),
		"F7842A5CEAA9C0B1": test.MustLoadFileToString("../gpg/testdata/janedoe.asc"),
	},
}

// GPG config map with a single key and bad mapping
var gpgCMSingleKeyWrongId = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
	Data: map[string]string{
		"5AEE18F83AFDEB23": test.MustLoadFileToString("../gpg/testdata/github.asc"),
	},
}

// GPG config map with a garbage pub key
var gpgCMGarbagePubkey = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
	Data: map[string]string{
		"4AEE18F83AFDEB23": test.MustLoadFileToString("../gpg/testdata/garbage.asc"),
	},
}

// GPG config map with a wrong key
var gpgCMGarbageCMKey = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
	Data: map[string]string{
		"wullerosekaufe": test.MustLoadFileToString("../gpg/testdata/github.asc"),
	},
}

// Returns a fake client set for use in tests
func getGPGKeysClientset(gpgCM v1.ConfigMap) *fake.Clientset {
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: nil,
	}

	return fake.NewSimpleClientset([]runtime.Object{&cm, &gpgCM}...)
}

func Test_ValidatePGPKey(t *testing.T) {
	// Good case - single PGP key
	{
		key, err := validatePGPKey(test.MustLoadFileToString("../gpg/testdata/github.asc"))
		assert.NoError(t, err)
		assert.NotNil(t, key)
		assert.Equal(t, "4AEE18F83AFDEB23", key.KeyID)
		assert.NotEmpty(t, key.Owner)
		assert.NotEmpty(t, key.KeyData)
		assert.NotEmpty(t, key.SubType)
	}
	// Bad case - Garbage
	{
		key, err := validatePGPKey(test.MustLoadFileToString("../gpg/testdata/garbage.asc"))
		assert.Error(t, err)
		assert.Nil(t, key)
	}
	// Bad case - more than one key
	{
		key, err := validatePGPKey(test.MustLoadFileToString("../gpg/testdata/multi.asc"))
		assert.Error(t, err)
		assert.Nil(t, key)
	}
}

func Test_ListConfiguredGPGPublicKeys(t *testing.T) {
	// Good case. Single key in input, right mapping to Key ID in CM
	{
		clientset := getGPGKeysClientset(gpgCMSingleGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)
		if db == nil {
			panic("could not get database")
		}
		keys, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, keys, 1)
	}
	// Good case. No certificates in ConfigMap
	{
		clientset := getGPGKeysClientset(gpgCMEmpty)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)
		if db == nil {
			panic("could not get database")
		}
		keys, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, keys, 0)
	}
	// Bad case. Single key in input, wrong mapping to Key ID in CM
	{
		clientset := getGPGKeysClientset(gpgCMSingleKeyWrongId)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)
		if db == nil {
			panic("could not get database")
		}
		keys, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.Error(t, err)
		assert.Len(t, keys, 0)
	}
	// Bad case. Garbage public key
	{
		clientset := getGPGKeysClientset(gpgCMGarbagePubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)
		if db == nil {
			panic("could not get database")
		}
		keys, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.Error(t, err)
		assert.Len(t, keys, 0)
	}
	// Bad case. Garbage ConfigMap key in data
	{
		clientset := getGPGKeysClientset(gpgCMGarbageCMKey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)
		if db == nil {
			panic("could not get database")
		}
		keys, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.Error(t, err)
		assert.Len(t, keys, 0)
	}
}

func Test_AddGPGPublicKey(t *testing.T) {
	// Good case
	{
		clientset := getGPGKeysClientset(gpgCMEmpty)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		// Key should be added
		new, skipped, err := db.AddGPGPublicKey(context.Background(), test.MustLoadFileToString("../gpg/testdata/github.asc"))
		assert.NoError(t, err)
		assert.Len(t, new, 1)
		assert.Len(t, skipped, 0)
		cm, err := settings.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
		assert.NoError(t, err)
		assert.Len(t, cm.Data, 1)

		// Same key should not be added, but skipped
		new, skipped, err = db.AddGPGPublicKey(context.Background(), test.MustLoadFileToString("../gpg/testdata/github.asc"))
		assert.NoError(t, err)
		assert.Len(t, new, 0)
		assert.Len(t, skipped, 1)
		cm, err = settings.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
		assert.NoError(t, err)
		assert.Len(t, cm.Data, 1)

		// New keys should be added
		new, skipped, err = db.AddGPGPublicKey(context.Background(), test.MustLoadFileToString("../gpg/testdata/multi.asc"))
		assert.NoError(t, err)
		assert.Len(t, new, 2)
		assert.Len(t, skipped, 0)
		cm, err = settings.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
		assert.NoError(t, err)
		assert.Len(t, cm.Data, 3)

		// Same new keys should be skipped
		new, skipped, err = db.AddGPGPublicKey(context.Background(), test.MustLoadFileToString("../gpg/testdata/multi.asc"))
		assert.NoError(t, err)
		assert.Len(t, new, 0)
		assert.Len(t, skipped, 2)
		cm, err = settings.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
		assert.NoError(t, err)
		assert.Len(t, cm.Data, 3)

		// Garbage input should result in error
		new, skipped, err = db.AddGPGPublicKey(context.Background(), test.MustLoadFileToString("../gpg/testdata/garbage.asc"))
		assert.Error(t, err)
		assert.Nil(t, new)
		assert.Nil(t, skipped)
		cm, err = settings.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
		assert.NoError(t, err)
		assert.Len(t, cm.Data, 3)
	}
}

func Test_DeleteGPGPublicKey(t *testing.T) {
	defer os.Setenv("GNUPGHOME", "")
	// Good case
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		// Key should be removed
		err := db.DeleteGPGPublicKey(context.Background(), "FDC79815400D88A9")
		assert.NoError(t, err)

		// Key should not exist anymore, therefore can't be deleted again
		err = db.DeleteGPGPublicKey(context.Background(), "FDC79815400D88A9")
		assert.Error(t, err)

		// One key left in configuration
		n, err := db.ListConfiguredGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 1)

		// Key should be removed
		err = db.DeleteGPGPublicKey(context.Background(), "F7842A5CEAA9C0B1")
		assert.NoError(t, err)

		// Key should not exist anymore, therefore can't be deleted again
		err = db.DeleteGPGPublicKey(context.Background(), "F7842A5CEAA9C0B1")
		assert.Error(t, err)

		// No key left in configuration
		n, err = db.ListConfiguredGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 0)

	}
	// Bad case - empty ConfigMap
	{
		clientset := getGPGKeysClientset(gpgCMEmpty)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		// Key should be removed
		err := db.DeleteGPGPublicKey(context.Background(), "F7842A5CEAA9C0B1")
		assert.Error(t, err)
	}
}
