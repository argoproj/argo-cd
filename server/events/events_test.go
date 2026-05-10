package events

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestK8sEventListToAPIEventList(t *testing.T) {
	t.Run("nil EventList returns empty list", func(t *testing.T) {
		result := K8sEventListToAPIEventList(nil)
		require.NotNil(t, result)
		assert.Empty(t, result.Items)
	})

	t.Run("empty EventList returns empty items", func(t *testing.T) {
		result := K8sEventListToAPIEventList(&corev1.EventList{Items: []corev1.Event{}})
		require.NotNil(t, result)
		assert.Empty(t, result.Items)
	})

	t.Run("EventList with events converts each field", func(t *testing.T) {
		eventTime := metav1.NewTime(time.Now())
		input := &corev1.EventList{
			ListMeta: metav1.ListMeta{ResourceVersion: "12345"},
			Items: []corev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-event-1", Namespace: "default"},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "test-pod",
						Namespace: "default",
						UID:       "abc-123",
					},
					Reason:         "Created",
					Message:        "Pod created successfully",
					Type:           corev1.EventTypeNormal,
					FirstTimestamp: eventTime,
					LastTimestamp:  eventTime,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "test-event-2", Namespace: "default"},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "test-pod",
						Namespace: "default",
					},
					Reason:         "Started",
					Message:        "Container started",
					Type:           corev1.EventTypeNormal,
					FirstTimestamp: eventTime,
					LastTimestamp:  eventTime,
				},
			},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		assert.Equal(t, "12345", result.Metadata.ResourceVersion)
		require.Len(t, result.Items, 2)

		first := result.Items[0]
		assert.Equal(t, "test-event-1", first.Metadata.Name)
		assert.Equal(t, "default", first.Metadata.Namespace)
		assert.Equal(t, "Created", first.Reason)
		assert.Equal(t, "Pod created successfully", first.Message)
		assert.Equal(t, "Normal", first.Type)
		assert.Equal(t, "Pod", first.InvolvedObject.Kind)
		assert.Equal(t, "test-pod", first.InvolvedObject.Name)
		assert.Equal(t, "abc-123", first.InvolvedObject.UID)
	})

	t.Run("EventList metadata is preserved", func(t *testing.T) {
		input := &corev1.EventList{
			ListMeta: metav1.ListMeta{
				ResourceVersion: "67890",
				Continue:        "continue-token",
			},
			Items: []corev1.Event{},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		assert.Equal(t, "67890", result.Metadata.ResourceVersion)
		assert.Equal(t, "continue-token", result.Metadata.Continue)
	})

	t.Run("optional pointer fields are converted", func(t *testing.T) {
		input := &corev1.EventList{
			Items: []corev1.Event{
				{
					ObjectMeta:          metav1.ObjectMeta{Name: "evt", Namespace: "default"},
					Reason:              "Updated",
					Series:              &corev1.EventSeries{Count: 3},
					Related:             &corev1.ObjectReference{Kind: "Deployment", Name: "dep"},
					ReportingController: "argocd-application-controller",
					ReportingInstance:   "argocd-0",
				},
			},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		require.Len(t, result.Items, 1)

		got := result.Items[0]
		require.NotNil(t, got.Series)
		assert.Equal(t, int32(3), got.Series.Count)
		require.NotNil(t, got.Related)
		assert.Equal(t, "Deployment", got.Related.Kind)
		assert.Equal(t, "dep", got.Related.Name)
		assert.Equal(t, "argocd-application-controller", got.ReportingController)
		assert.Equal(t, "argocd-0", got.ReportingInstance)
	})
}
