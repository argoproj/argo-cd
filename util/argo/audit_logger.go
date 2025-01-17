package argo

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type AuditLogger struct {
	kIf            kubernetes.Interface
	component      string
	ns             string
	enableEventLog map[string]bool
}

type EventInfo struct {
	Type   string
	Reason string
}

type ObjectRef struct {
	Name            string
	Namespace       string
	ResourceVersion string
	UID             types.UID
}

const (
	EventReasonStatusRefreshed    = "StatusRefreshed"
	EventReasonResourceCreated    = "ResourceCreated"
	EventReasonResourceUpdated    = "ResourceUpdated"
	EventReasonResourceDeleted    = "ResourceDeleted"
	EventReasonResourceActionRan  = "ResourceActionRan"
	EventReasonOperationStarted   = "OperationStarted"
	EventReasonOperationCompleted = "OperationCompleted"
)

func (l *AuditLogger) logEvent(objMeta ObjectRef, gvk schema.GroupVersionKind, info EventInfo, message string, logFields map[string]string, eventLabels map[string]string) {
	logCtx := log.WithFields(log.Fields{
		"type":   info.Type,
		"reason": info.Reason,
	})
	for field, val := range logFields {
		logCtx = logCtx.WithField(field, val)
	}

	switch gvk.Kind {
	case application.ApplicationKind:
		logCtx = logCtx.WithField("application", objMeta.Name)
	case application.AppProjectKind:
		logCtx = logCtx.WithField("project", objMeta.Name)
	default:
		logCtx = logCtx.WithField("name", objMeta.Name)
	}
	t := metav1.Time{Time: time.Now()}
	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v.%x", objMeta.Name, t.UnixNano()),
			Labels:      eventLabels,
			Annotations: logFields,
		},
		Source: v1.EventSource{
			Component: l.component,
		},
		InvolvedObject: v1.ObjectReference{
			Kind:            gvk.Kind,
			Name:            objMeta.Name,
			Namespace:       objMeta.Namespace,
			ResourceVersion: objMeta.ResourceVersion,
			APIVersion:      gvk.GroupVersion().String(),
			UID:             objMeta.UID,
		},
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Message:        message,
		Type:           info.Type,
		Reason:         info.Reason,
	}
	logCtx.Info(message)
	_, err := l.kIf.CoreV1().Events(objMeta.Namespace).Create(context.Background(), &event, metav1.CreateOptions{})
	if err != nil {
		logCtx.Errorf("Unable to create audit event: %v", err)
		return
	}
}

func (l *AuditLogger) enableK8SEventLog(info EventInfo) bool {
	return l.enableEventLog["all"] || l.enableEventLog[info.Reason]
}

func (l *AuditLogger) LogAppEvent(app *v1alpha1.Application, info EventInfo, message, user string, eventLabels map[string]string) {
	if !l.enableK8SEventLog(info) {
		return
	}

	objectMeta := ObjectRef{
		Name:            app.ObjectMeta.Name,
		Namespace:       app.ObjectMeta.Namespace,
		ResourceVersion: app.ObjectMeta.ResourceVersion,
		UID:             app.ObjectMeta.UID,
	}
	fields := map[string]string{
		"dest-server":    app.Spec.Destination.Server,
		"dest-namespace": app.Spec.Destination.Namespace,
	}
	if user != "" {
		fields["user"] = user
	}
	l.logEvent(objectMeta, v1alpha1.ApplicationSchemaGroupVersionKind, info, message, fields, eventLabels)
}

func (l *AuditLogger) LogAppSetEvent(app *v1alpha1.ApplicationSet, info EventInfo, message, user string) {
	if !l.enableK8SEventLog(info) {
		return
	}

	objectMeta := ObjectRef{
		Name:            app.ObjectMeta.Name,
		Namespace:       app.ObjectMeta.Namespace,
		ResourceVersion: app.ObjectMeta.ResourceVersion,
		UID:             app.ObjectMeta.UID,
	}
	fields := map[string]string{}
	if user != "" {
		fields["user"] = user
	}
	l.logEvent(objectMeta, v1alpha1.ApplicationSetSchemaGroupVersionKind, info, message, fields, nil)
}

func (l *AuditLogger) LogResourceEvent(res *v1alpha1.ResourceNode, info EventInfo, message, user string) {
	if !l.enableK8SEventLog(info) {
		return
	}

	objectMeta := ObjectRef{
		Name:            res.ResourceRef.Name,
		Namespace:       res.ResourceRef.Namespace,
		ResourceVersion: res.ResourceRef.Version,
		UID:             types.UID(res.ResourceRef.UID),
	}
	fields := map[string]string{}
	if user != "" {
		fields["user"] = user
	}
	l.logEvent(objectMeta, schema.GroupVersionKind{
		Group:   res.Group,
		Version: res.Version,
		Kind:    res.Kind,
	}, info, message, fields, nil)
}

func (l *AuditLogger) LogAppProjEvent(proj *v1alpha1.AppProject, info EventInfo, message, user string) {
	if !l.enableK8SEventLog(info) {
		return
	}

	objectMeta := ObjectRef{
		Name:            proj.ObjectMeta.Name,
		Namespace:       proj.ObjectMeta.Namespace,
		ResourceVersion: proj.ObjectMeta.ResourceVersion,
		UID:             proj.ObjectMeta.UID,
	}
	fields := map[string]string{}
	if user != "" {
		fields["user"] = user
	}
	l.logEvent(objectMeta, v1alpha1.AppProjectSchemaGroupVersionKind, info, message, nil, nil)
}

func NewAuditLogger(ns string, kIf kubernetes.Interface, component string, enableK8sEvent []string) *AuditLogger {
	return &AuditLogger{
		ns:             ns,
		kIf:            kIf,
		component:      component,
		enableEventLog: setK8sEventList(enableK8sEvent),
	}
}

func setK8sEventList(enableK8sEvent []string) map[string]bool {
	enableK8sEventList := make(map[string]bool)

	for _, event := range enableK8sEvent {
		if event == "all" {
			enableK8sEventList = map[string]bool{
				"all": true,
			}
			return enableK8sEventList
		} else if event == "none" {
			enableK8sEventList = map[string]bool{}
			return enableK8sEventList
		}

		enableK8sEventList[event] = true
	}

	return enableK8sEventList
}

func DefaultEnableEventList() []string {
	return []string{"all"}
}
