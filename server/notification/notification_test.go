package notification

import (
	"context"
	"os"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
	"github.com/argoproj/argo-cd/v2/util/notification/k8s"
	"github.com/argoproj/argo-cd/v2/util/notification/settings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/kubectl/pkg/scheme"
)

const testNamespace = "default"

func TestNotificationServer(t *testing.T) {
	// catalogPath := path.Join(paths[1], "config", "notifications-catalog")
	b, err := os.ReadFile("../../notifications_catalog/install.yaml")
	require.NoError(t, err)

	cm := &corev1.ConfigMap{}
	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(b, nil, cm)
	require.NoError(t, err)
	cm.Namespace = testNamespace

	kubeclientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "argocd-notifications-cm",
		},
		Data: map[string]string{
			"service.webhook.test": "url: https://test.example.com",
			"template.app-created": "email:\n  subject: Application {{.app.metadata.name}} has been created.\nmessage: Application {{.app.metadata.name}} has been created.\nteams:\n  title: Application {{.app.metadata.name}} has been created.\n",
			"trigger.on-created":   "- description: Application is created.\n  oncePer: app.metadata.name\n  send:\n  - app-created\n  when: \"true\"\n",
		},
	},
		&corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      "argocd-notifications-secret",
				Namespace: testNamespace,
			},
			Data: map[string][]byte{},
		})

	ctx := context.Background()
	secretInformer := k8s.NewSecretInformer(kubeclientset, testNamespace, "argocd-notifications-secret")
	configMapInformer := k8s.NewConfigMapInformer(kubeclientset, testNamespace, "argocd-notifications-cm")
	go secretInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), secretInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}
	go configMapInformer.Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), configMapInformer.HasSynced) {
		panic("Timed out waiting for caches to sync")
	}
	mockRepoClient := &mocks.Clientset{RepoServerServiceClient: &mocks.RepoServerServiceClient{}}

	argocdService, err := service.NewArgoCDService(kubeclientset, testNamespace, mockRepoClient)
	require.NoError(t, err)
	defer argocdService.Close()
	apiFactory := api.NewFactory(settings.GetFactorySettings(argocdService, "argocd-notifications-secret", "argocd-notifications-cm", false), testNamespace, secretInformer, configMapInformer)

	t.Run("TestListServices", func(t *testing.T) {
		server := NewServer(apiFactory)
		services, err := server.ListServices(ctx, &notification.ServicesListRequest{})
		require.NoError(t, err)
		assert.Len(t, services.Items, 1)
		assert.Equal(t, services.Items[0].Name, ptr.To("test"))
		assert.NotEmpty(t, services.Items[0])
	})
	t.Run("TestListTriggers", func(t *testing.T) {
		server := NewServer(apiFactory)
		triggers, err := server.ListTriggers(ctx, &notification.TriggersListRequest{})
		require.NoError(t, err)
		assert.Len(t, triggers.Items, 1)
		assert.Equal(t, triggers.Items[0].Name, ptr.To("on-created"))
		assert.NotEmpty(t, triggers.Items[0])
	})
	t.Run("TestListTemplates", func(t *testing.T) {
		server := NewServer(apiFactory)
		templates, err := server.ListTemplates(ctx, &notification.TemplatesListRequest{})
		require.NoError(t, err)
		assert.Len(t, templates.Items, 1)
		assert.Equal(t, templates.Items[0].Name, ptr.To("app-created"))
		assert.NotEmpty(t, templates.Items[0])
	})
}
