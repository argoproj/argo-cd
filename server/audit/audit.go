// Package audit records "who did what" through the Argo CD API server.
//
// Every mutating gRPC call (and therefore every mutating REST call, which the
// grpc-gateway turns into gRPC calls) is turned into an audit.Record carrying
// the caller's identity, the target object, a small set of non-sensitive
// parameters and the outcome. Records are handed to a Sink, which in
// production is an audit.Shipper that forwards them to the
// argocd-audit-controller.
package audit

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/util/audit"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	util_session "github.com/argoproj/argo-cd/v3/util/session"
)

// Sink receives audit records. Implementations must not block.
type Sink interface {
	Enqueue(r audit.Record)
}

// Options configure an Auditor.
type Options struct {
	// GatewayToken, TrustedProxies and ClientIPHeader are used to resolve the
	// client address, exactly as for source IP logging.
	GatewayToken   string
	TrustedProxies []netip.Prefix
	ClientIPHeader string
	// Scopes returns the OIDC scopes RBAC reads groups from. Defaults to ["groups"].
	Scopes func() []string
	// Now returns the current time. Defaults to time.Now; overridden in tests.
	Now func() time.Time
}

// Auditor builds audit records for API server activity.
type Auditor struct {
	sink Sink
	opts Options
}

// NewAuditor returns an Auditor sending records to sink.
func NewAuditor(sink Sink, opts Options) *Auditor {
	if opts.Scopes == nil {
		opts.Scopes = func() []string { return []string{"groups"} }
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Auditor{sink: sink, opts: opts}
}

// identityHolder carries the caller identity from the interceptor that runs
// after authentication back to the interceptor that runs before it.
type identityHolder struct {
	set   bool
	actor audit.Actor
}

type identityHolderKey struct{}

// UnaryServerInterceptor records audited calls. It must be installed *before*
// the authentication interceptor, so that calls rejected by authentication are
// audited too, and be paired with IdentityUnaryServerInterceptor installed
// *after* the authentication interceptor.
func (a *Auditor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		spec, ok := Classify(info.FullMethod)
		if !ok {
			return handler(ctx, req)
		}
		holder := &identityHolder{}
		ctx = context.WithValue(ctx, identityHolderKey{}, holder)
		start := a.opts.Now()
		resp, err := handler(ctx, req)
		a.sink.Enqueue(a.buildRecord(ctx, holder, spec, info.FullMethod, req, err, start))
		return resp, err
	}
}

// IdentityUnaryServerInterceptor captures the authenticated identity for
// UnaryServerInterceptor. It must be installed after the authentication interceptor.
func (a *Auditor) IdentityUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if holder, ok := ctx.Value(identityHolderKey{}).(*identityHolder); ok {
			holder.actor = a.identityFromClaims(ctx)
			holder.set = true
		}
		return handler(ctx, req)
	}
}

func (a *Auditor) identityFromClaims(ctx context.Context) audit.Actor {
	actor := audit.Actor{
		Username: util_session.Username(ctx),
		Subject:  util_session.GetUserIdentifier(ctx),
		Issuer:   util_session.Iss(ctx),
		Groups:   util_session.Groups(ctx, a.opts.Scopes()),
	}
	if actor.Username == "" {
		actor.Username = audit.UserAnonymous
	}
	return actor
}

func (a *Auditor) buildRecord(ctx context.Context, holder *identityHolder, spec MethodSpec, fullMethod string, req any, err error, start time.Time) audit.Record {
	var actor audit.Actor
	switch {
	case spec.Verb == VerbLogin:
		// Login requests bypass authentication, so they carry no claims: the only identity is the one
		// being claimed. This must be checked before holder.set, which is also set for unauthenticated calls.
		if r, ok := req.(interface{ GetUsername() string }); ok {
			actor.Username = r.GetUsername()
		}
	case holder.set:
		actor = holder.actor
	case status.Code(err) == codes.Unauthenticated:
		actor.Username = audit.UserUnauthenticated
	default:
		actor.Username = audit.UserAnonymous
	}
	if actor.Username == "" {
		actor.Username = audit.UserAnonymous
	}
	actor.SourceIP = grpc_util.SourceIP(ctx, a.opts.GatewayToken, a.opts.TrustedProxies, a.opts.ClientIPHeader)
	actor.UserAgent = userAgent(ctx)

	resource, details := Describe(spec.ResourceType, req)
	if spec.Verb == VerbLogin || spec.Verb == VerbLogout {
		resource.Name = actor.Username
	}

	now := a.opts.Now()
	return audit.Record{
		Kind:           audit.RecordKind,
		ID:             uuid.NewString(),
		Timestamp:      start.UTC(),
		Source:         audit.SourceAPIServer,
		Actor:          actor,
		Action:         spec.Action,
		Verb:           spec.Verb,
		Method:         fullMethod,
		Resource:       resource,
		Details:        details,
		Result:         resultFromError(err),
		DurationMillis: now.Sub(start).Milliseconds(),
	}
}

func resultFromError(err error) audit.Result {
	if err == nil {
		return audit.Result{Status: audit.ResultSuccess, Code: "OK"}
	}
	st := status.Convert(err)
	return audit.Result{
		Status:  audit.ResultFailure,
		Code:    st.Code().String(),
		Message: audit.Truncate(st.Message(), audit.MaxMessageLength),
	}
}

func userAgent(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	// Calls relayed by grpc-gateway carry the HTTP client's user agent under this key.
	for _, key := range []string{"grpcgateway-user-agent", "user-agent"} {
		if v := md.Get(key); len(v) > 0 && v[0] != "" {
			return audit.Truncate(v[0], 256)
		}
	}
	return ""
}

// Verbs used in audit records.
const (
	VerbCreate          = "create"
	VerbUpdate          = "update"
	VerbPatch           = "patch"
	VerbDelete          = "delete"
	VerbSync            = "sync"
	VerbRollback        = "rollback"
	VerbTerminate       = "terminate"
	VerbRunAction       = "run-action"
	VerbUpdatePassword  = "update-password"
	VerbRotateAuth      = "rotate-auth"
	VerbInvalidateCache = "invalidate-cache"
	VerbLogin           = "login"
	VerbLogout          = "logout"
	VerbExec            = "exec"
)

// MethodSpec describes how an audited gRPC method is reported.
type MethodSpec struct {
	Action       string
	Verb         string
	ResourceType string
}

// verbPrefixes maps RPC name prefixes to verbs. Any RPC whose name starts with
// one of these prefixes is audited, so new mutating RPCs following the naming
// conventions of the API are covered without further changes. Longer prefixes
// must come first.
var verbPrefixes = []struct {
	prefix string
	verb   string
}{
	{"RunResourceAction", VerbRunAction},
	{"UpdatePassword", VerbUpdatePassword},
	{"RotateAuth", VerbRotateAuth},
	{"InvalidateCache", VerbInvalidateCache},
	{"Create", VerbCreate},
	{"Update", VerbUpdate},
	{"Patch", VerbPatch},
	{"Delete", VerbDelete},
	{"Sync", VerbSync},
	{"Rollback", VerbRollback},
	{"Terminate", VerbTerminate},
}

// methodOverrides takes precedence over the prefix rules.
var methodOverrides = map[string]MethodSpec{
	"/session.SessionService/Create": {Action: "session.login", Verb: VerbLogin, ResourceType: "session"},
	"/session.SessionService/Delete": {Action: "session.logout", Verb: VerbLogout, ResourceType: "session"},
}

// Classify reports whether fullMethod ("/package.Service/Method") is audited and how.
func Classify(fullMethod string) (MethodSpec, bool) {
	if spec, ok := methodOverrides[fullMethod]; ok {
		return spec, true
	}
	pkg, rpc, ok := splitMethod(fullMethod)
	if !ok {
		return MethodSpec{}, false
	}
	for _, p := range verbPrefixes {
		if strings.HasPrefix(rpc, p.prefix) {
			return MethodSpec{
				Action:       pkg + "." + kebab(rpc),
				Verb:         p.verb,
				ResourceType: pkg,
			}, true
		}
	}
	return MethodSpec{}, false
}

// splitMethod turns "/application.ApplicationService/Sync" into ("application", "Sync").
func splitMethod(fullMethod string) (pkg string, rpc string, ok bool) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	service, rpc, found := strings.Cut(fullMethod, "/")
	if !found || rpc == "" {
		return "", "", false
	}
	pkg, _, found = strings.Cut(service, ".")
	if !found || pkg == "" {
		return "", "", false
	}
	return strings.ToLower(pkg), rpc, true
}

// kebab converts "RunResourceActionV2" to "run-resource-action-v2".
func kebab(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// TerminalExec records a web terminal (pod exec) session. A nil *TerminalExec
// is valid and records nothing, so callers need not check whether auditing is enabled.
type TerminalExec struct {
	a     *Auditor
	base  audit.Record
	start time.Time
}

// NewTerminalExec prepares the audit records of a web terminal session. It
// returns nil when a is nil.
func (a *Auditor) NewTerminalExec(r *http.Request, app, appNamespace, project, namespace, pod, container string) *TerminalExec {
	if a == nil {
		return nil
	}
	ctx := r.Context()
	actor := a.identityFromClaims(ctx)
	actor.SourceIP = grpc_util.HTTPClientIP(r, a.opts.TrustedProxies, a.opts.ClientIPHeader)
	actor.UserAgent = audit.Truncate(r.UserAgent(), 256)
	return &TerminalExec{
		a: a,
		base: audit.Record{
			Kind:   audit.RecordKind,
			Source: audit.SourceAPIServer,
			Actor:  actor,
			Verb:   VerbExec,
			Resource: audit.Resource{
				Type:      "application",
				Name:      app,
				Namespace: appNamespace,
				Project:   project,
			},
			Details: map[string]string{
				"resource.kind":      "Pod",
				"resource.namespace": namespace,
				"resource.name":      pod,
				"container":          container,
			},
		},
		start: a.opts.Now(),
	}
}

func (t *TerminalExec) emit(action string, failureCode codes.Code, err error, withDuration bool) {
	if t == nil {
		return
	}
	r := t.base
	r.ID = uuid.NewString()
	r.Action = action
	now := t.a.opts.Now()
	r.Timestamp = now.UTC()
	r.Details = make(map[string]string, len(t.base.Details))
	for k, v := range t.base.Details {
		r.Details[k] = v
	}
	if err == nil {
		r.Result = audit.Result{Status: audit.ResultSuccess, Code: codes.OK.String()}
	} else {
		r.Result = audit.Result{
			Status:  audit.ResultFailure,
			Code:    failureCode.String(),
			Message: audit.Truncate(err.Error(), audit.MaxMessageLength),
		}
	}
	if withDuration {
		r.DurationMillis = now.Sub(t.start).Milliseconds()
	}
	t.a.sink.Enqueue(r)
}

// Denied records a terminal session that was refused (e.g. by RBAC).
func (t *TerminalExec) Denied(err error) {
	t.emit("application.exec", codes.PermissionDenied, err, false)
}

// Started records that a terminal session was opened.
func (t *TerminalExec) Started() {
	t.emit("application.exec", codes.OK, nil, false)
}

// Finished records that a terminal session ended, with its duration.
func (t *TerminalExec) Finished(err error) {
	t.emit("application.exec-end", codes.Unknown, err, true)
}
