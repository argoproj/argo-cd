package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	appclient "github.com/argoproj/argo-cd/v2/event_reporter/application"
	appMocks "github.com/argoproj/argo-cd/v2/event_reporter/application/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	apiclientapppkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1reg "github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/aws/smithy-go/ptr"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/watch"
	"net/http"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	"github.com/argoproj/gitops-engine/pkg/health"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"

	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/codefresh"
	repoApiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

const (
	testNamespace = "default"
)

func TestGetResourceEventPayload(t *testing.T) {
	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		man := "{ \"key\" : \"manifest\" }"

		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := repoApiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
		assert.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		assert.NoError(t, err)

		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.ActualManifest)
	})

	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
			Status: v1alpha1.ApplicationStatus{},
		}
		rs := v1alpha1.ResourceStatus{}
		man := "{ \"key\" : \"manifest\" }"
		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := repoApiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
		assert.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		assert.NoError(t, err)

		assert.Equal(t, "", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "", eventPayload.Source.ActualManifest)
	})
}

func TestGetApplicationLatestRevision(t *testing.T) {
	appRevision := "a-revision"
	history1Revision := "history-revision-1"
	history2Revision := "history-revision-2"

	t.Run("resource revision should be taken from sync.revision", func(t *testing.T) {
		noStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
			},
		}

		revisionResult := getApplicationLatestRevision(&noStatusHistoryAppMock)
		assert.Equal(t, revisionResult, appRevision)

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{},
			},
		}

		revision2Result := getApplicationLatestRevision(&emptyStatusHistoryAppMock)
		assert.Equal(t, revision2Result, appRevision)
	})

	t.Run("resource revision should be taken from latest history.revision", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{
					{
						Revision: history1Revision,
					},
					{
						Revision: history2Revision,
					},
				},
			},
		}

		revisionResult := getApplicationLatestRevision(&appMock)
		assert.Equal(t, revisionResult, history2Revision)
	})
}

func TestGetLatestAppHistoryId(t *testing.T) {
	history1Id := int64(1)
	history2Id := int64(2)

	t.Run("resource revision should be 0", func(t *testing.T) {
		noStatusHistoryAppMock := v1alpha1.Application{}

		idResult := getLatestAppHistoryId(&noStatusHistoryAppMock)
		assert.Equal(t, idResult, int64(0))

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{},
			},
		}

		id2Result := getLatestAppHistoryId(&emptyStatusHistoryAppMock)
		assert.Equal(t, id2Result, int64(0))
	})

	t.Run("resource revision should be taken from latest history.Id", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{
					{
						ID: history1Id,
					},
					{
						ID: history2Id,
					},
				},
			},
		}

		revisionResult := getLatestAppHistoryId(&appMock)
		assert.Equal(t, revisionResult, history2Id)
	})
}

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

type MockcodefreshClient interface {
	Send(ctx context.Context, appName string, event *events.Event) error
}

type MockCodefreshConfig struct {
	BaseURL   string
	AuthToken string
}

type MockCodefreshClient struct {
	cfConfig   *MockCodefreshConfig
	httpClient *http.Client
}

func (cc *MockCodefreshClient) SendEvent(ctx context.Context, appName string, event *events.Event) error {
	return nil
}

func (cc *MockCodefreshClient) SendGraphQL(query codefresh.GraphQLQuery) (*json.RawMessage, error) {
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

	cfClient := &MockCodefreshClient{
		cfConfig: &MockCodefreshConfig{
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
		_ = eventReporter.StreamApplicationEvents(context.Background(), app, "", false, common.LabelKeyAppInstance, argo.TrackingMethodLabel)
	})

}

func TestGetResourceEventPayloadWithoutRevision(t *testing.T) {
	app := v1alpha1.Application{}
	rs := v1alpha1.ResourceStatus{}

	mf := "{ \"key\" : \"manifest\" }"

	actualState := application.ApplicationResourceResponse{
		Manifest: &mf,
	}
	desiredState := repoApiclient.Manifest{
		CompiledManifest: "{ \"key\" : \"manifest\" }",
	}
	appTree := v1alpha1.ApplicationTree{}

	_, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, nil, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
	assert.NoError(t, err)

}

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestAddCommitDetailsToLabels(t *testing.T) {
	revisionMetadata := v1alpha1.RevisionMetadata{
		Author:  "demo usert",
		Date:    metav1.Time{},
		Message: "some message",
	}

	t.Run("set labels when lable object missing", func(t *testing.T) {
		resource := StrToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := addCommitDetailsToLabels(resource, &revisionMetadata)
		labels := result.GetLabels()
		assert.Equal(t, revisionMetadata.Author, labels["app.meta.commit-author"])
		assert.Equal(t, revisionMetadata.Message, labels["app.meta.commit-message"])
	})

	t.Run("set labels when labels present", func(t *testing.T) {
		resource := StrToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    labels:
      link: http://my-grafana.com/pre-generated-link
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := addCommitDetailsToLabels(resource, &revisionMetadata)
		labels := result.GetLabels()
		assert.Equal(t, revisionMetadata.Author, labels["app.meta.commit-author"])
		assert.Equal(t, revisionMetadata.Message, labels["app.meta.commit-message"])
		assert.Equal(t, "http://my-grafana.com/pre-generated-link", result.GetLabels()["link"])
	})
}

func TestSetHealthStatusIfMissing(t *testing.T) {
	resource := appsv1.ResourceStatus{Status: appsv1.SyncStatusCodeSynced}
	setHealthStatusIfMissing(&resource)
	assert.Equal(t, resource.Health.Status, health.HealthStatusHealthy)
}

func TestGetParentAppIdentityWithinNonControllerNs(t *testing.T) {
	resourceTracking := argo.NewResourceTracking()
	annotations := make(map[string]string)
	constrollerNs := "runtime"
	expectedName := "guestbook"
	expectedNamespace := "test-apps"

	guestbookApp := appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedName,
			Namespace: expectedNamespace,
		},
	}
	annotations[common.AnnotationKeyAppInstance] = resourceTracking.BuildAppInstanceValue(argo.AppInstanceValue{
		Name:            "test",
		ApplicationName: guestbookApp.InstanceName(constrollerNs),
		Group:           "group",
		Kind:            "Rollout",
		Namespace:       "test-resources",
	})
	guestbookApp.Annotations = annotations

	res := getParentAppIdentity(&guestbookApp, common.LabelKeyAppInstance, "annotation")

	assert.Equal(t, expectedName, res.name)
	assert.Equal(t, expectedNamespace, res.namespace)
}

func TestGetParentAppIdentityWithinControllerNs(t *testing.T) {
	resourceTracking := argo.NewResourceTracking()
	annotations := make(map[string]string)
	constrollerNs := "runtime"
	expectedName := "guestbook"
	expectedNamespace := ""

	guestbookApp := appsv1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "argoproj.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedName,
			Namespace: constrollerNs,
		},
	}
	annotations[common.AnnotationKeyAppInstance] = resourceTracking.BuildAppInstanceValue(argo.AppInstanceValue{
		Name:            "test",
		ApplicationName: guestbookApp.InstanceName(constrollerNs),
		Group:           "group",
		Kind:            "Rollout",
		Namespace:       "test-resources",
	})
	guestbookApp.Annotations = annotations

	res := getParentAppIdentity(&guestbookApp, common.LabelKeyAppInstance, "annotation")

	assert.Equal(t, expectedName, res.name)
	assert.Equal(t, expectedNamespace, res.namespace)
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
		assert.NoError(t, err)

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
		assert.NoError(t, err)

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
		assert.NoError(t, err)

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

		assert.Equal(t, appEvent.ObjectMeta.Name, manifestApp.ObjectMeta.Name)
		// should set type meta
		assert.Equal(t, appEvent.TypeMeta.Kind, "Application")
		assert.Equal(t, appEvent.TypeMeta.APIVersion, "argoproj.io/v1alpha1")
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
