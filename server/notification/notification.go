package notification

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	"github.com/argoproj/notifications-engine/pkg/api"
	apierr "k8s.io/apimachinery/pkg/api/errors"
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
func (s *Server) ListTriggers(ctx context.Context, q *notification.TriggersListRequest) (*notification.Triggers, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.Triggers{}, nil
		}
	}
	triggers := []string{}
	for trigger := range api.GetConfig().Triggers {
		triggers = append(triggers, trigger)
	}
	return &notification.Triggers{Triggers: triggers}, nil
}

// List returns list of notification services
func (s *Server) ListServices(ctx context.Context, q *notification.ServicesListRequest) (*notification.Services, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.Services{}, nil
		}
		return nil, err
	}
	services := []string{}
	for svc := range api.GetConfig().Services {
		services = append(services, svc)
	}
	return &notification.Services{Services: services}, nil
}

// List returns list of notification templates
func (s *Server) ListTemplates(ctx context.Context, q *notification.TemplatesListRequest) (*notification.Templates, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.Templates{}, nil
		}
		return nil, err
	}
	templates := []string{}
	for tmpl := range api.GetConfig().Templates {
		templates = append(templates, tmpl)
	}
	return &notification.Templates{Templates: templates}, nil
}
