package gpgkey

import (
	"fmt"

	"golang.org/x/net/context"

	gpgkeypkg "github.com/argoproj/argo-cd/pkg/apiclient/gpgkey"
	appsv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/gpg"
	"github.com/argoproj/argo-cd/util/rbac"
)

// Server provides a Certificate service
type Server struct {
	db            db.ArgoDB
	repoClientset apiclient.Clientset
	enf           *rbac.Enforcer
}

// NewServer returns a new instance of the Certificate service
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

// TODO: RBAC policies are currently an all-or-nothing approach, so there is no
// fine grained control for certificate manipulation. Either a user has access
// to a given certificate operation (get/create/delete), or it doesn't.

// Returns a list of configured GnuPG public keys
func (s *Server) ListGnuPGPublicKeys(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*appsv1.GnuPGPublicKeyList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceCertificates, rbacpolicy.ActionGet, ""); err != nil {
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

func (s *Server) GetGnuPGPublicKey(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*appsv1.GnuPGPublicKey, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceCertificates, rbacpolicy.ActionGet, ""); err != nil {
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

// CreateGnuPGPublicKey imports one or more public keys to the server's keyring
func (s *Server) CreateGnuPGPublicKey(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyCreateRequest) (*gpgkeypkg.GnuPGPublicKeyCreateResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceCertificates, rbacpolicy.ActionCreate, ""); err != nil {
		return nil, err
	}

	added, skipped, err := s.db.AddGPGPublicKey(ctx, q.Publickey)
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

func (s *Server) DeleteGnuPGPublicKey(ctx context.Context, q *gpgkeypkg.GnuPGPublicKeyQuery) (*gpgkeypkg.GnuPGPublicKeyResponse, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceCertificates, rbacpolicy.ActionCreate, ""); err != nil {
		return nil, err
	}

	err := s.db.DeleteGPGPublicKey(ctx, q.KeyID)
	if err != nil {
		return nil, err
	}

	return &gpgkeypkg.GnuPGPublicKeyResponse{}, nil
}

// // Batch deletes a list of certificates that match the query
// func (s *Server) DeleteCertificate(ctx context.Context, q *certificatepkg.RepositoryCertificateQuery) (*appsv1.RepositoryCertificateList, error) {
// 	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceCertificates, rbacpolicy.ActionDelete, ""); err != nil {
// 		return nil, err
// 	}
// 	certs, err := s.db.RemoveRepoCertificates(ctx, &db.CertificateListSelector{
// 		HostNamePattern: q.GetHostNamePattern(),
// 		CertType:        q.GetCertType(),
// 		CertSubType:     q.GetCertSubType(),
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return certs, nil
// }
