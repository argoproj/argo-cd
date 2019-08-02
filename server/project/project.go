package project

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	jwtutil "github.com/argoproj/argo-cd/util/jwt"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
)

const (
	// JWTTokenSubFormat format of the JWT token subject that Argo CD vends out.
	JWTTokenSubFormat = "proj:%s:%s"
)

// Server provides a Project service
type Server struct {
	ns            string
	enf           *rbac.Enforcer
	appclientset  appclientset.Interface
	kubeclientset kubernetes.Interface
	auditLogger   *argo.AuditLogger
	projectLock   *util.KeyLock
	sessionMgr    *session.SessionManager
}

// NewServer returns a new instance of the Project service
func NewServer(ns string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface, enf *rbac.Enforcer, projectLock *util.KeyLock, sessionMgr *session.SessionManager) *Server {
	auditLogger := argo.NewAuditLogger(ns, kubeclientset, "argocd-server")
	return &Server{enf: enf, appclientset: appclientset, kubeclientset: kubeclientset, ns: ns, projectLock: projectLock, auditLogger: auditLogger, sessionMgr: sessionMgr}
}

// CreateToken creates a new token to access a project
func (s *Server) CreateToken(ctx context.Context, q *project.ProjectTokenCreateRequest) (*project.ProjectTokenResponse, error) {
	prj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = prj.ValidateProject()
	if err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	role, index, err := prj.GetRoleByName(q.Role)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project '%s' does not have role '%s'", q.Project, q.Role)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project); err != nil {
		if !jwtutil.IsMember(jwtutil.Claims(ctx.Value("claims")), role.Groups) {
			return nil, err
		}
	}
	tokenName := fmt.Sprintf(JWTTokenSubFormat, q.Project, q.Role)
	jwtToken, err := s.sessionMgr.Create(tokenName, q.ExpiresIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	parser := &jwt.Parser{
		SkipClaimsValidation: true,
	}
	claims := jwt.StandardClaims{}
	_, _, err = parser.ParseUnverified(jwtToken, &claims)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	issuedAt := claims.IssuedAt
	expiresAt := claims.ExpiresAt

	prj.Spec.Roles[index].JWTTokens = append(prj.Spec.Roles[index].JWTTokens, v1alpha1.JWTToken{IssuedAt: issuedAt, ExpiresAt: expiresAt})
	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(prj)
	if err != nil {
		return nil, err
	}
	s.logEvent(prj, ctx, argo.EventReasonResourceCreated, "created token")
	return &project.ProjectTokenResponse{Token: jwtToken}, nil

}

// DeleteToken deletes a token in a project
func (s *Server) DeleteToken(ctx context.Context, q *project.ProjectTokenDeleteRequest) (*project.EmptyResponse, error) {
	prj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = prj.ValidateProject()
	if err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	role, roleIndex, err := prj.GetRoleByName(q.Role)
	if err != nil {
		return &project.EmptyResponse{}, nil
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project); err != nil {
		if !jwtutil.IsMember(jwtutil.Claims(ctx.Value("claims")), role.Groups) {
			return nil, err
		}
	}
	_, jwtTokenIndex, err := prj.GetJWTToken(q.Role, q.Iat)
	if err != nil {
		return &project.EmptyResponse{}, nil
	}
	prj.Spec.Roles[roleIndex].JWTTokens[jwtTokenIndex] = prj.Spec.Roles[roleIndex].JWTTokens[len(prj.Spec.Roles[roleIndex].JWTTokens)-1]
	prj.Spec.Roles[roleIndex].JWTTokens = prj.Spec.Roles[roleIndex].JWTTokens[:len(prj.Spec.Roles[roleIndex].JWTTokens)-1]
	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(prj)
	if err != nil {
		return nil, err
	}
	s.logEvent(prj, ctx, argo.EventReasonResourceDeleted, "deleted token")
	return &project.EmptyResponse{}, nil
}

// Create a new project.
func (s *Server) Create(ctx context.Context, q *project.ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionCreate, q.Project.Name); err != nil {
		return nil, err
	}
	q.Project.NormalizePolicies()
	err := q.Project.ValidateProject()
	if err != nil {
		return nil, err
	}
	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Create(q.Project)
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceCreated, "created project")
	}
	return res, err
}

// List returns list of projects
func (s *Server) List(ctx context.Context, q *project.ProjectQuery) (*v1alpha1.AppProjectList, error) {
	list, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).List(metav1.ListOptions{})
	if list != nil {
		newItems := make([]v1alpha1.AppProject, 0)
		for i := range list.Items {
			project := list.Items[i]
			if s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, project.Name) {
				newItems = append(newItems, project)
			}
		}
		list.Items = newItems
	}
	return list, err
}

// Get returns a project by name
func (s *Server) Get(ctx context.Context, q *project.ProjectQuery) (*v1alpha1.AppProject, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Name, metav1.GetOptions{})
}

// Update updates a project
func (s *Server) Update(ctx context.Context, q *project.ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project.Name); err != nil {
		return nil, err
	}
	q.Project.NormalizePolicies()
	err := q.Project.ValidateProject()
	if err != nil {
		return nil, err
	}
	s.projectLock.Lock(q.Project.Name)
	defer s.projectLock.Unlock(q.Project.Name)

	oldProj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, cluster := range difference(q.Project.Spec.DestinationClusters(), oldProj.Spec.DestinationClusters()) {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, cluster); err != nil {
			return nil, err
		}
	}

	for _, repoUrl := range difference(q.Project.Spec.SourceRepos, oldProj.Spec.SourceRepos) {
		if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceRepositories, rbacpolicy.ActionUpdate, repoUrl); err != nil {
			return nil, err
		}
	}

	clusterResourceWhitelistsEqual := reflect.DeepEqual(q.Project.Spec.ClusterResourceWhitelist, oldProj.Spec.ClusterResourceWhitelist)
	namespacesResourceBlacklistsEqual := reflect.DeepEqual(q.Project.Spec.NamespaceResourceBlacklist, oldProj.Spec.NamespaceResourceBlacklist)
	if !clusterResourceWhitelistsEqual || !namespacesResourceBlacklistsEqual {
		for _, cluster := range q.Project.Spec.DestinationClusters() {
			if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceClusters, rbacpolicy.ActionUpdate, cluster); err != nil {
				return nil, err
			}
		}
	}

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var srcValidatedApps []v1alpha1.Application
	var dstValidatedApps []v1alpha1.Application

	for _, a := range argo.FilterByProjects(appsList.Items, []string{q.Project.Name}) {
		if oldProj.IsSourcePermitted(a.Spec.Source) {
			srcValidatedApps = append(srcValidatedApps, a)
		}
		if oldProj.IsDestinationPermitted(a.Spec.Destination) {
			dstValidatedApps = append(dstValidatedApps, a)
		}
	}

	invalidSrcCount := 0
	invalidDstCount := 0

	for _, a := range srcValidatedApps {
		if !q.Project.IsSourcePermitted(a.Spec.Source) {
			invalidSrcCount++
		}
	}
	for _, a := range dstValidatedApps {
		if !q.Project.IsDestinationPermitted(a.Spec.Destination) {
			invalidDstCount++
		}
	}

	var parts []string
	if invalidSrcCount > 0 {
		parts = append(parts, fmt.Sprintf("%d applications source became invalid", invalidSrcCount))
	}
	if invalidDstCount > 0 {
		parts = append(parts, fmt.Sprintf("%d applications destination became invalid", invalidDstCount))
	}
	if len(parts) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "as a result of project update %s", strings.Join(parts, " and "))
	}

	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(q.Project)
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceUpdated, "updated project")
	}
	return res, err
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *project.ProjectQuery) (*project.EmptyResponse, error) {
	if q.Name == common.DefaultAppProjectName {
		return nil, status.Errorf(codes.InvalidArgument, "name '%s' is reserved and cannot be deleted", q.Name)
	}
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionDelete, q.Name); err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Name)
	defer s.projectLock.Unlock(q.Name)

	p, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	apps := argo.FilterByProjects(appsList.Items, []string{q.Name})
	if len(apps) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "project is referenced by %d applications", len(apps))
	}
	err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
	if err == nil {
		s.logEvent(p, ctx, argo.EventReasonResourceDeleted, "deleted project")
	}
	return &project.EmptyResponse{}, err
}

func (s *Server) ListEvents(ctx context.Context, q *project.ProjectQuery) (*v1.EventList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name); err != nil {
		return nil, err
	}
	proj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	fieldSelector := fields.SelectorFromSet(map[string]string{
		"involvedObject.name":      proj.Name,
		"involvedObject.uid":       string(proj.UID),
		"involvedObject.namespace": proj.Namespace,
	}).String()
	return s.kubeclientset.CoreV1().Events(s.ns).List(metav1.ListOptions{FieldSelector: fieldSelector})
}

func (s *Server) logEvent(a *v1alpha1.AppProject, ctx context.Context, reason string, action string) {
	eventInfo := argo.EventInfo{Type: v1.EventTypeNormal, Reason: reason}
	user := session.Username(ctx)
	if user == "" {
		user = "Unknown user"
	}
	message := fmt.Sprintf("%s %s", user, action)
	s.auditLogger.LogAppProjEvent(a, eventInfo, message)
}
