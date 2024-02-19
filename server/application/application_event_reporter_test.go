package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	apps "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformers "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/test"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

func TestGetResourceEventPayload(t *testing.T) {
	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}
		es := events.EventSource{}

		man := "{ \"key\" : \"manifest\" }"

		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := apiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &es, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &apiclient.ApplicationVersions{})
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
		es := events.EventSource{}
		man := "{ \"key\" : \"manifest\" }"
		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := apiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &es, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &apiclient.ApplicationVersions{})
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

func fakeServer() *Server {
	cm := test.NewFakeConfigMap()
	secret := test.NewFakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm, secret)
	appClientSet := apps.NewSimpleClientset()

	appInformer := appinformers.NewApplicationInformer(appClientSet, "", time.Minute, cache.Indexers{})

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
	// _, _ := test.NewInMemoryRedis()

	cache := servercache.NewCache(
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
			1*time.Minute,
		),
		1*time.Minute,
		1*time.Minute,
		1*time.Minute,
	)

	server, _ := NewServer(test.FakeArgoCDNamespace, kubeclientset, appClientSet, appLister, appInformer, nil, nil, cache, nil, nil, nil, nil, nil, nil, nil)
	return server.(*Server)
}

func TestShouldSendEvent(t *testing.T) {
	serverInstance := fakeServer()
	t.Run("should send because cache is missing", func(t *testing.T) {
		eventReporter := applicationEventReporter{
			server: serverInstance,
		}

		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		res := eventReporter.shouldSendResourceEvent(app, rs)
		assert.True(t, res)
	})

	t.Run("should not send - same entities", func(t *testing.T) {
		eventReporter := applicationEventReporter{
			server: serverInstance,
		}

		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		_ = eventReporter.server.cache.SetLastResourceEvent(app, rs, time.Minute, "")

		res := eventReporter.shouldSendResourceEvent(app, rs)
		assert.False(t, res)
	})

	t.Run("should send - different entities", func(t *testing.T) {
		eventReporter := applicationEventReporter{
			server: serverInstance,
		}

		app := &v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		_ = eventReporter.server.cache.SetLastResourceEvent(app, rs, time.Minute, "")

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
	serverInstance := fakeServer()
	t.Run("root application", func(t *testing.T) {
		eventReporter := applicationEventReporter{
			server: serverInstance,
		}

		app := &v1alpha1.Application{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       "Application",
			},
		}
		name := "name"

		result = func(event *events.Event) error {
			var payload events.EventPayload
			_ = json.Unmarshal(event.Payload, &payload)

			var actualApp v1alpha1.Application
			_ = json.Unmarshal([]byte(payload.Source.ActualManifest), &actualApp)
			assert.Equal(t, *app, actualApp)
			return nil
		}

		_ = eventReporter.streamApplicationEvents(context.Background(), app, &events.EventSource{Name: &name}, &MockEventing_StartEventSourceServer{}, "", false, common.LabelKeyAppInstance, argo.TrackingMethodLabel)
	})

}

func TestGetResourceEventPayloadWithoutRevision(t *testing.T) {
	app := v1alpha1.Application{}
	rs := v1alpha1.ResourceStatus{}
	es := events.EventSource{}

	mf := "{ \"key\" : \"manifest\" }"

	actualState := application.ApplicationResourceResponse{
		Manifest: &mf,
	}
	desiredState := apiclient.Manifest{
		CompiledManifest: "{ \"key\" : \"manifest\" }",
	}
	appTree := v1alpha1.ApplicationTree{}

	_, err := getResourceEventPayload(&app, &rs, &es, &actualState, &desiredState, &appTree, true, "", nil, nil, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &apiclient.ApplicationVersions{})
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
