package project

import (
	"context"
	"fmt"

	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Server provides a Project service
type Server struct {
	ns           string
	enf          *rbac.Enforcer
	appclientset appclientset.Interface
	projectLock  *util.KeyLock
}

// NewServer returns a new instance of the Project service
func NewServer(ns string, appclientset appclientset.Interface, enf *rbac.Enforcer, projectLock *util.KeyLock) *Server {
	return &Server{enf: enf, appclientset: appclientset, ns: ns, projectLock: projectLock}
}

// Create a new project.
func (s *Server) Create(ctx context.Context, q *ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "create", q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Create(q.Project)
}

// List returns list of projects
func (s *Server) List(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProjectList, error) {
	list, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).List(metav1.ListOptions{})
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

// Update updates a project
func (s *Server) Update(ctx context.Context, q *ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "update", q.Project.Name) {
		return nil, grpc.ErrPermissionDenied
	}
	s.projectLock.Lock(q.Project.Name)
	defer s.projectLock.Unlock(q.Project.Name)

	oldProj, err := s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Project.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	removed := getRemovedDestination(oldProj, q.Project)

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	removedUsed := make([]v1alpha1.ApplicationDestination, 0)
	for _, a := range argo.FilterByProjects(appsList.Items, []string{q.Project.Name}) {
		if dest, ok := removed[fmt.Sprintf("%s/%s", a.Spec.Destination.Server, a.Spec.Destination.Namespace)]; ok {
			removedUsed = append(removedUsed, dest)
		}
	}
	if len(removedUsed) > 0 {
		formattedRemovedUsedList := make([]string, len(removedUsed))
		for i := 0; i < len(removedUsed); i++ {
			formattedRemovedUsedList[i] = fmt.Sprintf("server: %s, namespace: %s", removedUsed[i].Server, removedUsed[i].Namespace)
		}
		return nil, status.Errorf(
			codes.InvalidArgument, "following destinations are used by one or more application and cannot be removed: %s", strings.Join(formattedRemovedUsedList, ";"))
	}

	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(q.Project)
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *ProjectQuery) (*EmptyResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "delete", q.Name) {
		return nil, grpc.ErrPermissionDenied
	}

	s.projectLock.Lock(q.Name)
	defer s.projectLock.Unlock(q.Name)

	appsList, err := s.appclientset.ArgoprojV1alpha1().Applications(s.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	apps := argo.FilterByProjects(appsList.Items, []string{q.Name})
	if len(apps) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "project is referenced by %d applications", len(apps))
	}
	return &EmptyResponse{}, s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
}
