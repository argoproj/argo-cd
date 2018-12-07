package project

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/grpc"
	projectutil "github.com/argoproj/argo-cd/util/project"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	jwt "github.com/dgrijalva/jwt-go"
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
func (s *Server) CreateToken(ctx context.Context, q *ProjectTokenCreateRequest) (*ProjectTokenResponse, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project) {
		return nil, grpc.ErrPermissionDenied
	}
	project, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = projectutil.ValidateProject(project)
	if err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	_, index, err := projectutil.GetRoleByName(project, q.Role)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project '%s' does not have role '%s'", q.Project, q.Role)
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

	project.Spec.Roles[index].JWTTokens = append(project.Spec.Roles[index].JWTTokens, v1alpha1.JWTToken{IssuedAt: issuedAt, ExpiresAt: expiresAt})
	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(project)
	if err != nil {
		return nil, err
	}
	s.logEvent(project, ctx, argo.EventReasonResourceCreated, "created token")
	return &ProjectTokenResponse{Token: jwtToken}, nil

}

// DeleteToken deletes a token in a project
func (s *Server) DeleteToken(ctx context.Context, q *ProjectTokenDeleteRequest) (*EmptyResponse, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionDelete, q.Project) {
		return nil, grpc.ErrPermissionDenied
	}
	project, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = projectutil.ValidateProject(project)
	if err != nil {
		return nil, err
	}

	s.projectLock.Lock(q.Project)
	defer s.projectLock.Unlock(q.Project)

	_, roleIndex, err := projectutil.GetRoleByName(project, q.Role)
	if err != nil {
		return &EmptyResponse{}, nil
	}
	_, jwtTokenIndex, err := projectutil.GetJWTToken(project, q.Role, q.Iat)
	if err != nil {
		return &EmptyResponse{}, nil
	}
	project.Spec.Roles[roleIndex].JWTTokens[jwtTokenIndex] = project.Spec.Roles[roleIndex].JWTTokens[len(project.Spec.Roles[roleIndex].JWTTokens)-1]
	project.Spec.Roles[roleIndex].JWTTokens = project.Spec.Roles[roleIndex].JWTTokens[:len(project.Spec.Roles[roleIndex].JWTTokens)-1]
	_, err = s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(project)
	if err != nil {
		return nil, err
	}
	s.logEvent(project, ctx, argo.EventReasonResourceDeleted, "deleted token")
	return &EmptyResponse{}, nil
}

// Create a new project.
func (s *Server) Create(ctx context.Context, q *ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionCreate, q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	projectutil.NormalizePolicies(q.Project)
	err := projectutil.ValidateProject(q.Project)
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
func (s *Server) List(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProjectList, error) {
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
func (s *Server) Get(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProject, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Name, metav1.GetOptions{})
}

func getRemovedDestination(oldProj, newProj *v1alpha1.AppProject) map[string]v1alpha1.ApplicationDestination {
	oldDest := make(map[string]v1alpha1.ApplicationDestination)
	newDest := make(map[string]v1alpha1.ApplicationDestination)
	for i := range oldProj.Spec.Destinations {
		dest := oldProj.Spec.Destinations[i]
		oldDest[fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)] = dest
	}
	for i := range newProj.Spec.Destinations {
		dest := newProj.Spec.Destinations[i]
		newDest[fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)] = dest
	}

	removed := make(map[string]v1alpha1.ApplicationDestination, 0)
	for key, dest := range oldDest {
		if _, ok := newDest[key]; !ok {
			removed[key] = dest
		}
	}
	return removed
}

func getRemovedSources(oldProj, newProj *v1alpha1.AppProject) map[string]bool {
	oldSrc := make(map[string]bool)
	newSrc := make(map[string]bool)
	for _, src := range oldProj.Spec.SourceRepos {
		oldSrc[src] = true
	}
	for _, src := range newProj.Spec.SourceRepos {
		newSrc[src] = true
	}

	removed := make(map[string]bool)
	for src := range oldSrc {
		if _, ok := newSrc[src]; !ok {
			removed[src] = true
		}
	}
	return removed
}

// Update updates a project
func (s *Server) Update(ctx context.Context, q *ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionUpdate, q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	projectutil.NormalizePolicies(q.Project)
	err := projectutil.ValidateProject(q.Project)
	if err != nil {
		return nil, err
	}
	s.projectLock.Lock(q.Project.Name)
	defer s.projectLock.Unlock(q.Project.Name)

	oldProj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	removedDst := getRemovedDestination(oldProj, q.Project)
	removedSrc := getRemovedSources(oldProj, q.Project)

	removedDstUsed := make([]v1alpha1.ApplicationDestination, 0)
	removedSrcUsed := make([]string, 0)

	for _, a := range argo.FilterByProjects(appsList.Items, []string{q.Project.Name}) {
		if dest, ok := removedDst[fmt.Sprintf("%s/%s", a.Spec.Destination.Server, a.Spec.Destination.Namespace)]; ok {
			removedDstUsed = append(removedDstUsed, dest)
		}
		if _, ok := removedSrc[a.Spec.Source.RepoURL]; ok {
			removedSrcUsed = append(removedSrcUsed, a.Spec.Source.RepoURL)
		}
	}
	if len(removedDstUsed) > 0 {
		formattedRemovedUsedList := make([]string, len(removedDstUsed))
		for i := 0; i < len(removedDstUsed); i++ {
			formattedRemovedUsedList[i] = fmt.Sprintf("server: %s, namespace: %s", removedDstUsed[i].Server, removedDstUsed[i].Namespace)
		}
		return nil, status.Errorf(
			codes.InvalidArgument, "following destinations are used by one or more application and cannot be removed: %s", strings.Join(formattedRemovedUsedList, ";"))
	}
	if len(removedSrcUsed) > 0 {
		return nil, status.Errorf(
			codes.InvalidArgument, "following source repos are used by one or more application and cannot be removed: %s", strings.Join(removedSrcUsed, ";"))
	}

	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(q.Project)
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceUpdated, "updated project")
	}
	return res, err
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *ProjectQuery) (*EmptyResponse, error) {
	if q.Name == common.DefaultAppProjectName {
		return nil, status.Errorf(codes.InvalidArgument, "name '%s' is reserved and cannot be deleted", q.Name)
	}
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionDelete, q.Name) {
		return nil, grpc.ErrPermissionDenied
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
	return &EmptyResponse{}, err
}

func (s *Server) ListEvents(ctx context.Context, q *ProjectQuery) (*v1.EventList, error) {
	if !s.enf.Enforce(ctx.Value("claims"), rbacpolicy.ResourceProjects, rbacpolicy.ActionGet, q.Name) {
		return nil, grpc.ErrPermissionDenied
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
