package rbacpolicy

import (
	"strings"

	jwt "github.com/dgrijalva/jwt-go/v4"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	applister "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/rbac"
)

const (
	// please add new items to Resources
	ResourceClusters     = "clusters"
	ResourceProjects     = "projects"
	ResourceApplications = "applications"
	ResourceRepositories = "repositories"
	ResourceCertificates = "certificates"
	ResourceAccounts     = "accounts"
	ResourceGPGKeys      = "gpgkeys"

	// please add new items to Actions
	ActionGet      = "get"
	ActionCreate   = "create"
	ActionUpdate   = "update"
	ActionDelete   = "delete"
	ActionSync     = "sync"
	ActionOverride = "override"
	ActionAction   = "action"
)

var (
	defaultScopes = []string{"groups"}
	Resources     = []string{
		ResourceClusters,
		ResourceProjects,
		ResourceApplications,
		ResourceRepositories,
		ResourceCertificates,
	}
	Actions = []string{
		ActionGet,
		ActionCreate,
		ActionUpdate,
		ActionDelete,
		ActionSync,
		ActionOverride,
	}
)

// RBACPolicyEnforcer provides an RBAC Claims Enforcer which additionally consults AppProject
// roles, jwt tokens, and groups. It is backed by a AppProject informer/lister cache and does not
// make any API calls during enforcement.
type RBACPolicyEnforcer struct {
	enf        *rbac.Enforcer
	projLister applister.AppProjectNamespaceLister
	scopes     []string
}

// NewRBACPolicyEnforcer returns a new RBAC Enforcer for the Argo CD API Server
func NewRBACPolicyEnforcer(enf *rbac.Enforcer, projLister applister.AppProjectNamespaceLister) *RBACPolicyEnforcer {
	return &RBACPolicyEnforcer{
		enf:        enf,
		projLister: projLister,
		scopes:     nil,
	}
}

func (p *RBACPolicyEnforcer) SetScopes(scopes []string) {
	p.scopes = scopes
}

func (p *RBACPolicyEnforcer) GetScopes() []string {
	scopes := p.scopes
	if scopes == nil {
		scopes = defaultScopes
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
func (p *RBACPolicyEnforcer) EnforceClaims(claims jwt.Claims, rvals ...interface{}) bool {
	mapClaims, err := jwtutil.MapClaims(claims)
	if err != nil {
		return false
	}

	subject := jwtutil.StringField(mapClaims, "sub")
	// Check if the request is for an application resource. We have special enforcement which takes
	// into consideration the project's token and group bindings
	var runtimePolicy string
	proj := p.getProjectFromRequest(rvals...)
	if proj != nil {
		if IsProjectSubject(subject) {
			return p.enforceProjectToken(subject, proj, rvals...)
		}
		runtimePolicy = proj.ProjectPoliciesString()
	}

	// Check the subject. This is typically the 'admin' case.
	// NOTE: the call to EnforceRuntimePolicy will also consider the default role
	vals := append([]interface{}{subject}, rvals[1:]...)
	if p.enf.EnforceRuntimePolicy(runtimePolicy, vals...) {
		return true
	}

	scopes := p.scopes
	if scopes == nil {
		scopes = defaultScopes
	}
	// Finally check if any of the user's groups grant them permissions
	groups := jwtutil.GetScopeValues(mapClaims, scopes)
	for _, group := range groups {
		vals := append([]interface{}{group}, rvals[1:]...)
		if p.enf.EnforceRuntimePolicy(runtimePolicy, vals...) {
			return true
		}
	}
	logCtx := log.WithField("claims", claims).WithField("rval", rvals)
	logCtx.Debug("enforce failed")
	return false
}

// getProjectFromRequest parses the project name from the RBAC request and returns the associated
// project (if it exists)
func (p *RBACPolicyEnforcer) getProjectFromRequest(rvals ...interface{}) *v1alpha1.AppProject {
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
			case ResourceApplications:
				if objSplit := strings.Split(obj, "/"); len(objSplit) == 2 {
					return getProjectByName(objSplit[0])
				}
			case ResourceProjects:
				// we also automatically give project tokens and groups 'get' access to the project
				return getProjectByName(obj)
			}
		}
	}
	return nil
}

// enforceProjectToken will check to see the valid token has not yet been revoked in the project
func (p *RBACPolicyEnforcer) enforceProjectToken(subject string, proj *v1alpha1.AppProject, rvals ...interface{}) bool {
	subjectSplit := strings.Split(subject, ":")
	if len(subjectSplit) != 3 {
		return false
	}
	projName, _ := subjectSplit[1], subjectSplit[2]
	if projName != proj.Name {
		// this should never happen (we generated a project token for a different project)
		return false
	}

	vals := append([]interface{}{subject}, rvals[1:]...)
	return p.enf.EnforceRuntimePolicy(proj.ProjectPoliciesString(), vals...)

}
