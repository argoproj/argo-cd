package application

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	gohttp "net/http"
	"net/textproto"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

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
	"spec.destination":           func(app *v1alpha1.Application) any { return app.Spec.Destination },
	"spec.project":               func(app *v1alpha1.Application) any { return app.Spec.Project },
	"spec.source":                func(app *v1alpha1.Application) any { return app.Spec.Source },
	"spec.sources":               func(app *v1alpha1.Application) any { return app.Spec.Sources },
	"spec.sourceHydrator":        func(app *v1alpha1.Application) any { return app.Spec.SourceHydrator },
	"spec.syncPolicy":            func(app *v1alpha1.Application) any { return app.Spec.SyncPolicy },
	"status.sourceHydrator":      func(app *v1alpha1.Application) any { return app.Status.SourceHydrator },
	"status.summary":             func(app *v1alpha1.Application) any { return app.Status.Summary },
	"status.sync.status":         func(app *v1alpha1.Application) any { return app.Status.Sync.Status },
	"status.sync.revision":       func(app *v1alpha1.Application) any { return app.Status.Sync.Revision },
	"status.health":              func(app *v1alpha1.Application) any { return app.Status.Health },
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
		if app.Status.OperationState != nil && app.Status.OperationState.Operation.Sync != nil {
			// The UI only needs the field's presence to classify the operation type,
			// so project an empty object as a pure presence marker and drop the
			// potentially large fields such as resources, manifests and sources to
			// keep the list/watch payload small.
			return &v1alpha1.SyncOperation{}
		}
		return nil
	},
}

// appFieldSpec is a precomputed appFields entry: the dotted field name, its
// pre-split path, and the value extractor.
type appFieldSpec struct {
	field string
	parts []string
	fn    func(app *v1alpha1.Application) any
}

// selectAppFields resolves the included fields against appFields once
// per request, so the per-application loop doesn't repeat the map lookups and
// path splitting for every item.
//
// appFields may contain both a field and its descendants (e.g. "spec" and
// "spec.destination"). When an ancestor is selected its value already covers
// every descendant, so descendant specs are dropped; otherwise the encoded item
// would repeat the shared "spec" key. Sorted by path segment, as
// encodeSelectedFields requires.
func selectAppFields(fields map[string]any) []appFieldSpec {
	selected := make(map[string]bool, len(appFields))
	for field := range appFields {
		if _, ok := fields["items."+field]; ok {
			selected[field] = true
		}
	}
	specs := make([]appFieldSpec, 0, len(selected))
	for field := range selected {
		parts := strings.Split(field, ".")
		coveredByAncestor := false
		for i := 1; i < len(parts); i++ {
			if selected[strings.Join(parts[:i], ".")] {
				coveredByAncestor = true
				break
			}
		}
		if coveredByAncestor {
			continue
		}
		specs = append(specs, appFieldSpec{field: field, parts: parts, fn: appFields[field]})
	}
	slices.SortFunc(specs, func(a, b appFieldSpec) int { return slices.Compare(a.parts, b.parts) })
	return specs
}

// streamApplicationListJSON streams appList to w. Outside exclude mode the
// encoder walks the struct, so list-level fields need no listing here;
// DefaultOptionsV1 keeps json.Marshal semantics.
func streamApplicationListJSON(w io.Writer, appList *v1alpha1.ApplicationList, fields map[string]any, exclude bool) error {
	switch {
	case len(fields) == 0:
		return jsonv2.MarshalWrite(w, appList, json.DefaultOptionsV1())
	case exclude:
		return writeExcludedList(w, appList, fields)
	}
	return writeFilteredList(w, appList, selectAppFields(fields))
}

// filteredApplicationList is the event-stream processor's result, encoded like
// the streaming branch.
type filteredApplicationList struct {
	list    *v1alpha1.ApplicationList
	fields  map[string]any
	exclude bool
}

func (f filteredApplicationList) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := streamApplicationListJSON(&buf, f.list, f.fields, f.exclude); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeFilteredList encodes each application as its selected fields. Filtered
// responses never carried kind/apiVersion.
func writeFilteredList(w io.Writer, appList *v1alpha1.ApplicationList, specs []appFieldSpec) error {
	filtered := *appList
	filtered.TypeMeta = metav1.TypeMeta{}
	encodeApp := jsonv2.MarshalToFunc(func(enc *jsontext.Encoder, app *v1alpha1.Application) error {
		return encodeSelectedFields(enc, app, specs)
	})
	return jsonv2.MarshalWrite(w, &filtered, json.DefaultOptionsV1(), jsonv2.WithMarshalers(encodeApp))
}

// writeExcludedList writes the full list minus the excluded paths, matching the
// generic argoproj/pkg filter: excluding a path drops everything under it. The
// appFields fast path can't express this (#31204). Each application is
// marshaled and filtered on its own, so the list is still streamed. That
// encodes each app twice (~1.6x a full list at 1k apps); exclude is rare.
// List-level members are listed by hand and must track ApplicationList.
func writeExcludedList(w io.Writer, appList *v1alpha1.ApplicationList, fields map[string]any) error {
	// ancestors holds every proper prefix of an excluded path; values elsewhere
	// are copied without being walked.
	ancestors := make(map[string]bool)
	for field := range fields {
		for i, c := range field {
			if c == '.' {
				ancestors[field[:i]] = true
			}
		}
	}
	encodeList := jsonv2.MarshalToFunc(func(enc *jsontext.Encoder, list *v1alpha1.ApplicationList) error {
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}
		member := func(name string, write func() error) error {
			if _, ok := fields[name]; ok {
				return nil
			}
			if err := enc.WriteToken(jsontext.String(name)); err != nil {
				return err
			}
			return write()
		}
		encode := func(path string, v any) func() error {
			return func() error { return encodeExcluding(enc, path, v, fields, ancestors) }
		}
		// Same members and omitempty handling as the ApplicationList struct tags.
		if list.Kind != "" {
			if err := member("kind", encode("kind", list.Kind)); err != nil {
				return err
			}
		}
		if list.APIVersion != "" {
			if err := member("apiVersion", encode("apiVersion", list.APIVersion)); err != nil {
				return err
			}
		}
		if err := member("metadata", encode("metadata", &list.ListMeta)); err != nil {
			return err
		}
		err := member("items", func() error {
			if list.Items == nil {
				return enc.WriteToken(jsontext.Null)
			}
			if err := enc.WriteToken(jsontext.BeginArray); err != nil {
				return err
			}
			for i := range list.Items {
				if err := encodeExcluding(enc, "items", &list.Items[i], fields, ancestors); err != nil {
					return err
				}
			}
			return enc.WriteToken(jsontext.EndArray)
		})
		if err != nil {
			return err
		}
		return enc.WriteToken(jsontext.EndObject)
	})
	return jsonv2.MarshalWrite(w, appList, json.DefaultOptionsV1(), jsonv2.WithMarshalers(encodeList))
}

// encodeExcluding writes v, located at path, without the excluded fields.
func encodeExcluding(enc *jsontext.Encoder, path string, v any, fields map[string]any, ancestors map[string]bool) error {
	if !ancestors[path] {
		return jsonv2.MarshalEncode(enc, v)
	}
	b, err := jsonv2.Marshal(v, json.DefaultOptionsV1())
	if err != nil {
		return err
	}
	return copyExcluding(enc, jsontext.NewDecoder(bytes.NewReader(b)), path, fields, ancestors)
}

// copyExcluding copies one value from dec to enc, skipping object members whose
// path is in fields. Array elements share their array's path, as in
// argoproj/pkg.
func copyExcluding(enc *jsontext.Encoder, dec *jsontext.Decoder, path string, fields map[string]any, ancestors map[string]bool) error {
	kind := dec.PeekKind()
	if !ancestors[path] || (kind != '{' && kind != '[') {
		v, err := dec.ReadValue()
		if err != nil {
			return err
		}
		return enc.WriteValue(v)
	}
	open, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if err := enc.WriteToken(open); err != nil {
		return err
	}
	closeKind := jsontext.Kind('}')
	if kind == '[' {
		closeKind = ']'
	}
	for dec.PeekKind() != closeKind {
		childPath := path
		if kind == '{' {
			name, err := dec.ReadToken()
			if err != nil {
				return err
			}
			childPath = path + "." + name.String()
			if _, ok := fields[childPath]; ok {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
			if err := enc.WriteToken(name); err != nil {
				return err
			}
		}
		if err := copyExcluding(enc, dec, childPath, fields, ancestors); err != nil {
			return err
		}
	}
	closing, err := dec.ReadToken()
	if err != nil {
		return err
	}
	return enc.WriteToken(closing)
}

// encodeSelectedFields writes the selected fields as one nested object, opening
// a parent only once a child has a value. specs must be sorted as
// selectAppFields sorts them.
func encodeSelectedFields(enc *jsontext.Encoder, app *v1alpha1.Application, specs []appFieldSpec) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	var open []string
	for _, spec := range specs {
		value := spec.fn(app)
		if value == nil {
			continue
		}
		parents := spec.parts[:len(spec.parts)-1]
		shared := commonPrefixLen(open, parents)
		for range len(open) - shared {
			if err := enc.WriteToken(jsontext.EndObject); err != nil {
				return err
			}
		}
		open = open[:shared]
		for _, p := range parents[shared:] {
			if err := enc.WriteToken(jsontext.String(p)); err != nil {
				return err
			}
			if err := enc.WriteToken(jsontext.BeginObject); err != nil {
				return err
			}
			open = append(open, p)
		}
		if err := enc.WriteToken(jsontext.String(spec.parts[len(spec.parts)-1])); err != nil {
			return err
		}
		if err := jsonv2.MarshalEncode(enc, value); err != nil {
			return err
		}
	}
	for range len(open) + 1 {
		if err := enc.WriteToken(jsontext.EndObject); err != nil {
			return err
		}
	}
	return nil
}

func commonPrefixLen(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// parseFieldSelection parses the "fields" query parameter into a field map and exclude flag.
// Keep the semantics in sync with newMarshaler in github.com/argoproj/pkg/v2/grpc/http
// (used via UnaryForwarderWithFieldProcessor on the Accept: text/event-stream branch),
// so both branches of the List endpoint filter identically.
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
	appList, ok := v.(*v1alpha1.ApplicationList)
	if !ok {
		return nil, errors.New("not an application list")
	}
	return filteredApplicationList{list: appList, fields: fields, exclude: exclude}, nil
}

func init() {
	logsForwarder := func(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w gohttp.ResponseWriter, req *gohttp.Request, recv func() (proto.Message, error), opts ...func(context.Context, gohttp.ResponseWriter, proto.Message) error) {
		if req.URL.Query().Get("download") == "true" {
			w.Header().Set("Content-Type", "application/octet-stream")
			fileName := "log"
			namespace := req.URL.Query().Get("namespace")
			podName := req.URL.Query().Get("podName")
			fromResourceName := podName == ""
			if fromResourceName {
				podName = req.URL.Query().Get("resourceName")
			}
			container := req.URL.Query().Get("container")
			// Pod names are DNS-1123 labels, but Deployment/ReplicaSet names (passed via
			// resourceName) are DNS-1123 subdomains, which additionally allow dots and are
			// longer, so they need the more permissive check.
			validName := kube.IsValidResourceName(podName)
			if fromResourceName {
				validName = len(validation.IsDNS1123Subdomain(podName)) == 0
			}
			if kube.IsValidResourceName(namespace) && validName && kube.IsValidResourceName(container) {
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
		md, hasMD := runtime.ServerMetadataFromContext(ctx)
		if hasMD {
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
			if req.Context().Err() != nil {
				// The client went away mid-stream (tab closed, CLI interrupted);
				// this is routine, not a server error.
				log.Debugf("Client disconnected while streaming application list response: %v", err)
			} else {
				log.Errorf("Failed to stream application list response: %v", err)
			}
			panic(gohttp.ErrAbortHandler)
		}

		if hasMD {
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
