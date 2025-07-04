package certificate

import (
	"context"

	certificatepkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/certificate"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/rbac"
)

// Server provides a Certificate service
type Server struct {
	db  db.ArgoDB
	enf *rbac.Enforcer
}

// NewServer returns a new instance of the Certificate service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer) *Server {
	return &Server{
		db:  db,
		enf: enf,
	}
}

// TODO: RBAC policies are currently an all-or-nothing approach, so there is no
// fine grained control for certificate manipulation. Either a user has access
// to a given certificate operation (get/create/delete), or it doesn't.

// Returns a list of configured certificates that match the query
func (s *Server) ListCertificates(ctx context.Context, q *certificatepkg.RepositoryCertificateQuery) (*appsv1.RepositoryCertificateList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceCertificates, rbac.ActionGet, ""); err != nil {
		return nil, err
	}
	certList, err := s.db.ListRepoCertificates(ctx, &db.CertificateListSelector{
		HostNamePattern: q.GetHostNamePattern(),
		CertType:        q.GetCertType(),
		CertSubType:     q.GetCertSubType(),
	})
	if err != nil {
		return nil, err
	}
	return certList, nil
}

// Batch creates certificates for verifying repositories
func (s *Server) CreateCertificate(ctx context.Context, q *certificatepkg.RepositoryCertificateCreateRequest) (*appsv1.RepositoryCertificateList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceCertificates, rbac.ActionCreate, ""); err != nil {
		return nil, err
	}
	certs, err := s.db.CreateRepoCertificate(ctx, q.Certificates, q.Upsert)
	if err != nil {
		return nil, err
	}

	return certs, nil
}

// Batch deletes a list of certificates that match the query
func (s *Server) DeleteCertificate(ctx context.Context, q *certificatepkg.RepositoryCertificateQuery) (*appsv1.RepositoryCertificateList, error) {
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbac.ResourceCertificates, rbac.ActionDelete, ""); err != nil {
		return nil, err
	}
	certs, err := s.db.RemoveRepoCertificates(ctx, &db.CertificateListSelector{
		HostNamePattern: q.GetHostNamePattern(),
		CertType:        q.GetCertType(),
		CertSubType:     q.GetCertSubType(),
	})
	if err != nil {
		return nil, err
	}
	return certs, nil
}
