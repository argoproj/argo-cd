package audit

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"

	accountpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	repositorypkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/audit"
	util_session "github.com/argoproj/argo-cd/v3/util/session"
)

type memorySink struct {
	mu      sync.Mutex
	records []audit.Record
}

func (s *memorySink) Enqueue(r audit.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

func (s *memorySink) all() []audit.Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]audit.Record(nil), s.records...)
}

func TestClassify(t *testing.T) {
	tests := []struct {
		method  string
		audited bool
		want    MethodSpec
	}{
		{"/application.ApplicationService/Sync", true, MethodSpec{Action: "application.sync", Verb: VerbSync, ResourceType: "application"}},
		{"/application.ApplicationService/Create", true, MethodSpec{Action: "application.create", Verb: VerbCreate, ResourceType: "application"}},
		{"/application.ApplicationService/DeleteResource", true, MethodSpec{Action: "application.delete-resource", Verb: VerbDelete, ResourceType: "application"}},
		{"/application.ApplicationService/RunResourceActionV2", true, MethodSpec{Action: "application.run-resource-action-v2", Verb: VerbRunAction, ResourceType: "application"}},
		{"/application.ApplicationService/TerminateOperation", true, MethodSpec{Action: "application.terminate-operation", Verb: VerbTerminate, ResourceType: "application"}},
		{"/cluster.ClusterService/RotateAuth", true, MethodSpec{Action: "cluster.rotate-auth", Verb: VerbRotateAuth, ResourceType: "cluster"}},
		{"/account.AccountService/UpdatePassword", true, MethodSpec{Action: "account.update-password", Verb: VerbUpdatePassword, ResourceType: "account"}},
		{"/project.ProjectService/CreateToken", true, MethodSpec{Action: "project.create-token", Verb: VerbCreate, ResourceType: "project"}},
		{"/session.SessionService/Create", true, MethodSpec{Action: "session.login", Verb: VerbLogin, ResourceType: "session"}},
		{"/session.SessionService/Delete", true, MethodSpec{Action: "session.logout", Verb: VerbLogout, ResourceType: "session"}},
		// Read only calls are not audited.
		{"/application.ApplicationService/Get", false, MethodSpec{}},
		{"/application.ApplicationService/List", false, MethodSpec{}},
		{"/application.ApplicationService/ManagedResources", false, MethodSpec{}},
		{"/session.SessionService/GetUserInfo", false, MethodSpec{}},
		{"/version.VersionService/Version", false, MethodSpec{}},
		// Malformed method names are ignored.
		{"", false, MethodSpec{}},
		{"/Sync", false, MethodSpec{}},
		{"/application.ApplicationService/", false, MethodSpec{}},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got, ok := Classify(tt.method)
			assert.Equal(t, tt.audited, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKebab(t *testing.T) {
	assert.Equal(t, "sync", kebab("Sync"))
	assert.Equal(t, "run-resource-action-v2", kebab("RunResourceActionV2"))
	assert.Equal(t, "update-spec", kebab("UpdateSpec"))
	assert.Equal(t, "create-repository-credentials", kebab("CreateRepositoryCredentials"))
}

func TestDescribe_Sync(t *testing.T) {
	req := &applicationpkg.ApplicationSyncRequest{
		Name:         ptr.To("guestbook"),
		AppNamespace: ptr.To("argocd"),
		Project:      ptr.To("default"),
		Revision:     ptr.To("main"),
		Prune:        ptr.To(true),
		DryRun:       ptr.To(false),
		Resources: []*v1alpha1.SyncOperationResource{
			{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "guestbook-ui"},
			{Kind: "Service", Name: "guestbook-ui"},
		},
		// Manifests are user supplied content and must never reach the audit trail.
		Manifests: []string{"apiVersion: v1\nkind: Secret\ndata:\n  password: c2VjcmV0"},
	}
	res, details := Describe("application", req)
	assert.Equal(t, audit.Resource{Type: "application", Name: "guestbook", Namespace: "argocd", Project: "default"}, res)
	assert.Equal(t, map[string]string{
		"revision":  "main",
		"prune":     "true",
		"resources": "apps/Deployment/default/guestbook-ui,Service/guestbook-ui",
	}, details)
}

func TestDescribe_DoesNotLeakSecrets(t *testing.T) {
	req := &repositorypkg.RepoCreateRequest{
		Repo: &v1alpha1.Repository{
			Repo:          "https://github.com/argoproj/argocd-example-apps",
			Project:       "default",
			Username:      "git",
			Password:      "super-secret-password",
			SSHPrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----",
			BearerToken:   "super-secret-token",
		},
		Upsert: true,
	}
	res, details := Describe("repository", req)
	assert.Equal(t, audit.Resource{Type: "repository", Name: "https://github.com/argoproj/argocd-example-apps", Project: "default"}, res)
	assert.Equal(t, map[string]string{"upsert": "true"}, details)

	// Passwords of account updates must not be recorded either.
	res, details = Describe("account", &accountpkg.UpdatePasswordRequest{Name: "alice", CurrentPassword: "old", NewPassword: "new"})
	assert.Equal(t, "alice", res.Name)
	assert.Nil(t, details)
}

func TestDescribe_ManyResourcesAreBounded(t *testing.T) {
	req := &applicationpkg.ApplicationSyncRequest{Name: ptr.To("big")}
	for range 50 {
		req.Resources = append(req.Resources, &v1alpha1.SyncOperationResource{Kind: "ConfigMap", Name: "cm"})
	}
	_, details := Describe("application", req)
	assert.Contains(t, details["resources"], "...and 30 more")
}

func claimsContext(claims jwt.Claims) context.Context {
	//nolint:staticcheck // Argo CD stores the claims under this string key.
	return context.WithValue(context.Background(), "claims", claims)
}

// chain runs the audit interceptors around a fake authentication step, in the
// order the API server installs them.
func chain(a *Auditor, authenticate func(ctx context.Context) (context.Context, error), method string, req any, handler grpc.UnaryHandler) (any, error) {
	return chainCtx(context.Background(), a, authenticate, method, req, handler)
}

func chainCtx(ctx context.Context, a *Auditor, authenticate func(ctx context.Context) (context.Context, error), method string, req any, handler grpc.UnaryHandler) (any, error) {
	info := &grpc.UnaryServerInfo{FullMethod: method}
	return a.UnaryServerInterceptor()(ctx, req, info, func(ctx context.Context, req any) (any, error) {
		ctx, err := authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return a.IdentityUnaryServerInterceptor()(ctx, req, info, handler)
	})
}

func fixedClock() func() time.Time {
	t := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		t = t.Add(250 * time.Millisecond)
		return t
	}
}

func TestUnaryServerInterceptor_RecordsAuthenticatedUser(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{Now: fixedClock()})
	authenticate := func(ctx context.Context) (context.Context, error) {
		//nolint:staticcheck // Argo CD stores the claims under this string key.
		return context.WithValue(ctx, "claims", jwt.MapClaims{
			"iss":    "https://dex.example.com",
			"sub":    "CgVhbGljZRIEbGRhcA",
			"email":  "alice@example.com",
			"groups": []any{"platform-admins", "devs"},
		}), nil
	}
	req := &applicationpkg.ApplicationSyncRequest{Name: ptr.To("guestbook"), AppNamespace: ptr.To("argocd"), Prune: ptr.To(true)}
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("user-agent", "argocd-client/v3.2.0"))
	resp, err := chainCtx(incoming, a, authenticate, "/application.ApplicationService/Sync", req, func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	records := sink.all()
	require.Len(t, records, 1)
	r := records[0]
	assert.Equal(t, audit.RecordKind, r.Kind)
	assert.NotEmpty(t, r.ID)
	assert.Equal(t, audit.SourceAPIServer, r.Source)
	assert.Equal(t, "alice@example.com", r.Actor.Username)
	assert.Equal(t, "CgVhbGljZRIEbGRhcA", r.Actor.Subject)
	assert.Equal(t, "https://dex.example.com", r.Actor.Issuer)
	assert.Equal(t, []string{"platform-admins", "devs"}, r.Actor.Groups)
	assert.Equal(t, "argocd-client/v3.2.0", r.Actor.UserAgent)
	assert.Equal(t, "application.sync", r.Action)
	assert.Equal(t, VerbSync, r.Verb)
	assert.Equal(t, "/application.ApplicationService/Sync", r.Method)
	assert.Equal(t, audit.Resource{Type: "application", Name: "guestbook", Namespace: "argocd"}, r.Resource)
	assert.Equal(t, map[string]string{"prune": "true"}, r.Details)
	assert.Equal(t, audit.Result{Status: audit.ResultSuccess, Code: "OK"}, r.Result)
	assert.Equal(t, int64(250), r.DurationMillis)
}

func TestUnaryServerInterceptor_LocalAccount(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{})
	authenticate := func(ctx context.Context) (context.Context, error) {
		//nolint:staticcheck // Argo CD stores the claims under this string key.
		return context.WithValue(ctx, "claims", &jwt.RegisteredClaims{Subject: "admin", Issuer: util_session.SessionManagerClaimsIssuer}), nil
	}
	_, err := chain(a, authenticate, "/application.ApplicationService/Delete", &applicationpkg.ApplicationDeleteRequest{Name: ptr.To("guestbook")}, func(_ context.Context, _ any) (any, error) {
		return nil, status.Error(codes.PermissionDenied, "permission denied: applications, delete, default/guestbook, sub: admin")
	})
	require.Error(t, err)
	records := sink.all()
	require.Len(t, records, 1)
	assert.Equal(t, "admin", records[0].Actor.Username)
	assert.Equal(t, util_session.SessionManagerClaimsIssuer, records[0].Actor.Issuer)
	assert.Equal(t, audit.ResultFailure, records[0].Result.Status)
	assert.Equal(t, "PermissionDenied", records[0].Result.Code)
	assert.Contains(t, records[0].Result.Message, "permission denied")
}

func TestUnaryServerInterceptor_RejectedByAuthentication(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{})
	handlerCalled := false
	_, err := chain(a, func(_ context.Context) (context.Context, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid session: token is expired")
	}, "/cluster.ClusterService/Delete", &applicationpkg.ApplicationDeleteRequest{}, func(_ context.Context, _ any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
	require.Error(t, err)
	assert.False(t, handlerCalled)
	records := sink.all()
	require.Len(t, records, 1)
	assert.Equal(t, audit.UserUnauthenticated, records[0].Actor.Username)
	assert.Equal(t, "Unauthenticated", records[0].Result.Code)
}

func TestUnaryServerInterceptor_Login(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{})
	req := &sessionpkg.SessionCreateRequest{Username: "alice", Password: "wrong-password"}
	// Session creation skips authentication: the call goes through with no claims in the context, and
	// the identity interceptor still runs. The record must name the user logging in, not "anonymous".
	_, err := chain(a, func(ctx context.Context) (context.Context, error) { return ctx, nil },
		"/session.SessionService/Create", req, func(_ context.Context, _ any) (any, error) {
			return nil, status.Error(codes.Unauthenticated, "Invalid username or password")
		})
	require.Error(t, err)
	records := sink.all()
	require.Len(t, records, 1)
	r := records[0]
	assert.Equal(t, "session.login", r.Action)
	assert.Equal(t, "alice", r.Actor.Username)
	assert.Equal(t, audit.Resource{Type: "session", Name: "alice"}, r.Resource)
	assert.Equal(t, audit.ResultFailure, r.Result.Status)
	assert.NotContains(t, r.Details, "password")
}

func TestUnaryServerInterceptor_IgnoresReadOnlyCalls(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{})
	_, err := chain(a, func(ctx context.Context) (context.Context, error) { return ctx, nil },
		"/application.ApplicationService/Get", &applicationpkg.ApplicationQuery{Name: ptr.To("guestbook")},
		func(_ context.Context, _ any) (any, error) { return nil, nil })
	require.NoError(t, err)
	assert.Empty(t, sink.all())
}

func TestUnaryServerInterceptor_AnonymousAccess(t *testing.T) {
	sink := &memorySink{}
	a := NewAuditor(sink, Options{})
	// With anonymous access enabled, authentication succeeds without claims.
	_, err := chain(a, func(ctx context.Context) (context.Context, error) { return ctx, nil },
		"/application.ApplicationService/Sync", &applicationpkg.ApplicationSyncRequest{Name: ptr.To("guestbook")},
		func(_ context.Context, _ any) (any, error) { return nil, nil })
	require.NoError(t, err)
	records := sink.all()
	require.Len(t, records, 1)
	assert.Equal(t, audit.UserAnonymous, records[0].Actor.Username)
}

func TestTerminalExec(t *testing.T) {
	var nilAuditor *Auditor
	exec := nilAuditor.NewTerminalExec(httptest.NewRequest("GET", "/terminal", nil), "app", "argocd", "default", "ns", "pod", "main")
	assert.Nil(t, exec)
	// A nil *TerminalExec must be safe to use.
	exec.Denied(errors.New("denied"))
	exec.Started()
	exec.Finished(nil)

	sink := &memorySink{}
	a := NewAuditor(sink, Options{Now: fixedClock()})
	r := httptest.NewRequest("GET", "/terminal?pod=guestbook-ui-123", nil)
	r.RemoteAddr = "10.1.2.3:4567"
	r.Header.Set("User-Agent", "Mozilla/5.0")
	r = r.WithContext(claimsContext(&jwt.RegisteredClaims{Subject: "admin", Issuer: util_session.SessionManagerClaimsIssuer}))
	exec = a.NewTerminalExec(r, "guestbook", "argocd", "default", "default", "guestbook-ui-123", "guestbook-ui")
	exec.Started()
	exec.Finished(nil)
	exec.Denied(errors.New("permission denied: exec, create"))

	records := sink.all()
	require.Len(t, records, 3)
	for _, rec := range records {
		assert.Equal(t, "admin", rec.Actor.Username)
		assert.Equal(t, "10.1.2.3", rec.Actor.SourceIP)
		assert.Equal(t, "Mozilla/5.0", rec.Actor.UserAgent)
		assert.Equal(t, VerbExec, rec.Verb)
		assert.Equal(t, audit.Resource{Type: "application", Name: "guestbook", Namespace: "argocd", Project: "default"}, rec.Resource)
		assert.Equal(t, "guestbook-ui-123", rec.Details["resource.name"])
		assert.Equal(t, "guestbook-ui", rec.Details["container"])
	}
	assert.Equal(t, "application.exec", records[0].Action)
	assert.Equal(t, audit.ResultSuccess, records[0].Result.Status)
	assert.Equal(t, "application.exec-end", records[1].Action)
	assert.Positive(t, records[1].DurationMillis)
	assert.Equal(t, audit.ResultFailure, records[2].Result.Status)
	assert.Equal(t, "PermissionDenied", records[2].Result.Code)
	// Records must not share their details map.
	records[0].Details["container"] = "changed"
	assert.Equal(t, "guestbook-ui", records[1].Details["container"])
}
