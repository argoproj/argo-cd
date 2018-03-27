package session

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/kube"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Session service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
}

// NewServer returns a new instance of the Session service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface) SessionServiceServer {
	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
	}
}

// ListPods returns application related pods in a session
func (s *Server) ListPods(ctx context.Context, q *SessionQuery) (*apiv1.PodList, error) {
	// TODO: filter by the app label
	return s.kubeclientset.CoreV1().Pods(s.ns).List(metav1.ListOptions{})
}

// List returns list of sessions
func (s *Server) List(ctx context.Context, q *SessionQuery) (*appv1.SessionList, error) {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.SecretTypeSession})
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	sessionSecrets, err := s.kubeclientset.CoreV1().Secrets(s.ns).List(listOpts)
	if err != nil {
		return nil, err
	}
	sessionList := appv1.SessionList{
		Items: make([]appv1.Session, len(sessionSecrets.Items)),
	}
	for i, sessionSecret := range sessionSecrets.Items {
		sessionList.Items[i] = *secretToSession(&sessionSecret)
	}
	return &sessionList, nil
}

// Create creates a session
func (s *Server) Create(ctx context.Context, c *appv1.Session) (*appv1.Session, error) {
	err := kube.TestConfig(c.RESTConfig())
	if err != nil {
		return nil, err
	}
	secName := serverToSecretName(c.Server)
	sessionSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.SecretTypeSession,
			},
		},
	}
	sessionSecret.StringData = sessionToStringData(c)
	sessionSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Create(sessionSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, grpc.Errorf(codes.AlreadyExists, "session '%s' already exists", c.Server)
		}
		return nil, err
	}
	return secretToSession(sessionSecret), nil
}

func (s *Server) getSessionSecret(server string) (*apiv1.Secret, error) {
	secName := serverToSecretName(server)
	sessionSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(secName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, grpc.Errorf(codes.NotFound, "session '%s' not found", server)
		}
		return nil, err
	}
	return sessionSecret, nil
}

// Get returns a session from a query
func (s *Server) Get(ctx context.Context, q *SessionQuery) (*appv1.Session, error) {
	sessionSecret, err := s.getSessionSecret(q.Server)
	if err != nil {
		return nil, err
	}
	return secretToSession(sessionSecret), nil
}

// Update updates a session
func (s *Server) Update(ctx context.Context, c *appv1.Session) (*appv1.Session, error) {
	err := kube.TestConfig(c.RESTConfig())
	if err != nil {
		return nil, err
	}
	sessionSecret, err := s.getSessionSecret(c.Server)
	if err != nil {
		return nil, err
	}
	sessionSecret.StringData = sessionToStringData(c)
	sessionSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(sessionSecret)
	if err != nil {
		return nil, err
	}
	return secretToSession(sessionSecret), nil
}

// UpdateREST updates a session (special handler intended to be used only by the gRPC gateway)
func (s *Server) UpdateREST(ctx context.Context, r *SessionUpdateRequest) (*appv1.Session, error) {
	return s.Update(ctx, r.Session)
}

// Delete deletes a session by name
func (s *Server) Delete(ctx context.Context, q *SessionQuery) (*SessionResponse, error) {
	secName := serverToSecretName(q.Server)
	err := s.kubeclientset.CoreV1().Secrets(s.ns).Delete(secName, &metav1.DeleteOptions{})
	return &SessionResponse{}, err
}

// serverToSecretName hashes server address to the secret name using a formula.
// Part of the server address is incorporated for debugging purposes
func serverToSecretName(server string) string {
	serverURL, err := url.ParseRequestURI(server)
	if err != nil {
		panic(err)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(server))
	host := strings.Split(serverURL.Host, ":")[0]
	return fmt.Sprintf("session-%s-%v", host, h.Sum32())
}

// sessionToStringData converts a session object to string data for serialization to a secret
func sessionToStringData(c *appv1.Session) map[string]string {
	stringData := make(map[string]string)
	stringData["server"] = c.Server
	if c.Name == "" {
		stringData["name"] = c.Server
	} else {
		stringData["name"] = c.Name
	}
	configBytes, err := json.Marshal(c.Config)
	if err != nil {
		panic(err)
	}
	stringData["config"] = string(configBytes)
	return stringData
}

// secretToRepo converts a secret into a repository object
func secretToSession(s *apiv1.Secret) *appv1.Session {
	var config appv1.SessionConfig
	err := json.Unmarshal(s.Data["config"], &config)
	if err != nil {
		panic(err)
	}
	session := appv1.Session{
		Server: string(s.Data["server"]),
		Name:   string(s.Data["name"]),
		Config: config,
	}
	return &session
}
