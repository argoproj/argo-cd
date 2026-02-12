package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	gohttp "net/http"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/kube"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	//nolint:staticcheck
	"github.com/golang/protobuf/proto"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	googproto "google.golang.org/protobuf/proto"
)

// appFields is a map of fields that can be selected from an application.
// The manually maintained list is required because application list response might include thousands of applications
// and JSON based field handling is too slow.
var appFields = map[string]func(app *v1alpha1.Application) any{
	"metadata.name":              func(app *v1alpha1.Application) any { return app.Name },
	"metadata.namespace":         func(app *v1alpha1.Application) any { return app.Namespace },
	"metadata.annotations":       func(app *v1alpha1.Application) any { return app.Annotations },
	"metadata.labels":            func(app *v1alpha1.Application) any { return app.Labels },
	"metadata.creationTimestamp": func(app *v1alpha1.Application) any { return app.CreationTimestamp },
	"metadata.deletionTimestamp": func(app *v1alpha1.Application) any { return app.DeletionTimestamp },
	"spec":                       func(app *v1alpha1.Application) any { return app.Spec },
	"status.sourceHydrator":      func(app *v1alpha1.Application) any { return app.Status.SourceHydrator },
	"status.sync.status":         func(app *v1alpha1.Application) any { return app.Status.Sync.Status },
	"status.health":              func(app *v1alpha1.Application) any { return app.Status.Health },
	"status.summary":             func(app *v1alpha1.Application) any { return app.Status.Summary },
	"status.operationState.startedAt": func(app *v1alpha1.Application) any {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.StartedAt
		}
		return nil
	},
	"status.operationState.finishedAt": func(app *v1alpha1.Application) any {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.FinishedAt
		}
		return nil
	},
	"status.resources": func(app *v1alpha1.Application) any {
		if len(app.Status.Resources) > 0 {
			return app.Status.Resources
		}
		return nil
	},
	"operation.sync": func(app *v1alpha1.Application) any {
		if app.Operation != nil {
			return app.Operation.Sync
		}
		return nil
	},
	"status.operationState.phase": func(app *v1alpha1.Application) any {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.Phase
		}
		return nil
	},
	"status.operationState.operation.sync": func(app *v1alpha1.Application) any {
		if app.Status.OperationState != nil {
			return app.Status.OperationState.SyncResult
		}
		return nil
	},
}

func processApplicationListField(v any, fields map[string]any, exclude bool) (any, error) {
	if appList, ok := v.(*v1alpha1.ApplicationList); ok {
		var items []map[string]any
		for _, app := range appList.Items {
			converted := make(map[string]any)
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
				for i := range parts {
					subField := parts[i]
					if i == len(parts)-1 {
						item[subField] = value
					} else {
						if _, ok := item[subField]; !ok {
							item[subField] = make(map[string]any)
						}
						nestedMap, ok := item[subField].(map[string]any)
						if !ok {
							return nil, fmt.Errorf("field %s is not a map", field)
						}
						item = nestedMap
					}
				}
			}
		}
		return map[string]any{
			"items":    items,
			"metadata": appList.ListMeta,
		}, nil
	}
	return nil, errors.New("not an application list")
}

// streamForwarder handles streaming responses for grpc-gateway v2
func streamForwarder(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (googproto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", marshaler.ContentType(nil))

	for {
		resp, err := recv()
		if err != nil {
			runtime.HTTPError(ctx, mux, marshaler, w, req, err)
			return
		}

		// Write the message
		buf, err := marshaler.Marshal(resp)
		if err != nil {
			runtime.HTTPError(ctx, mux, marshaler, w, req, err)
			return
		}

		if _, err := w.Write(buf); err != nil {
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return
		}

		if f, ok := w.(gohttp.Flusher); ok {
			f.Flush()
		}
	}
}

// newStreamForwarder creates a stream forwarder with a message key function
func newStreamForwarder(messageKey func(proto.Message) (string, error)) func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (googproto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
	return func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (googproto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Content-Type", marshaler.ContentType(nil))

		for {
			resp, err := recv()
			if err != nil {
				runtime.HTTPError(ctx, mux, marshaler, w, req, err)
				return
			}

			// Try to extract the key if the message is a gogo proto type
			if gogoMsg, ok := resp.(proto.Message); ok {
				_, _ = messageKey(gogoMsg)
			}

			buf, err := marshaler.Marshal(resp)
			if err != nil {
				runtime.HTTPError(ctx, mux, marshaler, w, req, err)
				return
			}

			if _, err := w.Write(buf); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				return
			}

			if f, ok := w.(gohttp.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// unaryForwarderWithFieldProcessor creates a unary forwarder with field processing
func unaryForwarderWithFieldProcessor(fieldProcessor func(val any, fields map[string]any, exclude bool) (any, error)) func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, resp googproto.Message, opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
	return func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, resp googproto.Message, opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
		// Process fields if requested
		fieldsParam := req.URL.Query().Get("fields")
		if fieldsParam != "" {
			fields := make(map[string]any)
			for _, field := range strings.Split(fieldsParam, ",") {
				fields[strings.TrimSpace(field)] = true
			}
			processed, err := fieldProcessor(resp, fields, false)
			if err == nil && processed != nil {
				// Marshal the processed result directly as JSON
				buf, err := json.Marshal(processed)
				if err != nil {
					runtime.HTTPError(ctx, mux, marshaler, w, req, err)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(buf)
				return
			}
		}

		// Fall back to default forwarding
		runtime.ForwardResponseMessage(ctx, mux, marshaler, w, req, resp, opts...)
	}
}

// unaryForwarder is a simple unary forwarder
func unaryForwarder(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, resp googproto.Message, opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
	runtime.ForwardResponseMessage(ctx, mux, marshaler, w, req, resp, opts...)
}

func init() {
	logsForwarder := func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (googproto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, googproto.Message) error) {
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
				// Type assert through any to handle gogo types
				if logEntry, ok := any(msg).(*LogEntry); ok {
					if logEntry.GetLast() {
						return
					}
					if _, err = w.Write([]byte(logEntry.GetContent() + "\n")); err != nil {
						return
					}
				}
			}
		} else {
			streamForwarder(ctx, mux, marshaler, w, req, recv, opts...)
		}
	}
	forward_ApplicationService_PodLogs_0 = logsForwarder
	forward_ApplicationService_PodLogs_1 = logsForwarder
	forward_ApplicationService_WatchResourceTree_0 = streamForwarder
	forward_ApplicationService_Watch_0 = newStreamForwarder(func(message proto.Message) (string, error) {
		// Use any to handle gogo types
		event, ok := any(message).(*v1alpha1.ApplicationWatchEvent)
		if !ok {
			return "", errors.New("unexpected message type")
		}
		return event.Application.Name, nil
	})
	forward_ApplicationService_List_0 = unaryForwarderWithFieldProcessor(processApplicationListField)
	forward_ApplicationService_ManagedResources_0 = unaryForwarder
}
