package notification

import (
	"context"

	"github.com/argoproj/notifications-engine/pkg/api"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/notification"
)

// Server provides an Application service
type Server struct {
	apiFactory api.Factory
}

// NewServer returns a new instance of the Application service
func NewServer(apiFactory api.Factory) notification.NotificationServiceServer {
	s := &Server{apiFactory: apiFactory}
	return s
}

// List returns list of notification triggers
func (s *Server) ListTriggers(_ context.Context, _ *notification.TriggersListRequest) (*notification.TriggerList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &notification.TriggerList{}, nil
		}
	}
	triggers := []*notification.Trigger{}
	for trigger := range api.GetConfig().Triggers {
		triggers = append(triggers, &notification.Trigger{Name: new(trigger)})
	}
	return &notification.TriggerList{Items: triggers}, nil
}

// List returns list of notification services
func (s *Server) ListServices(_ context.Context, _ *notification.ServicesListRequest) (*notification.ServiceList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &notification.ServiceList{}, nil
		}
		return nil, err
	}
	services := []*notification.Service{}
	for svc := range api.GetConfig().Services {
		services = append(services, &notification.Service{Name: new(svc)})
	}
	return &notification.ServiceList{Items: services}, nil
}

// List returns list of notification templates
func (s *Server) ListTemplates(_ context.Context, _ *notification.TemplatesListRequest) (*notification.TemplateList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &notification.TemplateList{}, nil
		}
		return nil, err
	}
	templates := []*notification.Template{}
	for tmpl := range api.GetConfig().Templates {
		templates = append(templates, &notification.Template{Name: new(tmpl)})
	}
	return &notification.TemplateList{Items: templates}, nil
}
