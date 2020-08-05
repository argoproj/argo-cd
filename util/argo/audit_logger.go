package argo

import (
	"context"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"fmt"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type AuditLogger struct {
	kIf       kubernetes.Interface
	component string
	ns        string
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

func (l *AuditLogger) logEvent(objMeta ObjectRef, gvk schema.GroupVersionKind, info EventInfo, message string, logFields map[string]string) {
	logCtx := log.WithFields(log.Fields{
		"type":   info.Type,
		"reason": info.Reason,
	})
	for field, val := range logFields {
		logCtx = logCtx.WithField(field, val)
	}

	switch gvk.Kind {
	case "Application":
		logCtx = logCtx.WithField("application", objMeta.Name)
	case "AppProject":
		logCtx = logCtx.WithField("project", objMeta.Name)
	default:
		logCtx = logCtx.WithField("name", objMeta.Name)
	}
	t := metav1.Time{Time: time.Now()}
	event := v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v.%x", objMeta.Name, t.UnixNano()),
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

func (l *AuditLogger) LogAppEvent(app *v1alpha1.Application, info EventInfo, message string) {
	objectMeta := ObjectRef{
		Name:            app.ObjectMeta.Name,
		Namespace:       app.ObjectMeta.Namespace,
		ResourceVersion: app.ObjectMeta.ResourceVersion,
		UID:             app.ObjectMeta.UID,
	}
	l.logEvent(objectMeta, v1alpha1.ApplicationSchemaGroupVersionKind, info, message, map[string]string{
		"dest-server":    app.Spec.Destination.Server,
		"dest-namespace": app.Spec.Destination.Namespace,
	})
}

func (l *AuditLogger) LogResourceEvent(res *v1alpha1.ResourceNode, info EventInfo, message string) {
	objectMeta := ObjectRef{
		Name:            res.ResourceRef.Name,
		Namespace:       res.ResourceRef.Namespace,
		ResourceVersion: res.ResourceRef.Version,
		UID:             types.UID(res.ResourceRef.UID),
	}
	l.logEvent(objectMeta, schema.GroupVersionKind{
		Group:   res.Group,
		Version: res.Version,
		Kind:    res.Kind,
	}, info, message, nil)
}

func (l *AuditLogger) LogAppProjEvent(proj *v1alpha1.AppProject, info EventInfo, message string) {
	objectMeta := ObjectRef{
		Name:            proj.ObjectMeta.Name,
		Namespace:       proj.ObjectMeta.Namespace,
		ResourceVersion: proj.ObjectMeta.ResourceVersion,
		UID:             proj.ObjectMeta.UID,
	}
	l.logEvent(objectMeta, v1alpha1.AppProjectSchemaGroupVersionKind, info, message, nil)
}

func NewAuditLogger(ns string, kIf kubernetes.Interface, component string) *AuditLogger {
	return &AuditLogger{
		ns:        ns,
		kIf:       kIf,
		component: component,
	}
}
