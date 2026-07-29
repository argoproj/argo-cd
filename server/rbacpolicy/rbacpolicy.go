package rbacpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/rbac"
)

// PermCheckCacheTTL is how long a cached permission-check result is valid.
// On RBAC policy changes the cache is invalidated immediately via FlushPermCheckCache;
// TTL is a safety net for cases where the flush is missed (e.g., Redis unavailable).
const PermCheckCacheTTL = 60 * time.Second

// permCheckEntry is the value stored in the permission-check Redis cache.
type permCheckEntry struct {
	Epoch   uint64 `json:"e"`
	Allowed bool   `json:"a"`
}

// Enforcer provides an RBAC Claims Enforcer which additionally consults AppProject
// roles, jwt tokens, and groups. It is backed by an AppProject informer /lister cache and does not
// make any API calls during enforcement.
type Enforcer struct {
	enf         *rbac.Enforcer
	projLister  applister.AppProjectNamespaceLister
	scopes      []string
	permCache   *cacheutil.Cache
	policyEpoch atomic.Uint64
}

// NewRBACPolicyEnforcer returns a new RBAC Enforcer for the Argo CD API Server
func NewRBACPolicyEnforcer(enf *rbac.Enforcer, projLister applister.AppProjectNamespaceLister) *Enforcer {
	return &Enforcer{
		enf:        enf,
		projLister: projLister,
		scopes:     nil,
	}
}

// SetPermCheckCache wires up a Redis-backed cache for UserHasAnyPermission results.
// If not called, every call falls through to a live Casbin lookup.
func (p *Enforcer) SetPermCheckCache(c *cacheutil.Cache) {
	p.permCache = c
}

// FlushPermCheckCache invalidates all cached permission-check results by advancing
// the policy epoch. Call this whenever the RBAC policy changes.
func (p *Enforcer) FlushPermCheckCache() {
	p.policyEpoch.Add(1)
}

func (p *Enforcer) SetScopes(scopes []string) {
	p.scopes = scopes
}

func (p *Enforcer) GetScopes() []string {
	scopes := p.scopes
	if scopes == nil {
		scopes = rbac.DefaultScopes
	}
	return scopes
}

func IsProjectSubject(subject string) bool {
	_, _, ok := GetProjectRoleFromSubject(subject)
	return ok
}

func GetProjectRoleFromSubject(subject string) (string, string, bool) {
	parts := strings.Split(subject, ":")
	if len(parts) == 3 && parts[0] == "proj" {
		return parts[1], parts[2], true
	}
	return "", "", false
}

// EnforceClaims is an RBAC claims enforcer specific to the Argo CD API server
func (p *Enforcer) EnforceClaims(claims jwt.Claims, rvals ...any) bool {
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		return false
	}

	subject := jwtutil.GetUserIdentifier(mapClaims)
	// Check if the request is for an application resource. We have special enforcement which takes
	// into consideration the project's token and group bindings
	var runtimePolicy string
	var projName string
	proj := p.getProjectFromRequest(rvals...)
	if proj != nil {
		if IsProjectSubject(subject) {
			return p.enforceProjectToken(subject, proj, rvals...)
		}
		runtimePolicy = proj.ProjectPoliciesString()
		projName = proj.Name
	}

	// NOTE: This calls prevent multiple creation of the wrapped enforcer
	enforcer := p.enf.CreateEnforcerWithRuntimePolicy(projName, runtimePolicy)

	// Check the subject. This is typically the 'admin' case.
	// NOTE: the call to EnforceWithCustomEnforcer will also consider the default role
	vals := append([]any{subject}, rvals[1:]...)
	if p.enf.EnforceWithCustomEnforcer(enforcer, vals...) {
		return true
	}

	scopes := p.scopes
	if scopes == nil {
		scopes = rbac.DefaultScopes
	}
	// Finally check if any of the user's groups grant them permissions
	groups := jwtutil.GetScopeValues(mapClaims, scopes)

	// Get groups to reduce the amount to checking groups
	groupingPolicies, err := enforcer.GetGroupingPolicy()
	if err != nil {
		log.WithError(err).Error("failed to get grouping policy")
		return false
	}
	for gidx := range groups {
		for gpidx := range groupingPolicies {
			// Prefilter user groups by groups defined in the model
			if groupingPolicies[gpidx][0] == groups[gidx] {
				vals := append([]any{groups[gidx]}, rvals[1:]...)
				if p.enf.EnforceWithCustomEnforcer(enforcer, vals...) {
					return true
				}
				break
			}
		}
	}
	logCtx := log.WithFields(log.Fields{"claims": claims, "rval": rvals, "subject": subject, "groups": groups, "project": projName, "scopes": scopes})
	logCtx.Debug("enforce failed")
	return false
}

// UserHasAnyPermission returns true if the user (or any of their groups) has at least one 'allow' permission granted
// by the current policy or the default role.
// Results are cached in Redis (keyed by username+groups+policy epoch) to avoid a Casbin lookup on every GetUserInfo
// call for SSO users.
func (p *Enforcer) UserHasAnyPermission(username string, groups []string) bool {
	if p.permCache != nil {
		epoch := p.policyEpoch.Load()
		key := permCheckCacheKey(username, groups)
		var entry permCheckEntry
		if err := p.permCache.GetItem(key, &entry); err == nil && entry.Epoch == epoch {
			return entry.Allowed
		}
		allowed := p.userHasAnyPermissionUncached(username, groups)
		_ = p.permCache.SetItem(key, &permCheckEntry{Epoch: epoch, Allowed: allowed},
			&cacheutil.CacheActionOpts{Expiration: PermCheckCacheTTL})
		return allowed
	}
	return p.userHasAnyPermissionUncached(username, groups)
}

func (p *Enforcer) userHasAnyPermissionUncached(username string, groups []string) bool {
	// The built-in admin is a superuser and is never subject to this check.
	if username == common.ArgoCDAdminUsername {
		return true
	}
	if defaultRole := p.enf.GetDefaultRole(); defaultRole != "" {
		if p.enf.HasAnyAllowPermission(defaultRole) {
			return true
		}
	}
	if p.enf.HasAnyAllowPermission(username) {
		return true
	}
	return slices.ContainsFunc(groups, p.enf.HasAnyAllowPermission)
}

// permCheckCacheKey builds a deterministic Redis key for a (username, groups) pair.
func permCheckCacheKey(username string, groups []string) string {
	sorted := make([]string, len(groups))
	copy(sorted, groups)
	sort.Strings(sorted)

	h := sha256.New()
	fmt.Fprintf(h, "%q", append([]string{username}, sorted...))
	return "rbac|perm-check|" + hex.EncodeToString(h.Sum(nil))
}

// GetPreventLoginWithoutPermissions returns the flag value from the RBAC configmap.
func (p *Enforcer) GetPreventLoginWithoutPermissions() bool {
	return p.enf.GetPreventLoginWithoutPermissions()
}

// getProjectFromRequest parses the project name from the RBAC request and returns the associated
// project (if it exists)
func (p *Enforcer) getProjectFromRequest(rvals ...any) *v1alpha1.AppProject {
	if len(rvals) != 4 {
		return nil
	}
	getProjectByName := func(projName string) *v1alpha1.AppProject {
		proj, err := p.projLister.Get(projName)
		if err != nil {
			return nil
		}
		return proj
	}
	if res, ok := rvals[1].(string); ok {
		if obj, ok := rvals[3].(string); ok {
			switch res {
			case rbac.ResourceApplicationSets, rbac.ResourceApplications, rbac.ResourceRepositories, rbac.ResourceClusters, rbac.ResourceLogs, rbac.ResourceExec:
				if objSplit := strings.Split(obj, "/"); len(objSplit) >= 2 {
					return getProjectByName(objSplit[0])
				}
			case rbac.ResourceProjects:
				// we also automatically give project tokens and groups 'get' access to the project
				return getProjectByName(obj)
			}
		}
	}
	return nil
}

// enforceProjectToken will check to see the valid token has not yet been revoked in the project
func (p *Enforcer) enforceProjectToken(subject string, proj *v1alpha1.AppProject, rvals ...any) bool {
	subjectSplit := strings.Split(subject, ":")
	if len(subjectSplit) != 3 {
		return false
	}
	projName, _ := subjectSplit[1], subjectSplit[2]
	if projName != proj.Name {
		// this should never happen (we generated a project token for a different project)
		return false
	}

	vals := append([]any{subject}, rvals[1:]...)
	return p.enf.EnforceRuntimePolicy(proj.Name, proj.ProjectPoliciesString(), vals...)
}
