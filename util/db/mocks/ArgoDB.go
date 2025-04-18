// Code generated by mockery v2.52.4. DO NOT EDIT.

package mocks

import (
	context "context"

	db "github.com/argoproj/argo-cd/v3/util/db"
	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ArgoDB is an autogenerated mock type for the ArgoDB type
type ArgoDB struct {
	mock.Mock
}

// AddGPGPublicKey provides a mock function with given fields: ctx, keyData
func (_m *ArgoDB) AddGPGPublicKey(ctx context.Context, keyData string) (map[string]*v1alpha1.GnuPGPublicKey, []string, error) {
	ret := _m.Called(ctx, keyData)

	if len(ret) == 0 {
		panic("no return value specified for AddGPGPublicKey")
	}

	var r0 map[string]*v1alpha1.GnuPGPublicKey
	var r1 []string
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (map[string]*v1alpha1.GnuPGPublicKey, []string, error)); ok {
		return rf(ctx, keyData)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) map[string]*v1alpha1.GnuPGPublicKey); ok {
		r0 = rf(ctx, keyData)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*v1alpha1.GnuPGPublicKey)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) []string); ok {
		r1 = rf(ctx, keyData)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]string)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string) error); ok {
		r2 = rf(ctx, keyData)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// CreateCluster provides a mock function with given fields: ctx, c
func (_m *ArgoDB) CreateCluster(ctx context.Context, c *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	ret := _m.Called(ctx, c)

	if len(ret) == 0 {
		panic("no return value specified for CreateCluster")
	}

	var r0 *v1alpha1.Cluster
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) (*v1alpha1.Cluster, error)); ok {
		return rf(ctx, c)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) *v1alpha1.Cluster); ok {
		r0 = rf(ctx, c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Cluster)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Cluster) error); ok {
		r1 = rf(ctx, c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateRepoCertificate provides a mock function with given fields: ctx, certificate, upsert
func (_m *ArgoDB) CreateRepoCertificate(ctx context.Context, certificate *v1alpha1.RepositoryCertificateList, upsert bool) (*v1alpha1.RepositoryCertificateList, error) {
	ret := _m.Called(ctx, certificate, upsert)

	if len(ret) == 0 {
		panic("no return value specified for CreateRepoCertificate")
	}

	var r0 *v1alpha1.RepositoryCertificateList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepositoryCertificateList, bool) (*v1alpha1.RepositoryCertificateList, error)); ok {
		return rf(ctx, certificate, upsert)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepositoryCertificateList, bool) *v1alpha1.RepositoryCertificateList); ok {
		r0 = rf(ctx, certificate, upsert)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepositoryCertificateList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RepositoryCertificateList, bool) error); ok {
		r1 = rf(ctx, certificate, upsert)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateRepository provides a mock function with given fields: ctx, r
func (_m *ArgoDB) CreateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for CreateRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) *v1alpha1.Repository); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Repository) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateRepositoryCredentials provides a mock function with given fields: ctx, r
func (_m *ArgoDB) CreateRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for CreateRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RepoCreds) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateWriteRepository provides a mock function with given fields: ctx, r
func (_m *ArgoDB) CreateWriteRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for CreateWriteRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) *v1alpha1.Repository); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Repository) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateWriteRepositoryCredentials provides a mock function with given fields: ctx, r
func (_m *ArgoDB) CreateWriteRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for CreateWriteRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RepoCreds) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteCluster provides a mock function with given fields: ctx, server
func (_m *ArgoDB) DeleteCluster(ctx context.Context, server string) error {
	ret := _m.Called(ctx, server)

	if len(ret) == 0 {
		panic("no return value specified for DeleteCluster")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, server)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteGPGPublicKey provides a mock function with given fields: ctx, keyID
func (_m *ArgoDB) DeleteGPGPublicKey(ctx context.Context, keyID string) error {
	ret := _m.Called(ctx, keyID)

	if len(ret) == 0 {
		panic("no return value specified for DeleteGPGPublicKey")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, keyID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteRepository provides a mock function with given fields: ctx, name, project
func (_m *ArgoDB) DeleteRepository(ctx context.Context, name string, project string) error {
	ret := _m.Called(ctx, name, project)

	if len(ret) == 0 {
		panic("no return value specified for DeleteRepository")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, name, project)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteRepositoryCredentials provides a mock function with given fields: ctx, name
func (_m *ArgoDB) DeleteRepositoryCredentials(ctx context.Context, name string) error {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for DeleteRepositoryCredentials")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteWriteRepository provides a mock function with given fields: ctx, name, project
func (_m *ArgoDB) DeleteWriteRepository(ctx context.Context, name string, project string) error {
	ret := _m.Called(ctx, name, project)

	if len(ret) == 0 {
		panic("no return value specified for DeleteWriteRepository")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, name, project)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteWriteRepositoryCredentials provides a mock function with given fields: ctx, name
func (_m *ArgoDB) DeleteWriteRepositoryCredentials(ctx context.Context, name string) error {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for DeleteWriteRepositoryCredentials")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetAllHelmRepositoryCredentials provides a mock function with given fields: ctx
func (_m *ArgoDB) GetAllHelmRepositoryCredentials(ctx context.Context) ([]*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetAllHelmRepositoryCredentials")
	}

	var r0 []*v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*v1alpha1.RepoCreds); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetApplicationControllerReplicas provides a mock function with no fields
func (_m *ArgoDB) GetApplicationControllerReplicas() int {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetApplicationControllerReplicas")
	}

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// GetCluster provides a mock function with given fields: ctx, server
func (_m *ArgoDB) GetCluster(ctx context.Context, server string) (*v1alpha1.Cluster, error) {
	ret := _m.Called(ctx, server)

	if len(ret) == 0 {
		panic("no return value specified for GetCluster")
	}

	var r0 *v1alpha1.Cluster
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1alpha1.Cluster, error)); ok {
		return rf(ctx, server)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1alpha1.Cluster); ok {
		r0 = rf(ctx, server)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Cluster)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, server)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetClusterServersByName provides a mock function with given fields: ctx, name
func (_m *ArgoDB) GetClusterServersByName(ctx context.Context, name string) ([]string, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetClusterServersByName")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]string, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []string); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProjectClusters provides a mock function with given fields: ctx, project
func (_m *ArgoDB) GetProjectClusters(ctx context.Context, project string) ([]*v1alpha1.Cluster, error) {
	ret := _m.Called(ctx, project)

	if len(ret) == 0 {
		panic("no return value specified for GetProjectClusters")
	}

	var r0 []*v1alpha1.Cluster
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*v1alpha1.Cluster, error)); ok {
		return rf(ctx, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*v1alpha1.Cluster); ok {
		r0 = rf(ctx, project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Cluster)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProjectRepositories provides a mock function with given fields: project
func (_m *ArgoDB) GetProjectRepositories(project string) ([]*v1alpha1.Repository, error) {
	ret := _m.Called(project)

	if len(ret) == 0 {
		panic("no return value specified for GetProjectRepositories")
	}

	var r0 []*v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]*v1alpha1.Repository, error)); ok {
		return rf(project)
	}
	if rf, ok := ret.Get(0).(func(string) []*v1alpha1.Repository); ok {
		r0 = rf(project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProjectWriteRepositories provides a mock function with given fields: project
func (_m *ArgoDB) GetProjectWriteRepositories(project string) ([]*v1alpha1.Repository, error) {
	ret := _m.Called(project)

	if len(ret) == 0 {
		panic("no return value specified for GetProjectWriteRepositories")
	}

	var r0 []*v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(string) ([]*v1alpha1.Repository, error)); ok {
		return rf(project)
	}
	if rf, ok := ret.Get(0).(func(string) []*v1alpha1.Repository); ok {
		r0 = rf(project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRepository provides a mock function with given fields: ctx, url, project
func (_m *ArgoDB) GetRepository(ctx context.Context, url string, project string) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, url, project)

	if len(ret) == 0 {
		panic("no return value specified for GetRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, url, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1alpha1.Repository); ok {
		r0 = rf(ctx, url, project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, url, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRepositoryCredentials provides a mock function with given fields: ctx, name
func (_m *ArgoDB) GetRepositoryCredentials(ctx context.Context, name string) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWriteRepository provides a mock function with given fields: ctx, url, project
func (_m *ArgoDB) GetWriteRepository(ctx context.Context, url string, project string) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, url, project)

	if len(ret) == 0 {
		panic("no return value specified for GetWriteRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, url, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *v1alpha1.Repository); ok {
		r0 = rf(ctx, url, project)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, url, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWriteRepositoryCredentials provides a mock function with given fields: ctx, name
func (_m *ArgoDB) GetWriteRepositoryCredentials(ctx context.Context, name string) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for GetWriteRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListClusters provides a mock function with given fields: ctx
func (_m *ArgoDB) ListClusters(ctx context.Context) (*v1alpha1.ClusterList, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListClusters")
	}

	var r0 *v1alpha1.ClusterList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*v1alpha1.ClusterList, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *v1alpha1.ClusterList); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.ClusterList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListConfiguredGPGPublicKeys provides a mock function with given fields: ctx
func (_m *ArgoDB) ListConfiguredGPGPublicKeys(ctx context.Context) (map[string]*v1alpha1.GnuPGPublicKey, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListConfiguredGPGPublicKeys")
	}

	var r0 map[string]*v1alpha1.GnuPGPublicKey
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (map[string]*v1alpha1.GnuPGPublicKey, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) map[string]*v1alpha1.GnuPGPublicKey); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*v1alpha1.GnuPGPublicKey)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListHelmRepositories provides a mock function with given fields: ctx
func (_m *ArgoDB) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListHelmRepositories")
	}

	var r0 []*v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*v1alpha1.Repository, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*v1alpha1.Repository); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListRepoCertificates provides a mock function with given fields: ctx, selector
func (_m *ArgoDB) ListRepoCertificates(ctx context.Context, selector *db.CertificateListSelector) (*v1alpha1.RepositoryCertificateList, error) {
	ret := _m.Called(ctx, selector)

	if len(ret) == 0 {
		panic("no return value specified for ListRepoCertificates")
	}

	var r0 *v1alpha1.RepositoryCertificateList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *db.CertificateListSelector) (*v1alpha1.RepositoryCertificateList, error)); ok {
		return rf(ctx, selector)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *db.CertificateListSelector) *v1alpha1.RepositoryCertificateList); ok {
		r0 = rf(ctx, selector)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepositoryCertificateList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *db.CertificateListSelector) error); ok {
		r1 = rf(ctx, selector)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListRepositories provides a mock function with given fields: ctx
func (_m *ArgoDB) ListRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListRepositories")
	}

	var r0 []*v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*v1alpha1.Repository, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*v1alpha1.Repository); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListRepositoryCredentials provides a mock function with given fields: ctx
func (_m *ArgoDB) ListRepositoryCredentials(ctx context.Context) ([]string, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListRepositoryCredentials")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]string, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []string); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListWriteRepositories provides a mock function with given fields: ctx
func (_m *ArgoDB) ListWriteRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListWriteRepositories")
	}

	var r0 []*v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]*v1alpha1.Repository, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []*v1alpha1.Repository); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListWriteRepositoryCredentials provides a mock function with given fields: ctx
func (_m *ArgoDB) ListWriteRepositoryCredentials(ctx context.Context) ([]string, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ListWriteRepositoryCredentials")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]string, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []string); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveRepoCertificates provides a mock function with given fields: ctx, selector
func (_m *ArgoDB) RemoveRepoCertificates(ctx context.Context, selector *db.CertificateListSelector) (*v1alpha1.RepositoryCertificateList, error) {
	ret := _m.Called(ctx, selector)

	if len(ret) == 0 {
		panic("no return value specified for RemoveRepoCertificates")
	}

	var r0 *v1alpha1.RepositoryCertificateList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *db.CertificateListSelector) (*v1alpha1.RepositoryCertificateList, error)); ok {
		return rf(ctx, selector)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *db.CertificateListSelector) *v1alpha1.RepositoryCertificateList); ok {
		r0 = rf(ctx, selector)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepositoryCertificateList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *db.CertificateListSelector) error); ok {
		r1 = rf(ctx, selector)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RepositoryExists provides a mock function with given fields: ctx, repoURL, project
func (_m *ArgoDB) RepositoryExists(ctx context.Context, repoURL string, project string) (bool, error) {
	ret := _m.Called(ctx, repoURL, project)

	if len(ret) == 0 {
		panic("no return value specified for RepositoryExists")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (bool, error)); ok {
		return rf(ctx, repoURL, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) bool); ok {
		r0 = rf(ctx, repoURL, project)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, repoURL, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateCluster provides a mock function with given fields: ctx, c
func (_m *ArgoDB) UpdateCluster(ctx context.Context, c *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	ret := _m.Called(ctx, c)

	if len(ret) == 0 {
		panic("no return value specified for UpdateCluster")
	}

	var r0 *v1alpha1.Cluster
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) (*v1alpha1.Cluster, error)); ok {
		return rf(ctx, c)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Cluster) *v1alpha1.Cluster); ok {
		r0 = rf(ctx, c)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Cluster)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Cluster) error); ok {
		r1 = rf(ctx, c)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateRepository provides a mock function with given fields: ctx, r
func (_m *ArgoDB) UpdateRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) *v1alpha1.Repository); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Repository) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateRepositoryCredentials provides a mock function with given fields: ctx, r
func (_m *ArgoDB) UpdateRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RepoCreds) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateWriteRepository provides a mock function with given fields: ctx, r
func (_m *ArgoDB) UpdateWriteRepository(ctx context.Context, r *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for UpdateWriteRepository")
	}

	var r0 *v1alpha1.Repository
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) (*v1alpha1.Repository, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.Repository) *v1alpha1.Repository); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.Repository)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.Repository) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdateWriteRepositoryCredentials provides a mock function with given fields: ctx, r
func (_m *ArgoDB) UpdateWriteRepositoryCredentials(ctx context.Context, r *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error) {
	ret := _m.Called(ctx, r)

	if len(ret) == 0 {
		panic("no return value specified for UpdateWriteRepositoryCredentials")
	}

	var r0 *v1alpha1.RepoCreds
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) (*v1alpha1.RepoCreds, error)); ok {
		return rf(ctx, r)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RepoCreds) *v1alpha1.RepoCreds); ok {
		r0 = rf(ctx, r)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RepoCreds)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RepoCreds) error); ok {
		r1 = rf(ctx, r)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WatchClusters provides a mock function with given fields: ctx, handleAddEvent, handleModEvent, handleDeleteEvent
func (_m *ArgoDB) WatchClusters(ctx context.Context, handleAddEvent func(*v1alpha1.Cluster), handleModEvent func(*v1alpha1.Cluster, *v1alpha1.Cluster), handleDeleteEvent func(string)) error {
	ret := _m.Called(ctx, handleAddEvent, handleModEvent, handleDeleteEvent)

	if len(ret) == 0 {
		panic("no return value specified for WatchClusters")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, func(*v1alpha1.Cluster), func(*v1alpha1.Cluster, *v1alpha1.Cluster), func(string)) error); ok {
		r0 = rf(ctx, handleAddEvent, handleModEvent, handleDeleteEvent)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WriteRepositoryExists provides a mock function with given fields: ctx, repoURL, project
func (_m *ArgoDB) WriteRepositoryExists(ctx context.Context, repoURL string, project string) (bool, error) {
	ret := _m.Called(ctx, repoURL, project)

	if len(ret) == 0 {
		panic("no return value specified for WriteRepositoryExists")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (bool, error)); ok {
		return rf(ctx, repoURL, project)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) bool); ok {
		r0 = rf(ctx, repoURL, project)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, repoURL, project)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewArgoDB creates a new instance of ArgoDB. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewArgoDB(t interface {
	mock.TestingT
	Cleanup(func())
}) *ArgoDB {
	mock := &ArgoDB{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
