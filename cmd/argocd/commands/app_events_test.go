package commands

import (
	"bytes"
	stderrors "errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationmocks "github.com/argoproj/argo-cd/v3/pkg/apiclient/application/mocks"
	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func resourceNode(group, kind, namespace, name, uid string) v1alpha1.ResourceNode {
	return v1alpha1.ResourceNode{Group: group, Kind: kind, Namespace: namespace, Name: name, UID: uid}
}

func eventAt(reason string, lastTimestamp time.Time) eventspkg.Event {
	return eventspkg.Event{Reason: reason, LastTimestamp: metav1.NewTime(lastTimestamp)}
}

func TestNewApplicationCommand_EventsRegistered(t *testing.T) {
	command := NewApplicationCommand(&argocdclient.ClientOptions{})

	sub, _, err := command.Find([]string{"events"})
	require.NoError(t, err)
	assert.Equal(t, "events", sub.Name())
}

func TestResourceEventsFilter_validate(t *testing.T) {
	tests := []struct {
		name    string
		filter  resourceEventsFilter
		wantErr bool
	}{
		{name: "no resource lists the application events", filter: resourceEventsFilter{}},
		{name: "kind and name", filter: resourceEventsFilter{kind: "Deployment", name: "guestbook-ui"}},
		{name: "kind, name, group and namespace", filter: resourceEventsFilter{kind: "Deployment", name: "guestbook-ui", group: "apps", groupSet: true, namespace: "default", namespaceSet: true}},
		{name: "kind without name", filter: resourceEventsFilter{kind: "Deployment"}, wantErr: true},
		{name: "name without kind", filter: resourceEventsFilter{name: "guestbook-ui"}, wantErr: true},
		{name: "group without kind and name", filter: resourceEventsFilter{group: "apps", groupSet: true}, wantErr: true},
		{name: "empty group without kind and name", filter: resourceEventsFilter{groupSet: true}, wantErr: true},
		{name: "namespace without kind and name", filter: resourceEventsFilter{namespace: "default", namespaceSet: true}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.validate()
			if tt.wantErr {
				require.ErrorContains(t, err, "--kind and --resource-name must be specified together")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_findResourceNode(t *testing.T) {
	tree := &v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			resourceNode("apps", "Deployment", "default", "guestbook-ui", "uid-deploy"),
			resourceNode("", "Service", "default", "guestbook-ui", "uid-svc"),
			resourceNode("serving.knative.dev", "Service", "default", "guestbook-ui", "uid-ksvc"),
			resourceNode("", "ConfigMap", "ns-a", "config", "uid-cm-a"),
			resourceNode("", "ConfigMap", "ns-b", "config", "uid-cm-b"),
		},
		OrphanedNodes: []v1alpha1.ResourceNode{
			resourceNode("", "Secret", "default", "orphan", "uid-orphan"),
		},
	}

	t.Run("selects the resource by kind and name", func(t *testing.T) {
		node, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Deployment", name: "guestbook-ui"})
		require.NoError(t, err)
		assert.Equal(t, "uid-deploy", node.UID)
	})

	t.Run("filters by group", func(t *testing.T) {
		node, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Service", name: "guestbook-ui", group: "serving.knative.dev", groupSet: true})
		require.NoError(t, err)
		assert.Equal(t, "uid-ksvc", node.UID)

		_, err = findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Deployment", name: "guestbook-ui", group: "badgroup", groupSet: true})
		require.ErrorContains(t, err, "resource Deployment/guestbook-ui not found in application my-app")
	})

	t.Run("selects the core group with an empty group", func(t *testing.T) {
		node, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Service", name: "guestbook-ui", groupSet: true})
		require.NoError(t, err)
		assert.Equal(t, "uid-svc", node.UID)
	})

	t.Run("filters by namespace", func(t *testing.T) {
		node, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "ConfigMap", name: "config", namespace: "ns-b", namespaceSet: true})
		require.NoError(t, err)
		assert.Equal(t, "uid-cm-b", node.UID)
	})

	t.Run("includes orphaned resources", func(t *testing.T) {
		node, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Secret", name: "orphan"})
		require.NoError(t, err)
		assert.Equal(t, "uid-orphan", node.UID)
	})

	t.Run("fails when no resource matches", func(t *testing.T) {
		_, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Service", name: "missing"})
		require.ErrorContains(t, err, "resource Service/missing not found in application my-app")
	})

	t.Run("fails when several resources match", func(t *testing.T) {
		_, err := findResourceNode(tree, "my-app", resourceEventsFilter{kind: "Service", name: "guestbook-ui"})
		require.ErrorContains(t, err, "2 resources match Service/guestbook-ui in application my-app, use --group or --namespace to select one")

		_, err = findResourceNode(tree, "my-app", resourceEventsFilter{kind: "ConfigMap", name: "config"})
		require.ErrorContains(t, err, "2 resources match ConfigMap/config in application my-app, use --group or --namespace to select one")
	})
}

func Test_listApplicationEvents(t *testing.T) {
	appName, appNs, project := "my-app", "argocd", "default"
	base := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)

	t.Run("lists the application events without reading the resource tree", func(t *testing.T) {
		appIf := applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ListResourceEvents(mock.Anything, &applicationpkg.ApplicationResourceEventsQuery{Name: &appName, AppNamespace: &appNs, Project: &project}).
			Return(&eventspkg.EventList{Items: []eventspkg.Event{eventAt("OperationStarted", base)}}, nil).Once()

		events, err := listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "OperationStarted", events[0].Reason)
	})

	t.Run("lists the events of the selected resource by its UID", func(t *testing.T) {
		resName, resNs, resUID := "guestbook-ui", "default", "uid-deploy"
		appIf := applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ResourceTree(mock.Anything, &applicationpkg.ResourcesQuery{ApplicationName: &appName, AppNamespace: &appNs, Project: &project}).
			Return(&v1alpha1.ApplicationTree{Nodes: []v1alpha1.ResourceNode{resourceNode("apps", "Deployment", resNs, resName, resUID)}}, nil).Once()
		appIf.EXPECT().ListResourceEvents(mock.Anything, &applicationpkg.ApplicationResourceEventsQuery{
			Name:              &appName,
			AppNamespace:      &appNs,
			Project:           &project,
			ResourceName:      &resName,
			ResourceNamespace: &resNs,
			ResourceUID:       &resUID,
		}).Return(&eventspkg.EventList{Items: []eventspkg.Event{eventAt("ScalingReplicaSet", base)}}, nil).Once()

		events, err := listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{kind: "Deployment", name: resName})
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, "ScalingReplicaSet", events[0].Reason)
	})

	t.Run("does not list events when the resource is not in the application", func(t *testing.T) {
		appIf := applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ResourceTree(mock.Anything, mock.Anything).Return(&v1alpha1.ApplicationTree{}, nil).Once()

		_, err := listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{kind: "Deployment", name: "missing"})
		require.ErrorContains(t, err, "resource Deployment/missing not found in application my-app")
	})

	t.Run("sorts events from the oldest to the most recent", func(t *testing.T) {
		appIf := applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ListResourceEvents(mock.Anything, mock.Anything).Return(&eventspkg.EventList{Items: []eventspkg.Event{
			eventAt("third", base.Add(2*time.Minute)),
			eventAt("first", base),
			eventAt("second", base.Add(time.Minute)),
		}}, nil).Once()

		events, err := listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{})
		require.NoError(t, err)
		reasons := make([]string, 0, len(events))
		for _, e := range events {
			reasons = append(reasons, e.Reason)
		}
		assert.Equal(t, []string{"first", "second", "third"}, reasons)
	})

	t.Run("returns the API errors", func(t *testing.T) {
		expectedErr := stderrors.New("permission denied")
		appIf := applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ListResourceEvents(mock.Anything, mock.Anything).Return(nil, expectedErr).Once()
		_, err := listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{})
		require.ErrorIs(t, err, expectedErr)

		appIf = applicationmocks.NewApplicationServiceClient(t)
		appIf.EXPECT().ResourceTree(mock.Anything, mock.Anything).Return(nil, expectedErr).Once()
		_, err = listApplicationEvents(t.Context(), appIf, appName, appNs, project, resourceEventsFilter{kind: "Deployment", name: "guestbook-ui"})
		require.ErrorIs(t, err, expectedErr)
	})
}

func Test_printEventsTable(t *testing.T) {
	base := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	events := []eventspkg.Event{
		{
			Type:           "Normal",
			Reason:         "ScalingReplicaSet",
			InvolvedObject: eventspkg.ObjectReference{Kind: "Deployment", Name: "guestbook-ui"},
			Count:          3,
			Message:        "Scaled up replica set guestbook-ui-85985d774c from 0 to 1",
			LastTimestamp:  metav1.NewTime(base),
		},
		{
			Type:           "Warning",
			Reason:         "BackOff",
			InvolvedObject: eventspkg.ObjectReference{Kind: "Pod", Name: "guestbook-ui-85985d774c-abcde"},
			Count:          1,
			Message:        "Back-off restarting\nfailed\tcontainer",
			LastTimestamp:  metav1.NewTime(base.Add(time.Minute)),
		},
		{
			Type:           "Normal",
			Reason:         "NoTime",
			InvolvedObject: eventspkg.ObjectReference{Kind: "Application", Name: "my-app"},
		},
	}

	var out bytes.Buffer
	printEventsTable(&out, events)

	expected := "LAST SEEN             TYPE     REASON             OBJECT                             COUNT  MESSAGE\n" +
		"2026-11-19T10:00:00Z  Normal   ScalingReplicaSet  Deployment/guestbook-ui            3      Scaled up replica set guestbook-ui-85985d774c from 0 to 1\n" +
		"2026-11-19T10:01:00Z  Warning  BackOff            Pod/guestbook-ui-85985d774c-abcde  1      Back-off restarting failed container\n" +
		"-                     Normal   NoTime             Application/my-app                 0      \n"
	assert.Equal(t, expected, out.String())
}
