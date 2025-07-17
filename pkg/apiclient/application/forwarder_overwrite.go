package application

import (
	"context"
	"errors"
	"fmt"
	gohttp "net/http"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/kube"

	"github.com/argoproj/pkg/grpc/http"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	// nolint:staticcheck
	"github.com/golang/protobuf/proto"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// appFields is a map of fields that can be selected from an application.
// The manually maintained list is required because application list response might include thousands of applications
// and JSON based field handling is too slow.
var appFields = map[string]func(app *v1alpha1.Application) interface{}{
	"metadata.name":              func(app *v1alpha1.Application) interface{} { return app.Name },
	"metadata.namespace":         func(app *v1alpha1.Application) interface{} { return app.Namespace },
	"metadata.annotations":       func(app *v1alpha1.Application) interface{} { return app.Annotations },
	"metadata.labels":            func(app *v1alpha1.Application) interface{} { return app.Labels },
	"metadata.creationTimestamp": func(app *v1alpha1.Application) interface{} { return app.CreationTimestamp },
	"metadata.deletionTimestamp": func(app *v1alpha1.Application) interface{} { return app.DeletionTimestamp },
	"spec":                       func(app *v1alpha1.Application) interface{} { return app.Spec },
	"status.sync.status":         func(app *v1alpha1.Application) interface{} { return app.Status.Sync.Status },
	"status.health":              func(app *v1alpha1.Application) interface{} { return app.Status.Health },
	"status.summary":             func(app *v1alpha1.Application) interface{} { return app.Status.Summary },
	"status.operationState.startedAt": func(app *v1alpha1.Application) interface{} {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.StartedAt
		}
		return nil
	},
	"status.operationState.finishedAt": func(app *v1alpha1.Application) interface{} {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.FinishedAt
		}
		return nil
	},
	"status.resources": func(app *v1alpha1.Application) interface{} {
		if len(app.Status.Resources) > 0 {
			return app.Status.Resources
		}
		return nil
	},
	"operation.sync": func(app *v1alpha1.Application) interface{} {
		if app.Operation != nil {
			return app.Operation.Sync
		}
		return nil
	},
	"status.operationState.phase": func(app *v1alpha1.Application) interface{} {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.Phase
		}
		return nil
	},
	"status.operationState.operation.sync": func(app *v1alpha1.Application) interface{} {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.SyncResult
		}
		return nil
	},
}

func processApplicationListField(v interface{}, fields map[string]interface{}, exclude bool) (interface{}, error) {
	if appList, ok := v.(*v1alpha1.ApplicationList); ok {
		var items []map[string]interface{}
		for _, app := range appList.Items {
			converted := make(map[string]interface{})
			items = append(items, converted)
			for field, fn := range appFields {
				if _, ok := fields["items."+field]; ok == exclude {
					continue
				}
				value := fn(&app)
				if value == nil {
					continue
				}
				parts := strings.Split(field, ".")
				item := converted
				for i := 0; i < len(parts); i++ {
					subField := parts[i]
					if i == len(parts)-1 {
						item[subField] = value
					} else {
						if _, ok := item[subField]; !ok {
							item[subField] = make(map[string]interface{})
						}
						nestedMap, ok := item[subField].(map[string]interface{})
						if !ok {
							return nil, fmt.Errorf("field %s is not a map", field)
						}
						item = nestedMap
					}
				}
			}
		}
		return map[string]interface{}{
			"items":    items,
			"metadata": appList.ListMeta,
		}, nil
	}
	return nil, errors.New("not an application list")
}

func init() {
	logsForwarder := func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (proto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, proto.Message) error) {
		if req.URL.Query().Get("download") == "true" {
			w.Header().Set("Content-Type", "application/octet-stream")
			fileName := "log"
			namespace := req.URL.Query().Get("namespace")
			podName := req.URL.Query().Get("podName")
			container := req.URL.Query().Get("container")
			if kube.IsValidResourceName(namespace) && kube.IsValidResourceName(podName) && kube.IsValidResourceName(container) {
				fileName = fmt.Sprintf("%s-%s-%s", namespace, podName, container)
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment;filename="%s.log"`, fileName))
			for {
				msg, err := recv()
				if err != nil {
					_, _ = w.Write([]byte(err.Error()))
					return
				}
				if logEntry, ok := msg.(*LogEntry); ok {
					if logEntry.GetLast() {
						return
					}
					if _, err = w.Write([]byte(logEntry.GetContent() + "\n")); err != nil {
						return
					}
				}
			}
		} else {
			http.StreamForwarder(ctx, mux, marshaler, w, req, recv, opts...)
		}
	}
	forward_ApplicationService_PodLogs_0 = logsForwarder
	forward_ApplicationService_PodLogs_1 = logsForwarder
	forward_ApplicationService_WatchResourceTree_0 = http.StreamForwarder
	forward_ApplicationService_Watch_0 = http.NewStreamForwarder(func(message proto.Message) (string, error) {
		event, ok := message.(*v1alpha1.ApplicationWatchEvent)
		if !ok {
			return "", errors.New("unexpected message type")
		}
		return event.Application.Name, nil
	})
	forward_ApplicationService_List_0 = http.UnaryForwarderWithFieldProcessor(processApplicationListField)
	forward_ApplicationService_ManagedResources_0 = http.UnaryForwarder
}
