package db

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/settings"
)

// GPG config map with a single key and good mapping
var gpgCMEmpty = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "argocd-gpg-cm",
		Namespace: testNamespace,
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	},
}

// GPG config map with a single key and good mapping
var gpgCMSingleGoodPubkey = v1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "argocd-gpg-cm",
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
		Name:      "argocd-gpg-cm",
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
		Name:      "argocd-gpg-cm",
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
		Name:      "argocd-gpg-cm",
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
		Name:      "argocd-gpg-cm",
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

func Test_InitializeGPGKeyRing(t *testing.T) {
	defer os.Setenv("GNUPGHOME", "")

	// Good case
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 2)
		assert.Contains(t, n, "F7842A5CEAA9C0B1")
		assert.Contains(t, n, "FDC79815400D88A9")
	}

	// Bad case - unreachable GNUPGHOME
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path := "/some/where/non/existing"
		os.Setenv("GNUPGHOME", path)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.Error(t, err)
		assert.Len(t, n, 0)
	}

	// Bad case - bad permissions on GNUPGHOME
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)
		os.Chmod(path, 0100)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.Error(t, err)
		assert.Len(t, n, 0)
	}

	// Bad case - double initialization
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)

		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 2)

		n, err = db.InitializeGPGKeyRing(context.Background())
		assert.Error(t, err)
		assert.Len(t, n, 0)
	}
}

func Test_ListInstalledGPGPublicKeys(t *testing.T) {
	defer os.Setenv("GNUPGHOME", "")
	// Good case
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.NoError(t, err)

		n, err = db.ListInstalledGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Contains(t, n, "F7842A5CEAA9C0B1")
		assert.Contains(t, n, "FDC79815400D88A9")
	}

	// Bad case - not initialized
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path := "/some/where/non/existing"
		os.Setenv("GNUPGHOME", path)
		n, err := db.ListInstalledGPGPublicKeys(context.Background())
		assert.Error(t, err)
		assert.Len(t, n, 0)
	}
}

func Test_SynchronizeGPGPublicKeys(t *testing.T) {
	// Good case
	{
		clientset := getGPGKeysClientset(gpgCMEmpty)
		s := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, s, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 0)

		// Emulate an updated ConfigMap
		clientset = getGPGKeysClientset(gpgCMMultiGoodPubkey)
		s = settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db = NewDB(testNamespace, s, clientset)

		err = db.SynchronizeGPGPublicKeys(context.Background())
		assert.NoError(t, err)

		n, err = db.ListInstalledGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 2)
		assert.Contains(t, n, "FDC79815400D88A9")
		assert.Contains(t, n, "F7842A5CEAA9C0B1")

		// Emulate another new update on the ConfigMap
		clientset = getGPGKeysClientset(gpgCMSingleGoodPubkey)
		s = settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db = NewDB(testNamespace, s, clientset)

		err = db.SynchronizeGPGPublicKeys(context.Background())
		assert.NoError(t, err)

		n, err = db.ListInstalledGPGPublicKeys(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 1)
		assert.Contains(t, n, "4AEE18F83AFDEB23")
	}
}

// Only one process at a time is allowed to synchronize, test the locks
func Test_SynchronizeGPGPublicKeys_Multi(t *testing.T) {
	{
		clientset := getGPGKeysClientset(gpgCMMultiGoodPubkey)
		settings := settings.NewSettingsManager(context.Background(), clientset, testNamespace)
		db := NewDB(testNamespace, settings, clientset)

		path, err := ioutil.TempDir("", "gpg-unittest")
		if err != nil {
			panic(err.Error())
		}
		defer os.RemoveAll(path)
		os.Setenv("GNUPGHOME", path)
		n, err := db.InitializeGPGKeyRing(context.Background())
		assert.NoError(t, err)
		assert.Len(t, n, 2)

		numProcesses := 2

		errArr := make([]error, numProcesses+1)
		var wg sync.WaitGroup
		for i := 0; i < numProcesses; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errArr[idx] = db.SynchronizeGPGPublicKeys(context.Background())
			}(i)
		}

		wg.Wait()

		numErrors := 0
		for _, e := range errArr {
			if e != nil {
				numErrors += 1
			}
		}

		// Only one process should have performed the sync
		assert.Equal(t, numProcesses-1, numErrors)
	}

}
