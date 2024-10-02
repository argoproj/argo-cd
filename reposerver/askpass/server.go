package askpass

import (
	"context"
	"net"
	"os"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/io"
)

type Server interface {
	git.CredsStore
	AskPassServiceServer
	Run(path string) error
}

type server struct {
	lock  sync.Mutex
	creds map[string]Creds
}

// NewServer returns a new server
func NewServer() *server {
	return &server{
		creds: make(map[string]Creds),
	}
}

func (s *server) GetCredentials(_ context.Context, q *CredentialsRequest) (*CredentialsResponse, error) {
	if q.Nonce == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing nonce")
	}
	creds, ok := s.getCreds(q.Nonce)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown nonce")
	}
	return &CredentialsResponse{Username: creds.Username, Password: creds.Password}, nil
}

func (s *server) Start(path string) (io.Closer, error) {
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	server := grpc.NewServer()
	RegisterAskPassServiceServer(server, s)
	go func() {
		_ = server.Serve(listener)
	}()
	return io.NewCloser(listener.Close), nil
}

func (s *server) Run(path string) error {
	_, err := s.Start(path)
	return err
}

// Add adds a new credential to the server and returns associated id
func (s *server) Add(username string, password string) string {
	s.lock.Lock()
	defer s.lock.Unlock()
	id := uuid.New().String()
	s.creds[id] = Creds{
		Username: username,
		Password: password,
	}
	return id
}

// Remove removes the credential with the given id
func (s *server) Remove(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.creds, id)
}

func (s *server) getCreds(id string) (*Creds, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	creds, ok := s.creds[id]
	return &creds, ok
}
