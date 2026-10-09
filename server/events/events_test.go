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
	t.Parallel()
	t.Run("nil EventList returns empty list", func(t *testing.T) {
		t.Parallel()
		result := K8sEventListToAPIEventList(nil)
		require.NotNil(t, result)
		assert.Empty(t, result.Items)
	})

	t.Run("empty EventList returns empty items", func(t *testing.T) {
		t.Parallel()
		result := K8sEventListToAPIEventList(&corev1.EventList{Items: []corev1.Event{}})
		require.NotNil(t, result)
		assert.Empty(t, result.Items)
	})

	t.Run("EventList with events converts each field", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

	t.Run("series backfills legacy count and lastTimestamp", func(t *testing.T) {
		t.Parallel()
		legacyTime := metav1.NewTime(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
		observedTime := metav1.NewMicroTime(time.Date(2026, 10, 2, 8, 30, 0, 0, time.UTC))
		input := &corev1.EventList{
			Items: []corev1.Event{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "legacy", Namespace: "default"},
					Count:         5,
					LastTimestamp: legacyTime,
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "series-only", Namespace: "default"},
					Series:     &corev1.EventSeries{Count: 7, LastObservedTime: observedTime},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "singleton", Namespace: "default"},
				},
			},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		require.Len(t, result.Items, 3)

		legacy := result.Items[0]
		assert.Equal(t, int32(5), legacy.Count)
		assert.Equal(t, legacyTime, legacy.LastTimestamp)

		seriesOnly := result.Items[1]
		assert.Equal(t, int32(7), seriesOnly.Count)
		assert.Equal(t, metav1.NewTime(observedTime.Time), seriesOnly.LastTimestamp)
		require.NotNil(t, seriesOnly.Series)
		assert.Equal(t, int32(7), seriesOnly.Series.Count)

		singleton := result.Items[2]
		assert.Equal(t, int32(0), singleton.Count)
		assert.True(t, singleton.LastTimestamp.IsZero())
	})

	t.Run("series wins over legacy fields, like kubectl", func(t *testing.T) {
		t.Parallel()
		legacyTime := metav1.NewTime(time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
		observedTime := metav1.NewMicroTime(time.Date(2026, 10, 2, 8, 30, 0, 0, time.UTC))
		input := &corev1.EventList{
			Items: []corev1.Event{
				{
					ObjectMeta:    metav1.ObjectMeta{Name: "mixed", Namespace: "default"},
					Count:         5,
					LastTimestamp: legacyTime,
					Series:        &corev1.EventSeries{Count: 9, LastObservedTime: observedTime},
				},
			},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		require.Len(t, result.Items, 1)
		assert.Equal(t, int32(9), result.Items[0].Count)
		assert.Equal(t, metav1.NewTime(observedTime.Time), result.Items[0].LastTimestamp)
	})

	t.Run("singleton events.k8s.io event counts once with eventTime timestamps", func(t *testing.T) {
		t.Parallel()
		eventTime := metav1.NewMicroTime(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC))
		input := &corev1.EventList{
			Items: []corev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "new-api-singleton", Namespace: "default"},
					EventTime:  eventTime,
				},
			},
		}

		result := K8sEventListToAPIEventList(input)
		require.NotNil(t, result)
		require.Len(t, result.Items, 1)
		got := result.Items[0]
		assert.Equal(t, int32(1), got.Count)
		assert.Equal(t, metav1.NewTime(eventTime.Time), got.FirstTimestamp)
		assert.Equal(t, metav1.NewTime(eventTime.Time), got.LastTimestamp)
	})

	t.Run("optional pointer fields are converted", func(t *testing.T) {
		t.Parallel()
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
