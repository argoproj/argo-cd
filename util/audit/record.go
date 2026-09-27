// Package audit contains the types shared by the Argo CD API server and the
// argocd-audit-controller to describe "who did what" in Argo CD.
//
// The API server produces Records for every mutating API call (and for web
// terminal sessions) and ships them to the audit controller, which is the
// single place where the audit trail is written out.
package audit

import (
	"time"
)

const (
	// RecordKind is the value of Record.Kind. It makes audit lines easy to
	// filter out of a mixed log stream (e.g. `jq 'select(.kind=="AuditRecord")'`).
	RecordKind = "AuditRecord"

	// SourceAPIServer marks records produced by argocd-server for API calls.
	SourceAPIServer = "argocd-server"
	// SourceKubernetesEvent marks records derived from Kubernetes Events that
	// Argo CD controllers emit (e.g. automated syncs, sync results).
	SourceKubernetesEvent = "kubernetes-event"

	// ResultSuccess and ResultFailure are the possible values of Result.Status.
	ResultSuccess = "success"
	ResultFailure = "failure"

	// UserAnonymous is used when a request carries no identity (anonymous access enabled, or auth disabled).
	UserAnonymous = "anonymous"
	// UserUnauthenticated is used when a request was rejected because its credentials were missing or invalid.
	UserUnauthenticated = "unauthenticated"

	// IngestPath is the HTTP path the audit controller accepts records on.
	IngestPath = "/api/v1/records"
	// QueryPath is the HTTP path the audit controller serves recent records on.
	QueryPath = "/api/v1/records"
)

// Record is a single entry of the audit trail.
type Record struct {
	Kind string `json:"kind"`
	// ID uniquely identifies the record.
	ID string `json:"id"`
	// Timestamp is the time the action was performed.
	Timestamp time.Time `json:"timestamp"`
	// ReceivedAt is set by the audit controller when it accepted the record.
	ReceivedAt *time.Time `json:"receivedAt,omitempty"`
	// Source is the component the record originates from.
	Source string `json:"source"`
	// Actor is who performed the action.
	Actor Actor `json:"actor"`
	// Action is a stable, machine friendly name of what was done, e.g.
	// "application.sync" or "cluster.delete".
	Action string `json:"action"`
	// Verb is the coarse kind of action (create, update, patch, delete, sync, ...).
	Verb string `json:"verb"`
	// Method is the fully qualified gRPC method, when the record comes from an API call.
	Method string `json:"method,omitempty"`
	// Resource is what the action was performed on.
	Resource Resource `json:"resource"`
	// Details holds additional, non-sensitive, action specific parameters
	// (sync revision, prune flag, resource action name, ...).
	Details map[string]string `json:"details,omitempty"`
	// Result is the outcome of the action.
	Result Result `json:"result"`
	// DurationMillis is how long the action took, when known.
	DurationMillis int64 `json:"durationMillis,omitempty"`
}

// Actor identifies who performed an action.
type Actor struct {
	// Username is the human readable user name (local account name, or the
	// email claim for SSO users).
	Username string `json:"username"`
	// Subject is the unique user identifier (token subject / federated identity).
	Subject string `json:"subject,omitempty"`
	// Issuer is the issuer of the token the user authenticated with
	// ("argocd" for local accounts, the OIDC issuer for SSO users).
	Issuer string `json:"issuer,omitempty"`
	// Groups are the groups of the user, as seen by RBAC.
	Groups []string `json:"groups,omitempty"`
	// SourceIP is the client address, resolved through trusted proxies.
	SourceIP string `json:"sourceIP,omitempty"`
	// UserAgent is the client's user agent (argocd CLI, browser, ...).
	UserAgent string `json:"userAgent,omitempty"`
}

// Resource identifies the object an action was performed on.
type Resource struct {
	// Type is the Argo CD resource type (application, project, cluster, repository, ...).
	Type string `json:"type"`
	// Name is the name (or URL/server for repositories and clusters) of the object.
	Name string `json:"name,omitempty"`
	// Namespace is the namespace of the object, when namespaced.
	Namespace string `json:"namespace,omitempty"`
	// Project is the Argo CD project the object belongs to, when known.
	Project string `json:"project,omitempty"`
}

// Result is the outcome of an action.
type Result struct {
	// Status is either ResultSuccess or ResultFailure.
	Status string `json:"status"`
	// Code is the gRPC status code name (OK, PermissionDenied, ...), or the
	// Kubernetes event reason for records derived from events.
	Code string `json:"code,omitempty"`
	// Message is a (truncated) error or event message.
	Message string `json:"message,omitempty"`
}

// MaxMessageLength bounds Result.Message so a single record stays small.
const MaxMessageLength = 1024

// Truncate shortens s to at most n bytes, marking the cut.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
