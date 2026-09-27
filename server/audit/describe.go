package audit

import (
	"fmt"
	"strconv"
	"strings"

	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/audit"
)

// maxListedItems bounds list-valued details (sync resources, revisions, ...).
const maxListedItems = 20

// Describe extracts the target object and a small set of details from an API
// request.
//
// Only an explicit allow-list of fields is read, so that secrets carried by
// requests (passwords, tokens, TLS keys, patches, manifests, ...) can never end
// up in the audit trail.
func Describe(resourceType string, req any) (audit.Resource, map[string]string) {
	res := audit.Resource{Type: resourceType}
	details := map[string]string{}

	describeObjects(req, &res)
	describeIdentifiers(resourceType, req, &res)
	describeDetails(req, details)

	if len(details) == 0 {
		details = nil
	}
	return res, details
}

// describeObjects handles requests that embed a full object (create/update requests).
func describeObjects(req any, res *audit.Resource) {
	if r, ok := req.(interface {
		GetApplication() *v1alpha1.Application
	}); ok {
		if app := r.GetApplication(); app != nil {
			res.Name, res.Namespace, res.Project = app.Name, app.Namespace, app.Spec.Project
		}
	}
	if r, ok := req.(interface {
		GetApplicationset() *v1alpha1.ApplicationSet
	}); ok {
		if appset := r.GetApplicationset(); appset != nil {
			res.Name, res.Namespace, res.Project = appset.Name, appset.Namespace, appset.Spec.Template.Spec.Project
		}
	}
	if r, ok := req.(interface {
		GetProject() *v1alpha1.AppProject
	}); ok {
		if proj := r.GetProject(); proj != nil {
			res.Name, res.Namespace, res.Project = proj.Name, proj.Namespace, proj.Name
		}
	}
	if r, ok := req.(interface {
		GetCluster() *v1alpha1.Cluster
	}); ok {
		if cluster := r.GetCluster(); cluster != nil {
			res.Name, res.Project = cluster.Server, cluster.Project
		}
	}
	if r, ok := req.(interface {
		GetRepo() *v1alpha1.Repository
	}); ok {
		if repo := r.GetRepo(); repo != nil {
			res.Name, res.Project = repo.Repo, repo.Project
		}
	}
	if r, ok := req.(interface {
		GetCreds() *v1alpha1.RepoCreds
	}); ok {
		if creds := r.GetCreds(); creds != nil {
			res.Name = creds.URL
		}
	}
}

// describeIdentifiers handles requests that reference an object by name/URL.
func describeIdentifiers(resourceType string, req any, res *audit.Resource) {
	setIfEmpty := func(dst *string, v string) {
		if *dst == "" && v != "" {
			*dst = v
		}
	}
	if r, ok := req.(interface{ GetName() string }); ok {
		setIfEmpty(&res.Name, r.GetName())
	}
	if r, ok := req.(interface{ GetAppNamespace() string }); ok {
		setIfEmpty(&res.Namespace, r.GetAppNamespace())
	}
	if r, ok := req.(interface{ GetAppsetNamespace() string }); ok {
		setIfEmpty(&res.Namespace, r.GetAppsetNamespace())
	}
	if r, ok := req.(interface{ GetProject() string }); ok {
		p := r.GetProject()
		setIfEmpty(&res.Project, p)
		if resourceType == "project" {
			// Project token requests identify the project by this field.
			setIfEmpty(&res.Name, p)
		}
	}
	// Clusters are identified by server URL, name, or an explicit ID.
	if r, ok := req.(interface{ GetServer() string }); ok {
		setIfEmpty(&res.Name, r.GetServer())
	}
	if r, ok := req.(interface {
		GetId() *clusterpkg.ClusterID
	}); ok {
		if id := r.GetId(); id != nil {
			setIfEmpty(&res.Name, id.GetValue())
		}
	}
	if r, ok := req.(interface{ GetRepo() string }); ok {
		setIfEmpty(&res.Name, r.GetRepo())
	}
	if r, ok := req.(interface{ GetUrl() string }); ok {
		setIfEmpty(&res.Name, r.GetUrl())
	}
	if r, ok := req.(interface{ GetKeyID() string }); ok {
		setIfEmpty(&res.Name, r.GetKeyID())
	}
	if r, ok := req.(interface{ GetHostNamePattern() string }); ok {
		setIfEmpty(&res.Name, r.GetHostNamePattern())
	}
}

// describeDetails copies allow-listed, non-sensitive parameters.
func describeDetails(req any, details map[string]string) {
	putString := func(key, v string) {
		if v != "" {
			details[key] = audit.Truncate(v, 256)
		}
	}
	putBool := func(key string, v bool) {
		if v {
			details[key] = "true"
		}
	}
	putList := func(key string, items []string) {
		if len(items) == 0 {
			return
		}
		if len(items) > maxListedItems {
			items = append(items[:maxListedItems:maxListedItems], fmt.Sprintf("...and %d more", len(items)-maxListedItems))
		}
		details[key] = audit.Truncate(strings.Join(items, ","), 1024)
	}

	// Managed resource targeted by resource level requests (PatchResource, DeleteResource, RunResourceAction...).
	if r, ok := req.(interface{ GetResourceName() string }); ok {
		putString("resource.name", r.GetResourceName())
		if r, ok := req.(interface{ GetNamespace() string }); ok {
			putString("resource.namespace", r.GetNamespace())
		}
		if r, ok := req.(interface{ GetKind() string }); ok {
			putString("resource.kind", r.GetKind())
		}
		if r, ok := req.(interface{ GetGroup() string }); ok {
			putString("resource.group", r.GetGroup())
		}
		if r, ok := req.(interface{ GetVersion() string }); ok {
			putString("resource.version", r.GetVersion())
		}
	}

	if r, ok := req.(interface{ GetRevision() string }); ok {
		putString("revision", r.GetRevision())
	}
	if r, ok := req.(interface{ GetRevisions() []string }); ok {
		putList("revisions", r.GetRevisions())
	}
	if r, ok := req.(interface{ GetAction() string }); ok {
		putString("action", r.GetAction())
	}
	if r, ok := req.(interface{ GetPropagationPolicy() string }); ok {
		putString("propagationPolicy", r.GetPropagationPolicy())
	}
	if r, ok := req.(interface{ GetPatchType() string }); ok {
		putString("patchType", r.GetPatchType())
	}
	if r, ok := req.(interface{ GetRole() string }); ok {
		putString("role", r.GetRole())
	}
	if r, ok := req.(interface{ GetCertType() string }); ok {
		putString("certType", r.GetCertType())
	}
	if r, ok := req.(interface{ GetUpdatedFields() []string }); ok {
		putList("updatedFields", r.GetUpdatedFields())
	}
	if r, ok := req.(interface{ GetDryRun() bool }); ok {
		putBool("dryRun", r.GetDryRun())
	}
	if r, ok := req.(interface{ GetPrune() bool }); ok {
		putBool("prune", r.GetPrune())
	}
	if r, ok := req.(interface{ GetCascade() bool }); ok {
		putBool("cascade", r.GetCascade())
	}
	if r, ok := req.(interface{ GetUpsert() bool }); ok {
		putBool("upsert", r.GetUpsert())
	}
	if r, ok := req.(interface{ GetForce() bool }); ok {
		putBool("force", r.GetForce())
	}
	if r, ok := req.(interface{ GetExpiresIn() int64 }); ok {
		if v := r.GetExpiresIn(); v != 0 {
			details["expiresIn"] = strconv.FormatInt(v, 10)
		}
	}
	// Rollback targets a history ID; token requests carry a (non-secret) token ID.
	if r, ok := req.(interface{ GetId() int64 }); ok {
		details["historyId"] = strconv.FormatInt(r.GetId(), 10)
	}
	if r, ok := req.(interface{ GetId() string }); ok {
		putString("tokenId", r.GetId())
	}
	// Partial syncs: which resources were synced.
	if r, ok := req.(interface {
		GetResources() []*v1alpha1.SyncOperationResource
	}); ok {
		var items []string
		for _, res := range r.GetResources() {
			if res == nil {
				continue
			}
			items = append(items, syncResourceKey(res))
		}
		putList("resources", items)
	}
}

func syncResourceKey(r *v1alpha1.SyncOperationResource) string {
	parts := []string{}
	if r.Group != "" {
		parts = append(parts, r.Group)
	}
	parts = append(parts, r.Kind)
	if r.Namespace != "" {
		parts = append(parts, r.Namespace)
	}
	parts = append(parts, r.Name)
	return strings.Join(parts, "/")
}
