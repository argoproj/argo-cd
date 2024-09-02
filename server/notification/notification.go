package notification

import (
	"context"

	"github.com/argoproj/notifications-engine/pkg/api"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
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
func (s *Server) ListTriggers(ctx context.Context, q *notification.TriggersListRequest) (*notification.TriggerList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.TriggerList{}, nil
		}
	}
	triggers := []*notification.Trigger{}
	for trigger := range api.GetConfig().Triggers {
		triggers = append(triggers, &notification.Trigger{Name: ptr.To(trigger)})
	}
	return &notification.TriggerList{Items: triggers}, nil
}

// List returns list of notification services
func (s *Server) ListServices(ctx context.Context, q *notification.ServicesListRequest) (*notification.ServiceList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.ServiceList{}, nil
		}
		return nil, err
	}
	services := []*notification.Service{}
	for svc := range api.GetConfig().Services {
		services = append(services, &notification.Service{Name: ptr.To(svc)})
	}
	return &notification.ServiceList{Items: services}, nil
}

// List returns list of notification templates
func (s *Server) ListTemplates(ctx context.Context, q *notification.TemplatesListRequest) (*notification.TemplateList, error) {
	api, err := s.apiFactory.GetAPI()
	if err != nil {
		if apierr.IsNotFound(err) {
			return &notification.TemplateList{}, nil
		}
		return nil, err
	}
	templates := []*notification.Template{}
	for tmpl := range api.GetConfig().Templates {
		templates = append(templates, &notification.Template{Name: ptr.To(tmpl)})
	}
	return &notification.TemplateList{Items: templates}, nil
}
