package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/watch"

	appclient "github.com/argoproj/argo-cd/v2/event_reporter/application"
	appMocks "github.com/argoproj/argo-cd/v2/event_reporter/application/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	apiclientapppkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1reg "github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/util/io"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"

	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/khulnasoft"
)

const (
	testNamespace = "default"
)

func newAppLister(objects ...runtime.Object) applisters.ApplicationLister {
	fakeAppsClientset := fakeapps.NewSimpleClientset(objects...)
	factory := appinformer.NewSharedInformerFactoryWithOptions(fakeAppsClientset, 0, appinformer.WithNamespace(""), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
	appsInformer := factory.Argoproj().V1alpha1().Applications()
	for _, obj := range objects {
		switch obj.(type) {
		case *appsv1.Application:
			_ = appsInformer.Informer().GetStore().Add(obj)
		}
	}
	appLister := appsInformer.Lister()
	return appLister
}

type MockkhulnasoftClient interface {
	Send(ctx context.Context, appName string, event *events.Event) error
}

type MockKhulnasoftConfig struct {
	BaseURL   string
	AuthToken string
}

type MockKhulnasoftClient struct {
	cfConfig   *MockKhulnasoftConfig
	httpClient *http.Client
}

func (cc *MockKhulnasoftClient) SendEvent(ctx context.Context, appName string, event *events.Event) error {
	return nil
}

func (cc *MockKhulnasoftClient) SendGraphQL(query khulnasoft.GraphQLQuery) (*json.RawMessage, error) {
	return nil, nil
}

func fakeAppServiceClient() apiclientapppkg.ApplicationServiceClient {
	closer, applicationServiceClient, _ := apiclient.NewClientOrDie(&apiclient.ClientOptions{
		ServerAddr: "site.com",
	}).NewApplicationClient()
	defer io.Close(closer)

	return applicationServiceClient
}

func fakeReporter(customAppServiceClient appclient.ApplicationClient) *applicationEventReporter {
	guestbookApp := &appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guestbook",
			Namespace: testNamespace,
		},
		Spec: appsv1.ApplicationSpec{
			Project: "default",
			Source: &appsv1.ApplicationSource{
				RepoURL:        "https://test",
				TargetRevision: "HEAD",
				Helm: &appsv1.ApplicationSourceHelm{
					ValueFiles: []string{"values.yaml"},
				},
			},
		},
		Status: appsv1.ApplicationStatus{
			History: appsv1.RevisionHistories{
				{
					Revision: "abcdef123567",
					Source: appsv1.ApplicationSource{
						RepoURL:        "https://test",
						TargetRevision: "HEAD",
						Helm: &appsv1.ApplicationSourceHelm{
							ValueFiles: []string{"values-old.yaml"},
						},
					},
				},
			},
		},
	}

	appLister := newAppLister(guestbookApp)

	cache := servercache.NewCache(
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		),
		1*time.Minute,
		1*time.Minute,
		1*time.Minute,
	)

	cfClient := &MockKhulnasoftClient{
		cfConfig: &MockKhulnasoftConfig{
			BaseURL:   "",
			AuthToken: "",
		},
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	metricsServ := metrics.NewMetricsServer("", 8099)

	return &applicationEventReporter{
		cache,
		cfClient,
		appLister,
		customAppServiceClient,
		metricsServ,
	}
}

func TestShouldSendEvent(t *testing.T) {
	eventReporter := fakeReporter(fakeAppServiceClient())
	t.Run("should send because cache is missing", func(t *testing.T) {
		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		res := eventReporter.shouldSendResourceEvent(app, rs)
		assert.True(t, res)
	})

	t.Run("should not send - same entities", func(t *testing.T) {
		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		_ = eventReporter.cache.SetLastResourceEvent(app, rs, time.Minute, "")

		res := eventReporter.shouldSendResourceEvent(app, rs)
		assert.False(t, res)
	})

	t.Run("should send - different entities", func(t *testing.T) {
		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		_ = eventReporter.cache.SetLastResourceEvent(app, rs, time.Minute, "")

		rs.Status = v1alpha1.SyncStatusCodeOutOfSync

		res := eventReporter.shouldSendResourceEvent(app, rs)
		assert.True(t, res)
	})
}

type MockEventing_StartEventSourceServer struct {
	grpc.ServerStream
}

var result func(*events.Event) error

func (m *MockEventing_StartEventSourceServer) Send(event *events.Event) error {
	return result(event)
}

func TestStreamApplicationEvent(t *testing.T) {
	eventReporter := fakeReporter(fakeAppServiceClient())
	t.Run("root application", func(t *testing.T) {
		app := &v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       "Application",
			},
		}

		result = func(event *events.Event) error {
			var payload events.EventPayload
			_ = json.Unmarshal(event.Payload, &payload)

			var actualApp v1alpha1.Application
			_ = json.Unmarshal([]byte(payload.Source.ActualManifest), &actualApp)
			assert.Equal(t, *app, actualApp)
			return nil
		}
		_ = eventReporter.StreamApplicationEvents(context.Background(), app, "", false, getMockedArgoTrackingMetadata())
	})
}

func TestShouldSendApplicationEvent(t *testing.T) {
	eventReporter := fakeReporter(fakeAppServiceClient())

	t.Run("should send because cache is missing", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"sdsds": "sdsd",
				},
			},
		}

		shouldSend, _ := eventReporter.ShouldSendApplicationEvent(&v1alpha1.ApplicationWatchEvent{
			Type:        watch.Modified,
			Application: app,
		})
		assert.True(t, shouldSend)
	})

	t.Run("should send because labels changed", func(t *testing.T) {
		appCache := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"data": "old value",
				},
			},
		}

		err := eventReporter.cache.SetLastApplicationEvent(&appCache, time.Second*5)
		require.NoError(t, err)

		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"data": "new value",
				},
			},
		}

		shouldSend, _ := eventReporter.ShouldSendApplicationEvent(&v1alpha1.ApplicationWatchEvent{
			Type:        watch.Modified,
			Application: app,
		})
		assert.True(t, shouldSend)
	})

	t.Run("should send because annotations changed", func(t *testing.T) {
		appCache := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"data": "old value",
				},
			},
		}

		err := eventReporter.cache.SetLastApplicationEvent(&appCache, time.Second*5)
		require.NoError(t, err)

		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"data": "new value",
				},
			},
		}

		shouldSend, _ := eventReporter.ShouldSendApplicationEvent(&v1alpha1.ApplicationWatchEvent{
			Type:        watch.Modified,
			Application: app,
		})
		assert.True(t, shouldSend)
	})

	t.Run("should ignore some changed metadata fields", func(t *testing.T) {
		appCache := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "1",
				Generation:      1,
				GenerateName:    "first",
				ManagedFields:   []metav1.ManagedFieldsEntry{},
				Annotations: map[string]string{
					"kubectl.kubernetes.io/last-applied-configuration": "first",
				},
			},
		}

		err := eventReporter.cache.SetLastApplicationEvent(&appCache, time.Second*5)
		require.NoError(t, err)

		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "2",
				Generation:      2,
				GenerateName:    "changed",
				ManagedFields:   []metav1.ManagedFieldsEntry{{Manager: "changed"}},
				Annotations: map[string]string{
					"kubectl.kubernetes.io/last-applied-configuration": "changed",
				},
			},
		}

		shouldSend, _ := eventReporter.ShouldSendApplicationEvent(&v1alpha1.ApplicationWatchEvent{
			Type:        watch.Modified,
			Application: app,
		})
		assert.False(t, shouldSend)
	})
}

func TestGetResourceActualState(t *testing.T) {
	ctx := context.Background()
	// Create a new logrus entry (assuming you have a configured logger)
	logEntry := logrus.NewEntry(logrus.StandardLogger())

	t.Run("should use existing app event for application", func(t *testing.T) {
		eventReporter := fakeReporter(fakeAppServiceClient())

		appEvent := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app",
				Namespace: "test-app-ns",
			},
		}

		parentApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-parent-app",
				Namespace: "test-app-ns",
			},
			Spec: appsv1.ApplicationSpec{
				Project: appsv1.DefaultAppProjectName,
			},
		}
		rs := v1alpha1.ResourceStatus{
			Group:   v1alpha1.ApplicationSchemaGroupVersionKind.Group,
			Version: v1alpha1.ApplicationSchemaGroupVersionKind.Version,
			Kind:    v1alpha1.ApplicationSchemaGroupVersionKind.Kind,
		}

		res, err := eventReporter.getResourceActualState(ctx, logEntry, metrics.MetricAppEventType, rs, &parentApp, &appEvent)
		require.NoError(t, err)

		var manifestApp v1alpha1.Application
		if err := json.Unmarshal([]byte(*res.Manifest), &manifestApp); err != nil {
			t.Fatalf("failed to unmarshal manifest: %v", err)
		}

		assert.Equal(t, manifestApp.ObjectMeta.Name, appEvent.ObjectMeta.Name)
		// should set type meta
		assert.Equal(t, "Application", appEvent.TypeMeta.Kind)
		assert.Equal(t, "argoproj.io/v1alpha1", appEvent.TypeMeta.APIVersion)
	})

	t.Run("should get resource actual state for non-app resources", func(t *testing.T) {
		expectedAppSetName := "test-app-set"
		appSetCurrentActualState := v1alpha1.ApplicationSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       appv1reg.ApplicationSetKind,
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      expectedAppSetName,
				Namespace: "test-app-ns",
			},
		}
		manifestBytes, err := json.Marshal(appSetCurrentActualState)

		if len(manifestBytes) == 0 && err != nil {
			t.Fatalf("failed to Marshal manifest: %v", err)
		}

		manifest := string(manifestBytes)

		appServiceClient := &appMocks.ApplicationClient{}
		appServiceClient.On("GetResource", mock.Anything, mock.Anything).Return(&application.ApplicationResourceResponse{Manifest: &manifest}, nil)

		eventReporter := fakeReporter(appServiceClient)

		parentApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-parent-app",
				Namespace: "test-app-ns",
			},
			Spec: appsv1.ApplicationSpec{
				Project: appsv1.DefaultAppProjectName,
			},
		}
		rs := v1alpha1.ResourceStatus{
			Group:   v1alpha1.ApplicationSchemaGroupVersionKind.Group,
			Version: v1alpha1.ApplicationSchemaGroupVersionKind.Version,
			Kind:    "ApplicationSet",
		}

		res, err := eventReporter.getResourceActualState(ctx, logEntry, metrics.MetricAppEventType, rs, &parentApp, nil)
		require.NoError(t, err)

		var manifestApp v1alpha1.Application
		if err := json.Unmarshal([]byte(*res.Manifest), &manifestApp); err != nil {
			t.Fatalf("failed to unmarshal manifest: %v", err)
		}

		assert.Equal(t, expectedAppSetName, manifestApp.ObjectMeta.Name)
		assert.Equal(t, appv1reg.ApplicationSetKind, manifestApp.TypeMeta.Kind)
	})

	t.Run("should return empty manifest for not found resources", func(t *testing.T) {
		appServiceClient := &appMocks.ApplicationClient{}
		appServiceClient.On("GetResource", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("not found resource"))

		eventReporter := fakeReporter(appServiceClient)

		parentApp := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-parent-app",
				Namespace: "test-app-ns",
			},
			Spec: appsv1.ApplicationSpec{
				Project: appsv1.DefaultAppProjectName,
			},
		}
		rs := v1alpha1.ResourceStatus{
			Group:   v1alpha1.ApplicationSchemaGroupVersionKind.Group,
			Version: v1alpha1.ApplicationSchemaGroupVersionKind.Version,
			Kind:    "ApplicationSet",
		}

		res, err := eventReporter.getResourceActualState(ctx, logEntry, metrics.MetricAppEventType, rs, &parentApp, nil)
		require.NoError(t, err)
		assert.Equal(t, "", ptr.ToString(res.Manifest))
	})
}
