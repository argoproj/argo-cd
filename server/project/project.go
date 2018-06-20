package project

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rbac"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Server provides a Project service
type Server struct {
	ns           string
	db           db.ArgoDB
	enf          *rbac.Enforcer
	appclientset appclientset.Interface
}

// NewServer returns a new instance of the Project service
func NewServer(ns string, db db.ArgoDB, appclientset appclientset.Interface, enf *rbac.Enforcer) *Server {
	return &Server{db: db, enf: enf, appclientset: appclientset, ns: ns}
}

// Create a new project.
func (s *Server) Create(ctx context.Context, q *ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "create", fmt.Sprintf("*/%s", q.Project.Name)) {
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
			if s.enf.EnforceClaims(ctx.Value("claims"), "projects", "get", fmt.Sprintf("*/%s", project.Name)) {
				newItems = append(newItems, project)
			}
		}
		list.Items = newItems
	}
	return list, err
}

// Get returns a project by name
func (s *Server) Get(ctx context.Context, q *ProjectQuery) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "get", fmt.Sprintf("*/%s", q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Get(q.Name, metav1.GetOptions{})
}

// Update updates a project
func (s *Server) Update(ctx context.Context, q *ProjectUpdateRequest) (*v1alpha1.AppProject, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "update", fmt.Sprintf("*/%s", q.Project.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Update(q.Project)
}

// Delete deletes a project
func (s *Server) Delete(ctx context.Context, q *ProjectQuery) (*EmptyResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "projects", "delete", fmt.Sprintf("*/%s", q.Name)) {
		return nil, grpc.ErrPermissionDenied
	}
	return &EmptyResponse{}, s.appclientset.ArgoprojV1alpha1().AppProjects(s.ns).Delete(q.Name, &metav1.DeleteOptions{})
}
