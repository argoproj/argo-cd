package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	gohttp "net/http"
	"net/textproto"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/util/kube"

	"github.com/argoproj/pkg/v2/grpc/http"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"

	//nolint:staticcheck
	"github.com/golang/protobuf/proto"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

// processAppFields extracts the requested fields from a single application into a map.
func processAppFields(app *v1alpha1.Application, fields map[string]any, exclude bool) (map[string]any, error) {
	converted := make(map[string]any)
	for field, fn := range appFields {
		if _, ok := fields["items."+field]; ok == exclude {
			continue
		}
		value := fn(app)
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
	return converted, nil
}

// streamApplicationListJSON writes the ApplicationList as JSON directly to w,
// streaming one application at a time to avoid buffering the entire response.
func streamApplicationListJSON(w io.Writer, appList *v1alpha1.ApplicationList, fields map[string]any, exclude bool) error {
	useFieldFilter := len(fields) > 0
	enc := json.NewEncoder(w)

	if _, err := w.Write([]byte("{")); err != nil {
		return err
	}

	// When not field-filtering, include TypeMeta fields to match json.Marshal(ApplicationList) output.
	if !useFieldFilter {
		if err := writeInlineTypeMeta(w, &appList.TypeMeta); err != nil {
			return err
		}
	}

	if _, err := w.Write([]byte(`"metadata":`)); err != nil {
		return err
	}
	if err := enc.Encode(appList.ListMeta); err != nil {
		return err
	}
	if appList.Items == nil {
		if _, err := w.Write([]byte(`,"items":null}`)); err != nil {
			return err
		}
		return nil
	}

	if _, err := w.Write([]byte(`,"items":[`)); err != nil {
		return err
	}

	for i := range appList.Items {
		if i > 0 {
			if _, err := w.Write([]byte(",")); err != nil {
				return err
			}
		}
		if useFieldFilter {
			converted, err := processAppFields(&appList.Items[i], fields, exclude)
			if err != nil {
				return err
			}
			if err := enc.Encode(converted); err != nil {
				return err
			}
		} else {
			if err := enc.Encode(&appList.Items[i]); err != nil {
				return err
			}
		}
	}
	if _, err := w.Write([]byte("]}")); err != nil {
		return err
	}
	return nil
}

// writeInlineTypeMeta writes the TypeMeta fields as inline JSON key-value pairs
// (with trailing comma), matching the behavior of json:",inline" on the struct tag.
// Respects omitempty: fields are only written when non-empty.
func writeInlineTypeMeta(w io.Writer, tm *metav1.TypeMeta) error {
	if tm.Kind != "" {
		if _, err := fmt.Fprintf(w, `"kind":"%s",`, tm.Kind); err != nil {
			return err
		}
	}
	if tm.APIVersion != "" {
		if _, err := fmt.Fprintf(w, `"apiVersion":"%s",`, tm.APIVersion); err != nil {
			return err
		}
	}
	return nil
}

// parseFieldSelection parses the "fields" query parameter into a field map and exclude flag.
func parseFieldSelection(req *gohttp.Request) (fields map[string]any, exclude bool) {
	fieldsQuery := req.URL.Query().Get("fields")
	if fieldsQuery == "" {
		return nil, false
	}
	if strings.HasPrefix(fieldsQuery, "-") {
		fieldsQuery = fieldsQuery[1:]
		exclude = true
	}
	fields = make(map[string]any)
	for field := range strings.SplitSeq(fieldsQuery, ",") {
		fields[field] = true
	}
	return fields, exclude
}

func processApplicationListField(v any, fields map[string]any, exclude bool) (any, error) {
	if appList, ok := v.(*v1alpha1.ApplicationList); ok {
		var items []map[string]any
		for i := range appList.Items {
			converted, err := processAppFields(&appList.Items[i], fields, exclude)
			if err != nil {
				return nil, err
			}
			items = append(items, converted)
		}
		return map[string]any{
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
	forward_ApplicationService_List_0 = func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, resp proto.Message, opts ...func(context.Context, gohttp.ResponseWriter, proto.Message) error) {
		appList, ok := resp.(*v1alpha1.ApplicationList)
		if !ok {
			runtime.ForwardResponseMessage(ctx, mux, marshaler, w, req, resp, opts...)
			return
		}

		if req.Header.Get("Accept") == "text/event-stream" {
			// Use old non-streaming processor
			http.UnaryForwarderWithFieldProcessor(processApplicationListField)(ctx, mux, marshaler, w, req, resp, opts...)
			return
		}

		// Replicate grpc-gateway ForwardResponseMessage header handling.
		md, ok := runtime.ServerMetadataFromContext(ctx)
		if ok {
			for k, vs := range md.HeaderMD {
				for _, v := range vs {
					w.Header().Add(fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, k), v)
				}
			}
			for k := range md.TrailerMD {
				tKey := textproto.CanonicalMIMEHeaderKey(fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k))
				w.Header().Add("Trailer", tKey)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		for _, opt := range opts {
			if err := opt(ctx, w, resp); err != nil {
				runtime.HTTPError(ctx, mux, marshaler, w, req, err)
				return
			}
		}

		fields, exclude := parseFieldSelection(req)

		if err := streamApplicationListJSON(w, appList, fields, exclude); err != nil {
			log.Errorf("Failed to stream application list response: %v", err)
			panic(gohttp.ErrAbortHandler)
		}

		if ok {
			for k, vs := range md.TrailerMD {
				tKey := fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k)
				for _, v := range vs {
					w.Header().Add(tKey, v)
				}
			}
		}
	}
	forward_ApplicationService_ManagedResources_0 = http.UnaryForwarder
}
