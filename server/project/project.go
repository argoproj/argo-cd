package project

import (
	"context"
	"fmt"

	"strings"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Project service
type Server struct {
	ns           string
	enf          *rbac.Enforcer
	appclientset appclientset.Interface
	auditLogger  *argo.AuditLogger
	projectLock  *util.KeyLock
}

// NewServer returns a new instance of the Project service
func NewServer(ns string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface, enf *rbac.Enforcer, projectLock *util.KeyLock) *Server {
	auditLogger := argo.NewAuditLogger(ns, kubeclientset, "argocd-server")
	return &Server{enf: enf, appclientset: appclientset, ns: ns, projectLock: projectLock, auditLogger: auditLogger}
}

// Create a new project.
func (s *Server) Create(ctx context.Context, q *ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "create", q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	if q.Project.Name == common.DefaultAppProjectName {
		return nil, status.Errorf(codes.InvalidArgument, "name '%s' is reserved and cannot be used as a project name", q.Project.Name)
	}
	err := validateProject(q.Project)
	if err != nil {
		return nil, err
	}
	res, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Create(q.Project)
	if err == nil {
		s.logEvent(res, ctx, argo.EventReasonResourceCreated, "create")
	}
	return res, err
}

// List returns list of projects
func (s *Server) List(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProjectList, error) {
	list, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).List(metav1.ListOptions{})
	list.Items = append(list.Items, v1alpha1.GetDefaultProject(s.ns))
	if list != nil {
		newItems := make([]v1alpha1.AppProject, 0)
		for i := range list.Items {
			project := list.Items[i]
			if s.enf.EnforceClaims(ctx.Value("claims"), "projects", "get", project.Name) {
				newItems = append(newItems, project)
			}
		}
		list.Items = newItems
	}
	return list, err
}

// Get returns a project by name
func (s *Server) Get(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "get", q.Name) {
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

func validateProject(p *v1alpha1.AppProject) error {
	destKeys := make(map[string]bool)
	for _, dest := range p.Spec.Destinations {
		key := fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)
		if _, ok := destKeys[key]; !ok {
			destKeys[key] = true
		} else {
			return status.Errorf(codes.InvalidArgument, "destination %s should not be listed more than once.", key)
		}
	}
	srcRepos := make(map[string]bool)
	for i, src := range p.Spec.SourceRepos {
		src = git.NormalizeGitURL(src)
		p.Spec.SourceRepos[i] = src
		if _, ok := srcRepos[src]; !ok {
			srcRepos[src] = true
		} else {
			return status.Errorf(codes.InvalidArgument, "source repository %s should not be listed more than once.", src)
		}
	}
	return nil
}

// Update updates a project
func (s *Server) Update(ctx context.Context, q *ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if q.Project.Name == common.DefaultAppProjectName {
		return nil, grpc.ErrPermissionDenied
	}
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "update", q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	err := validateProject(q.Project)
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
		s.logEvent(res, ctx, argo.EventReasonResourceUpdated, "update")
	}
	return res, err
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *ProjectQuery) (*EmptyResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "delete", q.Name) {
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
		s.logEvent(p, ctx, argo.EventReasonResourceDeleted, "delete")
	}
	return &EmptyResponse{}, err
}

func (s *Server) logEvent(p *v1alpha1.AppProject, ctx context.Context, reason string, action string) {
	s.auditLogger.LogAppProjEvent(p, argo.EventInfo{Reason: reason, Action: action, Username: session.Username(ctx)}, v1.EventTypeNormal)
}
