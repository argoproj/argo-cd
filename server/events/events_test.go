package events

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEventListToStruct(t *testing.T) {
	t.Run("nil EventList returns empty struct", func(t *testing.T) {
		result, err := EventListToStruct(nil)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify the structure has items and metadata fields
		assert.NotNil(t, result.Fields["items"])
		assert.NotNil(t, result.Fields["metadata"])

		// Items should be an empty list
		items := result.Fields["items"].GetListValue()
		assert.NotNil(t, items)
		assert.Empty(t, items.Values)
	})

	t.Run("empty EventList returns empty items", func(t *testing.T) {
		eventList := &corev1.EventList{
			Items: []corev1.Event{},
		}

		result, err := EventListToStruct(eventList)
		require.NoError(t, err)
		require.NotNil(t, result)

		items := result.Fields["items"].GetListValue()
		assert.NotNil(t, items)
		assert.Empty(t, items.Values)
	})

	t.Run("EventList with events converts correctly", func(t *testing.T) {
		eventTime := metav1.NewTime(time.Now())
		eventList := &corev1.EventList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "EventList",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{
				ResourceVersion: "12345",
			},
			Items: []corev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-1",
						Namespace: "default",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "test-pod",
						Namespace: "default",
					},
					Reason:  "Created",
					Message: "Pod created successfully",
					Type:    corev1.EventTypeNormal,
					FirstTimestamp: eventTime,
					LastTimestamp:  eventTime,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-2",
						Namespace: "default",
					},
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Name:      "test-pod",
						Namespace: "default",
					},
					Reason:  "Started",
					Message: "Container started",
					Type:    corev1.EventTypeNormal,
					FirstTimestamp: eventTime,
					LastTimestamp:  eventTime,
				},
			},
		}

		result, err := EventListToStruct(eventList)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify items
		items := result.Fields["items"].GetListValue()
		assert.NotNil(t, items)
		assert.Len(t, items.Values, 2)

		// Verify first event
		firstEvent := items.Values[0].GetStructValue()
		assert.NotNil(t, firstEvent)

		metadata := firstEvent.Fields["metadata"].GetStructValue()
		assert.Equal(t, "test-event-1", metadata.Fields["name"].GetStringValue())
		assert.Equal(t, "default", metadata.Fields["namespace"].GetStringValue())

		assert.Equal(t, "Created", firstEvent.Fields["reason"].GetStringValue())
		assert.Equal(t, "Pod created successfully", firstEvent.Fields["message"].GetStringValue())
		assert.Equal(t, "Normal", firstEvent.Fields["type"].GetStringValue())

		// Verify involvedObject
		involvedObject := firstEvent.Fields["involvedObject"].GetStructValue()
		assert.Equal(t, "Pod", involvedObject.Fields["kind"].GetStringValue())
		assert.Equal(t, "test-pod", involvedObject.Fields["name"].GetStringValue())
	})

	t.Run("EventList metadata is preserved", func(t *testing.T) {
		eventList := &corev1.EventList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "EventList",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{
				ResourceVersion: "67890",
				Continue:        "continue-token",
			},
			Items: []corev1.Event{},
		}

		result, err := EventListToStruct(eventList)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify metadata
		listMetadata := result.Fields["metadata"].GetStructValue()
		assert.Equal(t, "67890", listMetadata.Fields["resourceVersion"].GetStringValue())
		assert.Equal(t, "continue-token", listMetadata.Fields["continue"].GetStringValue())

		// Verify kind and apiVersion
		assert.Equal(t, "EventList", result.Fields["kind"].GetStringValue())
		assert.Equal(t, "v1", result.Fields["apiVersion"].GetStringValue())
	})
}
