package configbus_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// TestControllerChainResolvesAllFields asserts the application-controller
// production chain resolves every Provider field getter without leaking
// ErrNotConfigured.
func TestControllerChainResolvesAllFields(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "argocd",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Data: map[string]string{},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: "argocd",
			Labels:    map[string]string{"app.kubernetes.io/part-of": "argocd"},
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	kubeClient := fake.NewClientset(cm, secret)
	settingsMgr := settings.NewSettingsManager(ctx, kubeClient, "argocd")
	require.NoError(t, settingsMgr.ResyncInformers())

	optsTimeout := 2 * time.Second
	labels := []string{"team"}
	var backoff *wait.Backoff
	chain := configbus.NewChainProvider(
		&configbus.StaticProvider{Fields: configbus.StaticFields{
			HardReconciliationTimeout: configbus.Ptr(time.Hour),
			IgnoreNormalizerJQTimeout: configbus.Ptr(optsTimeout),
			MetricsClusterLabels:      configbus.Ptr(labels),
			PersistResourceHealth:     configbus.Ptr(true),
			ReconciliationJitter:      configbus.Ptr(time.Second),
			ReconciliationTimeout:     configbus.Ptr(time.Minute),
			RepoErrorGracePeriod:      configbus.Ptr(90 * time.Second),
			SelfHealBackoff:           configbus.PtrPtr(backoff),
			SelfHealTimeout:           configbus.Ptr(30 * time.Second),
			ServerSideDiff:            configbus.Ptr(false),
			SyncTimeout:               configbus.Ptr(5 * time.Minute),
		}},
		configbus.NewSettingsManagerProvider(settingsMgr),
		configbus.NewEnvProvider(),
	)

	assertProviderFullyResolved(t, chain)
}

func assertProviderFullyResolved(t *testing.T, p configbus.Provider) {
	t.Helper()
	pt := reflect.TypeOf((*configbus.Provider)(nil)).Elem()
	pv := reflect.ValueOf(p)
	ctx := context.Background()
	for i := 0; i < pt.NumMethod(); i++ {
		m := pt.Method(i)
		switch m.Name {
		case "Subscribe", "Unsubscribe":
			continue
		}
		method := pv.MethodByName(m.Name)
		require.True(t, method.IsValid(), m.Name)
		in := make([]reflect.Value, method.Type().NumIn())
		for j := 0; j < method.Type().NumIn(); j++ {
			argType := method.Type().In(j)
			if argType.String() == "context.Context" {
				in[j] = reflect.ValueOf(ctx)
				continue
			}
			in[j] = reflect.Zero(argType)
		}
		out := method.Call(in)
		require.NotEmpty(t, out, m.Name)
		errVal := out[len(out)-1]
		if errVal.IsNil() {
			continue
		}
		err, ok := errVal.Interface().(error)
		require.True(t, ok, m.Name)
		require.NotErrorIs(t, err, configbus.ErrNotConfigured, "%s leaked ErrNotConfigured: %v", m.Name, err)
	}
}
