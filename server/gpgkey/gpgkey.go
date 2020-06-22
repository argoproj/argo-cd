package gpgkey

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	gpgkeypkg "github.com/argoproj/argo-cd/pkg/apiclient/gpgkey"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/gpg"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a service of type GPGKeyService
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
}

// NewServer returns a new instance of the service with type GPGKeyService
func NewServer(
	repoClientset apiclient.Clientset,
	db db.ArgoDB,
	enf *rbac.Enforcer,
) *Server {
	return &Server{
		db:            db,
		repoClientset: repoClientset,
		enf:           enf,
	}
}

// ListGnuPGPublicKeys returns a list of GnuPG public keys in the configuration
func (s *Server) List(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*appsv1.GnuPGPublicKeyList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceGPGKeys, rbacpolicy.ActionGet, ""); err != nil {
		return nil, err
	}
	keys, err := s.db.ListConfiguredGPGPublicKeys(ctx)
	if err != nil {
		return nil, err
	}
	keyList := &appsv1.GnuPGPublicKeyList{}
	for _, v := range keys {
		// Remove key's data from list result to save some bytes
		v.KeyData = ""
		keyList.Items = append(keyList.Items, *v)
	}
	return keyList, nil
}

// GetGnuPGPublicKey retrieves a single GPG public key from the configuration
func (s *Server) Get(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*appsv1.GnuPGPublicKey, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceGPGKeys, rbacpolicy.ActionGet, ""); err != nil {
		return nil, err
	}

	keyID := gpg.KeyID(q.KeyID)
	if keyID == "" {
		return nil, fmt.Errorf("KeyID is malformed or empty")
	}

	keys, err := s.db.ListConfiguredGPGPublicKeys(ctx)
	if err != nil {
		return nil, err
	}

	if key, ok := keys[keyID]; ok {
		return key, nil
	}

	return nil, fmt.Errorf("No such key: %s", keyID)
}

// CreateGnuPGPublicKey adds one or more GPG public keys to the server's configuration
func (s *Server) Create(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyCreateRequest) (*gpgkeypkg.GnuPGPublicKeyCreateResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceGPGKeys, rbacpolicy.ActionCreate, ""); err != nil {
		return nil, err
	}

	keyData := strings.TrimSpace(q.Publickey.KeyData)
	if keyData == "" {
		return nil, fmt.Errorf("Submitted key data is empty")
	}

	added, skipped, err := s.db.AddGPGPublicKey(ctx, q.Publickey.KeyData)
	if err != nil {
		return nil, err
	}

	items := make([]appsv1.GnuPGPublicKey, 0)
	for _, k := range added {
		items = append(items, *k)
	}

	response := &gpgkeypkg.GnuPGPublicKeyCreateResponse{
		Created: &appsv1.GnuPGPublicKeyList{Items: items},
		Skipped: skipped,
	}

	return response, nil
}

// DeleteGnuPGPublicKey removes a single GPG public key from the server's configuration
func (s *Server) Delete(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*gpgkeypkg.GnuPGPublicKeyResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceGPGKeys, rbacpolicy.ActionDelete, ""); err != nil {
		return nil, err
	}

	err := s.db.DeleteGPGPublicKey(ctx, q.KeyID)
	if err != nil {
		return nil, err
	}

	return &gpgkeypkg.GnuPGPublicKeyResponse{}, nil
}
