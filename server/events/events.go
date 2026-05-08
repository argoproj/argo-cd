package events

import (
	corev1 "k8s.io/api/core/v1"

	eventspb "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
)

// K8sEventListToAPIEventList converts a Kubernetes EventList into the typed
// Argo CD events API representation. A nil input is mapped to an empty list so
// callers can return the result directly without a nil-check.
func K8sEventListToAPIEventList(in *corev1.EventList) *eventspb.EventList {
	if in == nil {
		return &eventspb.EventList{}
	}

	out := &eventspb.EventList{
		Metadata: in.ListMeta,
		Items:    make([]eventspb.Event, 0, len(in.Items)),
	}
	for i := range in.Items {
		out.Items = append(out.Items, k8sEventToAPIEvent(&in.Items[i]))
	}
	return out
}

func k8sEventToAPIEvent(in *corev1.Event) eventspb.Event {
	return eventspb.Event{
		Metadata:           in.ObjectMeta,
		InvolvedObject:     k8sObjectReferenceToAPIObjectReference(in.InvolvedObject),
		Reason:             in.Reason,
		Message:            in.Message,
		Source:             k8sEventSourceToAPIEventSource(in.Source),
		FirstTimestamp:     in.FirstTimestamp,
		LastTimestamp:      in.LastTimestamp,
		Count:              in.Count,
		Type:               in.Type,
		EventTime:          in.EventTime,
		Series:             k8sEventSeriesPtrToAPIEventSeriesPtr(in.Series),
		Action:             in.Action,
		Related:            k8sObjectReferencePtrToAPIObjectReferencePtr(in.Related),
		ReportingComponent: in.ReportingController,
		ReportingInstance:  in.ReportingInstance,
	}
}

func k8sEventSourceToAPIEventSource(in corev1.EventSource) eventspb.EventSource {
	return eventspb.EventSource{
		Component: in.Component,
		Host:      in.Host,
	}
}

func k8sEventSeriesPtrToAPIEventSeriesPtr(in *corev1.EventSeries) *eventspb.EventSeries {
	if in == nil {
		return nil
	}
	return &eventspb.EventSeries{
		Count:            in.Count,
		LastObservedTime: in.LastObservedTime,
	}
}

func k8sObjectReferenceToAPIObjectReference(in corev1.ObjectReference) eventspb.ObjectReference {
	return eventspb.ObjectReference{
		Kind:            in.Kind,
		Namespace:       in.Namespace,
		Name:            in.Name,
		UID:             string(in.UID),
		APIVersion:      in.APIVersion,
		ResourceVersion: in.ResourceVersion,
		FieldPath:       in.FieldPath,
	}
}

func k8sObjectReferencePtrToAPIObjectReferencePtr(in *corev1.ObjectReference) *eventspb.ObjectReference {
	if in == nil {
		return nil
	}
	out := k8sObjectReferenceToAPIObjectReference(*in)
	return &out
}
